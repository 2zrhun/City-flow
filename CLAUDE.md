# CLAUDE.md — CityFlow Analytics

## Project overview

CityFlow Analytics is a student Smart City project: real-time IoT traffic data ingestion, congestion prediction (T+30 min), rerouting recommendations, and a live dashboard. Delivered in 5 days with zero cloud budget; runs locally via Docker Compose or Kubernetes (Docker Desktop) with GitOps (ArgoCD + Helm).

**Team**: Hamza (infra/ops), Walid (backend/API), Hugo (frontend/UX).
**Language**: French docs, English code.

## Architecture

```
Simulator (Node.js) → MQTT (Mosquitto) → Collector (Go) → TimescaleDB
                                              |                ↕
                                              +→ Redis ← Predictor / Rerouter (Go)
                                                  ↑               ↓
                                           API (Go/Gin) ← WebSocket pub/sub
                                              ↓
                                         Dashboard (vanilla JS + Chart.js)
Observability: Prometheus + Loki/Promtail + Grafana
CI/CD: GitHub Actions → GHCR
```

### Services

| Service | Tech | Description |
|---------|------|-------------|
| simulator | Node.js, mqtt.js | Emulates 10 IoT sensors across 5 Paris roads, publishes every 2s |
| collector | Go 1.22, pgx, paho.mqtt, go-redis | MQTT → TimescaleDB + Redis pub/sub |
| predictor | Go 1.24, pgx, gonum, go-redis | EWMA + linear regression (`ewma-lr-v2`), T+30 prediction every 60s |
| rerouter | Go 1.22, pgx, go-redis | Static adjacency map, reroutes on congestion > 0.5, CO2/ETA estimates |
| backend-api-auth | Go 1.24, Gin, GORM, JWT, go-redis, gorilla/websocket | REST + WebSocket + JWT + Redis cache + cursor pagination |
| dashboard | vanilla JS, Chart.js, nginx | SVG map, real-time metrics, predictions, rerouting panels |

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

Pagination: `?limit=50&before=<RFC3339>&road_id=<id>` → `{"data": [...], "next_cursor": "...", "has_more": true}`

## Repository layout

```
City-flow/
├── Backend_API_Auth/       # Go API (Gin + GORM + JWT + Redis + WebSocket)
│   ├── cmd/api/main.go
│   ├── config/             # Env-based config + tests
│   ├── models/             # GORM models (user, traffic, prediction, reroute)
│   ├── handlers/           # HTTP handlers
│   ├── middleware/          # JWT auth + CORS
│   ├── services/           # Auth service + Redis cache + tests
│   ├── Migrations/         # SQL migrations
│   └── Dockerfile
├── services/
│   ├── collector/          # MQTT→DB+Redis ingestion + tests
│   ├── predictor/          # EWMA+LR prediction engine + tests
│   ├── rerouter/           # Rerouting engine + tests
│   └── dashboard/          # Vanilla JS + Chart.js + nginx
├── simulator/              # Node.js IoT traffic simulator
├── ops/                    # Mosquitto, Prometheus, Loki, Promtail, Grafana, TimescaleDB configs
├── charts/cityflow/        # Helm chart (full stack)
├── argocd/                 # GitOps (AppProject + Application)
├── scripts/                # Bootstrap scripts (K3s + ArgoCD)
├── .github/workflows/      # CI/CD pipeline
├── docker-compose.yml      # Full local dev stack
└── .env.example            # Env vars template
```

## Common commands

```bash
# Local dev
cp .env.example .env
docker compose up -d --build
docker compose logs -f simulator collector predictor

# Go tests
cd services/collector && go test -v ./...
cd services/predictor && go test -v ./...
cd services/rerouter && go test -v ./...
cd Backend_API_Auth && go test -v ./...

# K8s + ArgoCD
./scripts/bootstrap-argocd.sh
```

## Database schema (TimescaleDB)

- **traffic_raw** — hypertable, PK `(ts, sensor_id)`
- **predictions** — hypertable, PK `(ts, road_id, horizon_min)`
- **reroutes** — hypertable, PK `(ts, route_id, alt_route_id)`
- **users** — auto-migrated by GORM

## Redis channels

- `cityflow:live` — raw traffic (from collector)
- `cityflow:predictions` — predictions (from predictor)
- `cityflow:reroutes` — rerouting suggestions (from rerouter)

## ML Model (predictor ewma-lr-v2)

1. Query 5-min `time_bucket()` over 30-min lookback
2. Congestion score per bucket: `0.4*speedScore + 0.4*occupancy + 0.2*flowScore`
3. Linear regression (gonum) → extrapolate to now + 30min
4. EWMA blend: `0.7*predicted + 0.3*current`
5. Rush hour factor: x1.15 (7-9h/17-19h), x0.85 (21-6h)
6. Clamp [0, 1]

## Port mappings (docker-compose)

| Port | Service |
|------|---------|
| 1883 | MQTT |
| 3000 | Grafana |
| 3001 | Dashboard |
| 3100 | Loki |
| 5432 | TimescaleDB |
| 6379 | Redis |
| 8080 | Collector |
| 8081 | Backend API |
| 8083 | Predictor |
| 8084 | Rerouter |
| 9090 | Prometheus |

## Code conventions

- Go services: env var config with `getEnv(key, fallback)`, multi-stage Docker builds, CGO_ENABLED=0
- Prometheus `/metrics` + `/health` on every service
- Cache-aside: Redis → miss → DB → store with TTL
- Cursor pagination via `ts` column, `limit+1` trick
- JWT via `Authorization: Bearer <token>`; WebSocket via `?token=`
- Never `db.AutoMigrate` on TimescaleDB-managed tables
