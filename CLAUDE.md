# CLAUDE.md — CityFlow Analytics

## Project overview

CityFlow Analytics is a student Smart City project: real-time IoT traffic data ingestion, congestion prediction (T+30 min), rerouting recommendations, and a live dashboard. The MVP targets a 5-day delivery with zero cloud budget; everything runs locally via Docker Compose or K3s.

**Team**: Hamza (infra/ops), Walid (backend/API), Hugo (frontend/UX).
**Language**: French docs, English code.

## Architecture (5 layers)

```
Simulator (Node.js) → MQTT (Mosquitto) → Collector (Go) → TimescaleDB
                                              |                ↕
                                              +→ Redis ← Predictor / Rerouter (Go, not yet built)
                                                  ↑               ↓
                                           API (Go/Gin) ← WebSocket pub/sub
                                              ↓
                                         Dashboard (not yet built)
Observability: Prometheus + Loki/Promtail + Grafana
```

### What exists today
- **simulator/** — Node.js (mqtt lib), publishes fake sensor data every 2 s to `cityflow/traffic/{sensor_id}`.
- **services/collector/** — Go (pgx + paho.mqtt + prometheus + go-redis), subscribes to MQTT, validates payloads, inserts into `traffic_raw`, publishes to Redis `cityflow:live` channel. Exposes `/health` and `/metrics` on :8080.
- **Backend_API_Auth/** — Go (Gin + GORM + JWT + Redis), full REST API with:
  - JWT authentication (register/login/logout)
  - CORS middleware
  - Cursor-based pagination on all data endpoints
  - Redis caching (5s TTL for traffic, 30s for predictions/reroutes)
  - WebSocket via Redis pub/sub (`/ws/live`)
- **ops/** — Config files for Mosquitto, Prometheus, Loki, Promtail, Grafana (datasource provisioning).
- **k8s/** — Raw K8s manifests (namespace, secrets, deployments, services, observability).
- **charts/cityflow/** — Helm chart wrapping the full stack, with `values.yaml` (dev) and `values-prod.yaml`.
- **argocd/** — AppProject + Application CRDs for GitOps sync via ArgoCD.
- **scripts/** — `bootstrap-k3s.sh` and `bootstrap-argocd.sh`.

### Not yet built
- `predictor` service (congestion forecast T+30)
- `rerouter` service (alternative routes)
- Frontend dashboard (OpenStreetMap + D3.js)

## API routes

```
GET    /health                          [public]
POST   /api/auth/register               [public]
POST   /api/auth/login                  [public]
POST   /api/auth/logout                 [JWT required]
GET    /api/traffic/live                 [JWT required, paginated, cached 5s]
GET    /api/predictions?horizon=30       [JWT required, paginated, cached 30s]
GET    /api/reroutes/recommended         [JWT required, paginated, cached 30s]
WS     /ws/live?token=<jwt>              [token via query param]
```

### Pagination
All GET data endpoints support cursor-based pagination:
- `?limit=50` (default 50, max 200)
- `?before=<RFC3339 timestamp>` (cursor for next page)
- `?road_id=<id>` (optional filter)
- Response: `{"data": [...], "next_cursor": "...", "has_more": true}`

## Key tech stack

| Component | Tech | Version |
|---|---|---|
| Simulator | Node.js 20, mqtt.js | 5.10.x |
| Collector | Go 1.22, pgx/v5, paho.mqtt, prometheus client, go-redis | see go.mod |
| Backend API Auth | Go 1.24, Gin, GORM, golang-jwt/v5, gin-contrib/cors, go-redis, gorilla/websocket | see go.mod |
| Database | TimescaleDB (Postgres 16) | latest-pg16 |
| Broker | Eclipse Mosquitto | 2.x |
| Cache/PubSub | Redis 7 Alpine | 7.x |
| Observability | Prometheus, Grafana, Loki 2.9.8, Promtail 2.9.8 | latest/2.9.8 |
| Orchestration | Docker Compose / K3s / ArgoCD + Helm | — |

## Repository layout

```
City-flow/
├── Backend_API_Auth/       # Go API service (Gin + GORM + JWT + Redis)
│   ├── cmd/api/main.go     # Entrypoint — wires all components
│   ├── config/config.go    # Env-based config (DB, JWT, Redis, CORS, WS)
│   ├── models/             # GORM models
│   │   ├── user.go         # User (id, email, password hash, role)
│   │   ├── traffic.go      # TrafficRaw (maps traffic_raw table)
│   │   ├── prediction.go   # Prediction (maps predictions table)
│   │   └── reroute.go      # Reroute (maps reroutes table)
│   ├── handlers/           # Gin HTTP handlers
│   │   ├── auth.go         # Register, Login, Logout
│   │   ├── traffic.go      # GET /api/traffic/live
│   │   ├── prediction.go   # GET /api/predictions
│   │   ├── reroute.go      # GET /api/reroutes/recommended
│   │   ├── websocket.go    # WS /ws/live (Redis pub/sub)
│   │   ├── health.go       # GET /health
│   │   └── pagination.go   # Cursor-based pagination helper
│   ├── middleware/          # Gin middleware
│   │   ├── auth.go         # JWT Bearer token validation
│   │   └── cors.go         # CORS setup (gin-contrib/cors)
│   ├── services/           # Business logic / infra clients
│   │   ├── auth_service.go # bcrypt + JWT generate/validate
│   │   └── cache.go        # Redis client (Get/Set/Publish/Subscribe)
│   ├── Migrations/         # SQL migrations
│   │   ├── 000002_create_users.up.sql
│   │   └── 000002_create_users.down.sql
│   ├── Dockerfile          # Multi-stage Go build
│   └── Makefile            # build/run/test/migrate/docker commands
├── services/
│   └── collector/          # Go MQTT→DB+Redis ingestion service
│       ├── main.go         # Subscribes MQTT, inserts DB, publishes Redis
│       └── Dockerfile
├── simulator/              # Node.js IoT traffic simulator
│   ├── index.js            # Single-file simulator
│   └── Dockerfile
├── ops/                    # Config for infra services
│   ├── mosquitto/          # mosquitto.conf
│   ├── prometheus/         # prometheus.yml (scrapes collector)
│   ├── loki/               # Loki config
│   ├── promtail/           # Promtail config (Docker log scraping)
│   ├── grafana/            # Datasource provisioning (Prometheus + Loki)
│   └── timescaledb/init/   # SQL init scripts (001 timescaledb ext, 002 schema)
├── k8s/                    # Raw Kubernetes manifests
├── charts/cityflow/        # Helm chart (wraps everything above)
├── argocd/                 # GitOps config
├── scripts/                # Bootstrap scripts (K3s + ArgoCD)
├── docker-compose.yml      # Full local dev stack (includes Redis)
├── .env.example            # Env vars template
└── README.md / ROADMAP.md  # Architecture docs & project plan
```

## Common commands

```bash
# Local dev (Docker Compose) — full stack
cp .env.example .env
docker compose up -d --build
docker compose logs -f simulator
docker compose logs -f collector
docker compose logs -f backend-api-auth
docker compose down

# Backend API Auth — standalone dev
cd Backend_API_Auth
make run              # requires local Postgres + Redis
make build            # produces ./traffic-api binary
make test             # go test ./...
make docker-up        # spins up API + Postgres + Redis

# K3s + ArgoCD deployment
./scripts/bootstrap-k3s.sh
./scripts/bootstrap-argocd.sh

# Test API endpoints
curl http://localhost:8081/health
curl -X POST http://localhost:8081/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@cityflow.dev","password":"password123"}'
```

## Database schema (TimescaleDB)

Initialized by `ops/timescaledb/init/002-schema.sql`:

- **traffic_raw** — `(ts TIMESTAMPTZ, sensor_id TEXT, road_id TEXT, speed_kmh, flow_rate, occupancy)` — PK `(ts, sensor_id)`, hypertable on `ts`.
- **predictions** — `(ts, road_id, horizon_min, congestion_score, confidence, model_version)` — PK `(ts, road_id, horizon_min)`.
- **reroutes** — `(ts, route_id, alt_route_id, reason, estimated_co2_gain, eta_gain_min)` — PK `(ts, route_id, alt_route_id)`.

Auto-migrated by Backend API Auth at startup:

- **users** — `(id BIGSERIAL, email UNIQUE, password TEXT, role TEXT DEFAULT 'operator', created_at, updated_at)`.

## MQTT topics

- `cityflow/traffic/{sensor_id}` — JSON payload with `ts`, `sensor_id`, `road_id`, `speed_kmh`, `flow_rate`, `occupancy`.

## Redis channels

- `cityflow:live` — Published by collector after each DB insert, subscribed by WebSocket handler for real-time push.

## Environment variables

See `.env.example` for the full list. Key ones:
- `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` — DB credentials
- `SIM_PUBLISH_INTERVAL_MS`, `SIM_SENSOR_COUNT` — Simulator tuning
- `GRAFANA_ADMIN_USER`, `GRAFANA_ADMIN_PASSWORD` — Grafana login
- `JWT_SECRET`, `JWT_EXPIRY_HOURS` — JWT auth config
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` — Redis config
- `CORS_ALLOWED_ORIGINS` — CORS policy (`*` for dev)

The collector uses `DB_DSN`, `MQTT_URL`, and `REDIS_URL`; the Backend API Auth uses individual `DB_HOST`, `DB_PORT`, etc. plus `REDIS_HOST`, `REDIS_PORT`.

## Local service URLs (docker compose)

- Collector health/metrics: `http://localhost:8080`
- Backend API Auth: `http://localhost:8081/health`
- Grafana: `http://localhost:3000` (admin/admin)
- Prometheus: `http://localhost:9090`
- Loki: `http://localhost:3100`
- MQTT broker: `localhost:1883`
- TimescaleDB: `localhost:5432`
- Redis: `localhost:6379`

## Code conventions

- Go services use env var config (no config files). Helper `getEnv(key, fallback)` pattern.
- Backend API Auth uses a structured layout: `models/`, `handlers/`, `middleware/`, `services/`.
- Handler pattern: struct with `*gorm.DB` and `*CacheService`, factory constructor (`NewXxxHandler`).
- Cache-aside pattern: check Redis, miss → query DB → store in Redis with TTL.
- Cursor-based pagination using `ts` column with `limit+1` trick for `has_more`.
- JWT auth via `Authorization: Bearer <token>` header; WebSocket uses `?token=` query param.
- Multi-stage Docker builds: Go builder stage → minimal Alpine runtime.
- Prometheus metrics exposed via `/metrics` endpoint on collector.
- Health checks via `/health` endpoint on every service.
- Never call `db.AutoMigrate` on `TrafficRaw`, `Prediction`, or `Reroute` — those tables are managed by TimescaleDB init scripts with hypertables.

## Testing

- Backend API Auth: `make test` (or `go test -v ./...` from Backend_API_Auth/)
- Collector: `go test -v ./...` from services/collector/ (no tests yet)
- Simulator: no test setup

## Important notes for contributors

- Never commit `.env` files — only `.env.example` with dev-safe defaults.
- The collector and backend-api-auth both listen on port 8080 inside containers but are mapped to different host ports in docker-compose (8080 and 8081).
- Mosquitto runs with `allow_anonymous true` in dev; production requires auth (`values-prod.yaml` sets `allowAnonymous: false`).
- ArgoCD `application-cityflow.yaml` has a placeholder `repoURL` — update it to point to your actual Git repo.
- The Helm chart images default to `cityflow/*:latest` — you must build and push images to a registry accessible by K3s before ArgoCD sync.
- Redis is required for the Backend API Auth service to start. In docker-compose, it depends on Redis being healthy.
- The `users` table is auto-migrated by GORM at startup. The SQL migration in `Migrations/000002_create_users.up.sql` is for production reference.
