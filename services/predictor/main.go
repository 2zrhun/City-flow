package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gonum.org/v1/gonum/stat"
)

const (
	maxSpeed  = 90.0
	maxFlow   = 120.0
	ewmaAlpha = 0.7 // EWMA blending factor (higher = more weight on predicted)
)

type Prediction struct {
	TS              time.Time `json:"ts"`
	RoadID          string    `json:"road_id"`
	HorizonMin      int       `json:"horizon_min"`
	CongestionScore float64   `json:"congestion_score"`
	Confidence      float64   `json:"confidence"`
	ModelVersion    string    `json:"model_version"`
}

// bucketData holds aggregated traffic metrics for a single time bucket + road.
type bucketData struct {
	offsetMin float64
	avgSpeed  float64
	avgOcc    float64
	avgFlow   float64
	samples   int64
}

var (
	predictionsGenerated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_predictor_predictions_generated_total",
		Help: "Total number of predictions computed.",
	})
	predictionsStored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_predictor_predictions_stored_total",
		Help: "Total number of predictions stored in DB.",
	})
	predictionsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_predictor_predictions_failed_total",
		Help: "Total number of prediction failures.",
	})
	predictionsPublished = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_predictor_predictions_published_total",
		Help: "Total number of predictions published to Redis.",
	})
	cycleDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cityflow_predictor_cycle_duration_seconds",
		Help:    "Duration of a full prediction cycle.",
		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
	})
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbDSN := getEnv("DB_DSN", "postgres://cityflow:cityflow_dev_password@localhost:5432/cityflow?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	metricsAddr := getEnv("METRICS_ADDR", ":8080")
	intervalSec := getEnvInt("PREDICTION_INTERVAL_SEC", 60)
	lookbackMin := getEnvInt("LOOKBACK_WINDOW_MIN", 30)
	horizonMin := getEnvInt("HORIZON_MIN", 30)
	modelVersion := getEnv("MODEL_VERSION", "ewma-lr-v2")

	dbPool, err := pgxpool.New(ctx, dbDSN)
	if err != nil {
		log.Fatalf("db pool init failed: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	log.Printf("db connected")

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	redisClient := redis.NewClient(opts)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
	log.Printf("redis connected: %s", redisURL)

	go serveHTTP(metricsAddr)

	interval := time.Duration(intervalSec) * time.Second
	lookback := time.Duration(lookbackMin) * time.Minute

	log.Printf("predictor running: interval=%s lookback=%s horizon=%dm model=%s",
		interval, lookback, horizonMin, modelVersion)

	runCycle(ctx, dbPool, redisClient, lookback, horizonMin, modelVersion)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runCycle(ctx, dbPool, redisClient, lookback, horizonMin, modelVersion)
		case <-ctx.Done():
			log.Printf("predictor shutting down")
			return
		}
	}
}

func runCycle(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client, lookback time.Duration, horizonMin int, modelVersion string) {
	start := time.Now()
	defer func() {
		cycleDuration.Observe(time.Since(start).Seconds())
	}()

	now := time.Now().UTC().Truncate(time.Second)
	windowStart := now.Add(-lookback)

	// Query time-bucketed data using TimescaleDB time_bucket()
	rows, err := dbPool.Query(ctx, `
		SELECT
			time_bucket('5 minutes', ts) AS bucket,
			road_id,
			AVG(speed_kmh)  AS avg_speed,
			AVG(occupancy)  AS avg_occ,
			AVG(flow_rate)  AS avg_flow,
			COUNT(*)        AS samples
		FROM traffic_raw
		WHERE ts >= $1
		GROUP BY bucket, road_id
		ORDER BY road_id, bucket
	`, windowStart)
	if err != nil {
		predictionsFailed.Inc()
		log.Printf("query traffic_raw failed: %v", err)
		return
	}
	defer rows.Close()

	// Group buckets by road
	roadBuckets := make(map[string][]bucketData)
	totalSamples := make(map[string]int64)

	for rows.Next() {
		var bucketTS time.Time
		var roadID string
		var avgSpeed, avgOcc, avgFlow float64
		var samples int64

		if err := rows.Scan(&bucketTS, &roadID, &avgSpeed, &avgOcc, &avgFlow, &samples); err != nil {
			predictionsFailed.Inc()
			log.Printf("row scan failed: %v", err)
			continue
		}

		offsetMin := bucketTS.Sub(windowStart).Minutes()
		roadBuckets[roadID] = append(roadBuckets[roadID], bucketData{
			offsetMin: offsetMin,
			avgSpeed:  avgSpeed,
			avgOcc:    avgOcc,
			avgFlow:   avgFlow,
			samples:   samples,
		})
		totalSamples[roadID] += samples
	}
	if rows.Err() != nil {
		predictionsFailed.Inc()
		log.Printf("rows iteration error: %v", rows.Err())
		return
	}

	if len(roadBuckets) == 0 {
		log.Printf("no traffic data in lookback window, skipping")
		return
	}

	// Generate predictions per road
	var predictions []Prediction
	lbMin := lookback.Minutes()
	futureOffset := lbMin + float64(horizonMin)
	hour := now.Hour()

	for roadID, buckets := range roadBuckets {
		if len(buckets) == 0 {
			continue
		}

		// Compute congestion score for each time bucket
		xs := make([]float64, len(buckets))
		ys := make([]float64, len(buckets))
		for i, b := range buckets {
			xs[i] = b.offsetMin
			ys[i] = computeCongestionScore(b.avgSpeed, b.avgOcc, b.avgFlow)
		}

		currentScore := ys[len(ys)-1]

		var finalScore float64
		var trendStability float64

		if len(buckets) >= 2 {
			// Fit linear regression on the time-series of congestion scores
			slope, intercept := fitLinearRegression(xs, ys)

			// Extrapolate to now + horizon
			predicted := slope*futureOffset + intercept

			// EWMA blend: weight predicted vs current
			blended := ewma(predicted, currentScore, ewmaAlpha)

			// Rush hour adjustment
			finalScore = blended * rushHourFactor(hour)
			finalScore = math.Max(0.0, math.Min(1.0, finalScore))

			// Trend stability: lower confidence when slope is steep (volatile data)
			trendStability = math.Max(0.3, 1.0-math.Abs(slope)*10)
		} else {
			// Single bucket fallback
			finalScore = currentScore * rushHourFactor(hour)
			finalScore = math.Max(0.0, math.Min(1.0, finalScore))
			trendStability = 0.5
		}

		sampleConfidence := math.Min(1.0, float64(totalSamples[roadID])/50.0)
		confidence := sampleConfidence * trendStability

		predictions = append(predictions, Prediction{
			TS:              now,
			RoadID:          roadID,
			HorizonMin:      horizonMin,
			CongestionScore: math.Round(finalScore*1000) / 1000,
			Confidence:      math.Round(confidence*100) / 100,
			ModelVersion:    modelVersion,
		})
		predictionsGenerated.Inc()
	}

	if len(predictions) == 0 {
		log.Printf("no predictions generated")
		return
	}

	stored := storePredictions(ctx, dbPool, predictions)
	published := publishPredictions(ctx, redisClient, predictions)

	log.Printf("prediction cycle [%s]: %d roads, %d stored, %d published (%.2fs)",
		modelVersion, len(predictions), stored, published, time.Since(start).Seconds())
}

