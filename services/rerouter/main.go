package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

// Route adjacency map: for each road, which roads are valid alternatives.
var adjacencyMap = map[string][]string{
	"RING-NORTH-12":      {"RING-SOUTH-09", "CITY-CENTER-01"},
	"RING-SOUTH-09":      {"RING-NORTH-12", "CITY-CENTER-01"},
	"CITY-CENTER-01":     {"RING-NORTH-12", "RING-SOUTH-09"},
	"AIRPORT-AXIS-03":    {"RING-SOUTH-09", "UNIVERSITY-LOOP-07"},
	"UNIVERSITY-LOOP-07": {"AIRPORT-AXIS-03", "CITY-CENTER-01"},
}

type RoadPrediction struct {
	RoadID          string
	CongestionScore float64
}

type Reroute struct {
	TS               time.Time `json:"ts"`
	RouteID          string    `json:"route_id"`
	AltRouteID       string    `json:"alt_route_id"`
	Reason           string    `json:"reason"`
	EstimatedCO2Gain *float64  `json:"estimated_co2_gain"`
	ETAGainMin       *float64  `json:"eta_gain_min"`
}

var (
	reroutesGenerated = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_rerouter_reroutes_generated_total",
		Help: "Total number of reroute recommendations generated.",
	})
	reroutesStored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_rerouter_reroutes_stored_total",
		Help: "Total number of reroutes stored in DB.",
	})
	reroutesFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_rerouter_reroutes_failed_total",
		Help: "Total number of reroute failures.",
	})
	reroutesPublished = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_rerouter_reroutes_published_total",
		Help: "Total number of reroutes published to Redis.",
	})
	cycleDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cityflow_rerouter_cycle_duration_seconds",
		Help:    "Duration of a full reroute cycle.",
		Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0},
	})
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbDSN := getEnv("DB_DSN", "postgres://cityflow:cityflow_dev_password@localhost:5432/cityflow?sslmode=disable")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	metricsAddr := getEnv("METRICS_ADDR", ":8080")
	intervalSec := getEnvInt("REROUTE_INTERVAL_SEC", 60)
	threshold := getEnvFloat("CONGESTION_THRESHOLD", 0.5)

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

	// Redis (required)
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

	interval := time.Duration(intervalSec) * time.Second

	log.Printf("rerouter running: interval=%s threshold=%.2f", interval, threshold)

	// Run first cycle immediately
	runCycle(ctx, dbPool, redisClient, threshold)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runCycle(ctx, dbPool, redisClient, threshold)
		case <-ctx.Done():
			log.Printf("rerouter shutting down")
			return
		}
	}
}

func runCycle(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client, threshold float64) {
	start := time.Now()
	defer func() {
		cycleDuration.Observe(time.Since(start).Seconds())
	}()

	now := time.Now().UTC().Truncate(time.Second)

	// Get latest prediction per road
	rows, err := dbPool.Query(ctx, `
		SELECT DISTINCT ON (road_id) road_id, congestion_score
		FROM predictions
		ORDER BY road_id, ts DESC
	`)
	if err != nil {
		reroutesFailed.Inc()
		log.Printf("query predictions failed: %v", err)
		return
	}
	defer rows.Close()

	// Build map of road -> congestion score
	scores := make(map[string]float64)
	for rows.Next() {
		var rp RoadPrediction
		if err := rows.Scan(&rp.RoadID, &rp.CongestionScore); err != nil {
			reroutesFailed.Inc()
			log.Printf("row scan failed: %v", err)
			continue
		}
		scores[rp.RoadID] = rp.CongestionScore
	}
	if rows.Err() != nil {
		reroutesFailed.Inc()
		log.Printf("rows iteration error: %v", rows.Err())
		return
	}

	if len(scores) == 0 {
		log.Printf("no predictions available, skipping")
		return
	}

	// Generate reroute recommendations
	var reroutes []Reroute
	for roadID, score := range scores {
		if score <= threshold {
			continue
		}

		alternatives, ok := adjacencyMap[roadID]
		if !ok {
			continue
		}

		// Find the least congested alternative
		bestAlt := ""
		bestAltScore := 1.0
		for _, alt := range alternatives {
			altScore, exists := scores[alt]
			if !exists {
				continue
			}
			if altScore < bestAltScore {
				bestAlt = alt
				bestAltScore = altScore
			}
		}

		if bestAlt == "" {
			continue
		}

		// Only recommend if alternative is meaningfully better
		delta := score - bestAltScore
		if delta < 0.1 {
			continue
		}

		etaGain := delta * 15.0
		co2Gain := delta * 2.5
		reason := fmt.Sprintf("high-congestion: %.2f on %s, reroute to %s (%.2f)", score, roadID, bestAlt, bestAltScore)

		reroutes = append(reroutes, Reroute{
			TS:               now,
			RouteID:          roadID,
			AltRouteID:       bestAlt,
			Reason:           reason,
			EstimatedCO2Gain: &co2Gain,
			ETAGainMin:       &etaGain,
		})
		reroutesGenerated.Inc()
	}

	if len(reroutes) == 0 {
		log.Printf("reroute cycle: no congested roads above threshold %.2f (scores: %v)", threshold, scores)
		return
	}

	stored := storeReroutes(ctx, dbPool, reroutes)
	published := publishReroutes(ctx, redisClient, reroutes)

	log.Printf("reroute cycle completed: %d recommendations, %d stored, %d published (%.2fs)",
		len(reroutes), stored, published, time.Since(start).Seconds())
}

func storeReroutes(ctx context.Context, dbPool *pgxpool.Pool, reroutes []Reroute) int {
	stored := 0
	for _, r := range reroutes {
		_, err := dbPool.Exec(ctx, `
			INSERT INTO reroutes (ts, route_id, alt_route_id, reason, estimated_co2_gain, eta_gain_min)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (ts, route_id, alt_route_id) DO UPDATE SET
				reason = EXCLUDED.reason,
				estimated_co2_gain = EXCLUDED.estimated_co2_gain,
				eta_gain_min = EXCLUDED.eta_gain_min
		`, r.TS, r.RouteID, r.AltRouteID, r.Reason, r.EstimatedCO2Gain, r.ETAGainMin)
		if err != nil {
			reroutesFailed.Inc()
			log.Printf("db insert failed for route=%s: %v", r.RouteID, err)
			continue
		}
		reroutesStored.Inc()
		stored++
	}
	return stored
}

func publishReroutes(ctx context.Context, redisClient *redis.Client, reroutes []Reroute) int {
	published := 0
	for _, r := range reroutes {
		data, err := json.Marshal(r)
		if err != nil {
			log.Printf("json marshal failed for route=%s: %v", r.RouteID, err)
			continue
		}
		if err := redisClient.Publish(ctx, "cityflow:reroutes", data).Err(); err != nil {
			log.Printf("redis publish failed for route=%s: %v", r.RouteID, err)
			continue
		}
		reroutesPublished.Inc()
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

func getEnvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Printf("invalid %s=%q, using default %.2f", key, value, fallback)
		return fallback
	}
	return f
}
