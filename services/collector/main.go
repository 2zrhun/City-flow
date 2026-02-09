package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type TrafficPayload struct {
	TS        string  `json:"ts"`
	SensorID  string  `json:"sensor_id"`
	RoadID    string  `json:"road_id"`
	SpeedKMH  float64 `json:"speed_kmh"`
	FlowRate  float64 `json:"flow_rate"`
	Occupancy float64 `json:"occupancy"`
}

var (
	msgsReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_collector_messages_received_total",
		Help: "Total number of MQTT messages received by collector.",
	})
	msgsStored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_collector_messages_stored_total",
		Help: "Total number of messages successfully inserted into TimescaleDB.",
	})
	msgsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cityflow_collector_messages_failed_total",
		Help: "Total number of messages rejected or failed to store.",
	})
)

var redisClient *redis.Client

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbDSN := getEnv("DB_DSN", "postgres://cityflow:cityflow_dev_password@localhost:5432/cityflow?sslmode=disable")
	mqttURL := getEnv("MQTT_URL", "tcp://localhost:1883")
	mqttTopic := getEnv("MQTT_TOPIC", "cityflow/traffic/+")
	metricsAddr := getEnv("METRICS_ADDR", ":8080")
	redisURL := getEnv("REDIS_URL", "")

	dbPool, err := pgxpool.New(ctx, dbDSN)
	if err != nil {
		log.Fatalf("db pool init failed: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("db ping failed: %v", err)
	}

	if redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("invalid REDIS_URL, skipping Redis: %v", err)
		} else {
			redisClient = redis.NewClient(opts)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("redis ping failed, skipping Redis: %v", err)
				redisClient = nil
			} else {
				log.Printf("redis connected: %s", redisURL)
			}
		}
	}

	go serveHTTP(metricsAddr)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttURL)
	opts.SetClientID("collector-" + time.Now().Format("20060102150405"))
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetDefaultPublishHandler(func(client mqtt.Client, message mqtt.Message) {
		processMessage(ctx, dbPool, message.Payload())
	})
	opts.OnConnect = func(client mqtt.Client) {
		token := client.Subscribe(mqttTopic, 0, nil)
		token.Wait()
		if token.Error() != nil {
			log.Printf("mqtt subscribe error: %v", token.Error())
			return
		}
		log.Printf("collector subscribed to topic=%s", mqttTopic)
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("mqtt connection lost: %v", err)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		log.Fatalf("mqtt connection failed: %v", token.Error())
	}

	log.Printf("collector running, mqtt=%s db=ok metrics=%s", mqttURL, metricsAddr)

	<-ctx.Done()
	log.Printf("collector shutting down")
	client.Disconnect(250)
	if redisClient != nil {
		redisClient.Close()
	}
}

func serveHTTP(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
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

func processMessage(ctx context.Context, dbPool *pgxpool.Pool, payloadRaw []byte) {
	msgsReceived.Inc()

	var payload TrafficPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		msgsFailed.Inc()
		log.Printf("invalid payload: %v", err)
		return
	}

	ts := time.Now().UTC()
	if payload.TS != "" {
		parsed, err := time.Parse(time.RFC3339, payload.TS)
		if err == nil {
			ts = parsed.UTC()
		}
	}

	if payload.SensorID == "" || payload.RoadID == "" {
		msgsFailed.Inc()
		log.Printf("missing required fields in payload")
		return
	}

	_, err := dbPool.Exec(ctx, `
		INSERT INTO traffic_raw (ts, sensor_id, road_id, speed_kmh, flow_rate, occupancy)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (ts, sensor_id) DO NOTHING
	`, ts, payload.SensorID, payload.RoadID, payload.SpeedKMH, payload.FlowRate, payload.Occupancy)
	if err != nil {
		msgsFailed.Inc()
		log.Printf("db insert failed: %v", err)
		return
	}

	msgsStored.Inc()

	if redisClient != nil {
		_ = redisClient.Publish(ctx, "cityflow:live", payloadRaw).Err()
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