// ── ML Functions ──

// computeCongestionScore computes a weighted congestion score from traffic metrics.
func computeCongestionScore(avgSpeed, avgOccupancy, avgFlow float64) float64 {
	speedScore := 1.0 - (avgSpeed / maxSpeed)
	occupancyScore := avgOccupancy
	flowScore := avgFlow / maxFlow

	score := 0.4*speedScore + 0.4*occupancyScore + 0.2*flowScore
	return math.Max(0.0, math.Min(1.0, score))
}

// ewma computes Exponential Weighted Moving Average.
func ewma(predicted, current, alpha float64) float64 {
	return alpha*predicted + (1-alpha)*current
}

// rushHourFactor returns a time-of-day multiplier for congestion prediction.
func rushHourFactor(hour int) float64 {
	switch {
	case (hour >= 7 && hour < 9) || (hour >= 17 && hour < 19):
		return 1.15 // morning/evening rush
	case hour >= 21 || hour < 6:
		return 0.85 // night
	default:
		return 1.0
	}
}

// fitLinearRegression fits y = slope*x + intercept using gonum.
func fitLinearRegression(xs, ys []float64) (slope, intercept float64) {
	if len(xs) < 2 {
		return 0, ys[0]
	}
	intercept, slope = stat.LinearRegression(xs, ys, nil, false)
	return slope, intercept
}

// ── Storage & Publishing ──

func storePredictions(ctx context.Context, dbPool *pgxpool.Pool, predictions []Prediction) int {
	stored := 0
	for _, p := range predictions {
		_, err := dbPool.Exec(ctx, `
			INSERT INTO predictions (ts, road_id, horizon_min, congestion_score, confidence, model_version)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (ts, road_id, horizon_min) DO UPDATE SET
				congestion_score = EXCLUDED.congestion_score,
				confidence = EXCLUDED.confidence,
				model_version = EXCLUDED.model_version
		`, p.TS, p.RoadID, p.HorizonMin, p.CongestionScore, p.Confidence, p.ModelVersion)
		if err != nil {
			predictionsFailed.Inc()
			log.Printf("db insert failed for road=%s: %v", p.RoadID, err)
			continue
		}
		predictionsStored.Inc()
		stored++
	}
	return stored
}

func publishPredictions(ctx context.Context, redisClient *redis.Client, predictions []Prediction) int {
	published := 0
	for _, p := range predictions {
		data, err := json.Marshal(p)
		if err != nil {
			log.Printf("json marshal failed for road=%s: %v", p.RoadID, err)
			continue
		}
		if err := redisClient.Publish(ctx, "cityflow:predictions", data).Err(); err != nil {
			log.Printf("redis publish failed for road=%s: %v", p.RoadID, err)
			continue
		}
		predictionsPublished.Inc()
		published++
	}
	return published
}

func serveHTTP(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("metrics server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("metrics server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("invalid %s=%q, using default %d", key, value, fallback)
		return fallback
	}
	return n
}
