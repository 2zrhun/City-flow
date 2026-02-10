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
)

const (
	maxSpeed = 90.0
	maxFlow  = 120.0
)

type Prediction struct {
	TS              time.Time `json:"ts"`
	RoadID          string    `json:"road_id"`
	HorizonMin      int       `json:"horizon_min"`
	CongestionScore float64   `json:"congestion_score"`
	Confidence      float64   `json:"confidence"`
	ModelVersion    string    `json:"model_version"`
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
	modelVersion := getEnv("MODEL_VERSION", "baseline-v1")

	// DB pool
	dbPool, err := pgxpool.New(ctx, dbDSN)
	if err != nil {
		log.Fatalf("db pool init failed: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}
	log.Printf("db connected")

	// Redis (required for real-time)
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

	// HTTP health + metrics
	go serveHTTP(metricsAddr)

	// Prediction loop
	interval := time.Duration(intervalSec) * time.Second
	lookback := time.Duration(lookbackMin) * time.Minute

	log.Printf("predictor running: interval=%s lookback=%s horizon=%dm model=%s",
		interval, lookback, horizonMin, modelVersion)

	// Run first cycle immediately
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

	rows, err := dbPool.Query(ctx, `
		SELECT road_id, AVG(speed_kmh), AVG(occupancy), AVG(flow_rate), COUNT(*)
		FROM traffic_raw
		WHERE ts >= $1
		GROUP BY road_id
	`, now.Add(-lookback))
	if err != nil {
		predictionsFailed.Inc()
		log.Printf("query traffic_raw failed: %v", err)
		return
	}
	defer rows.Close()

	var predictions []Prediction
	for rows.Next() {
		var roadID string
		var avgSpeed, avgOccupancy, avgFlow float64
		var sampleCount int64

		if err := rows.Scan(&roadID, &avgSpeed, &avgOccupancy, &avgFlow, &sampleCount); err != nil {
			predictionsFailed.Inc()
			log.Printf("row scan failed: %v", err)
			continue
		}

		score := computeCongestionScore(avgSpeed, avgOccupancy, avgFlow)
		confidence := math.Min(1.0, float64(sampleCount)/100.0)

		predictions = append(predictions, Prediction{
			TS:              now,
			RoadID:          roadID,
			HorizonMin:      horizonMin,
			CongestionScore: score,
			Confidence:      confidence,
			ModelVersion:    modelVersion,
		})
		predictionsGenerated.Inc()
	}
	if rows.Err() != nil {
		predictionsFailed.Inc()
		log.Printf("rows iteration error: %v", rows.Err())
		return
	}

	if len(predictions) == 0 {
		log.Printf("no traffic data in lookback window, skipping")
		return
	}

	// Store predictions
	stored := storePredictions(ctx, dbPool, predictions)

	// Publish to Redis
	published := publishPredictions(ctx, redisClient, predictions)

	log.Printf("prediction cycle completed: %d roads, %d stored, %d published (%.2fs)",
		len(predictions), stored, published, time.Since(start).Seconds())
}

func computeCongestionScore(avgSpeed, avgOccupancy, avgFlow float64) float64 {
	speedScore := 1.0 - (avgSpeed / maxSpeed)
	occupancyScore := avgOccupancy
	flowScore := avgFlow / maxFlow

	score := 0.4*speedScore + 0.4*occupancyScore + 0.2*flowScore

	return math.Max(0.0, math.Min(1.0, score))
}

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
