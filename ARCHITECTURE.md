# CityFlow — Architecture Complete

## Vue d'ensemble

```mermaid
flowchart TB
    subgraph SOURCES["Source de Donnees"]
        OPENDATA[("Paris Open Data API<br/>~100 capteurs reels<br/>comptages-routiers-permanents")]
    end

    subgraph IOT["Couche IoT"]
        SIM["Simulateur<br/>Node.js<br/>Fetch toutes les 2s"]
    end

    subgraph INGESTION["Couche Ingestion"]
        MQTT["Mosquitto<br/>Broker MQTT<br/>Port 1883"]
        COL["Collector<br/>Go 1.22<br/>pgx + paho.mqtt"]
    end

    subgraph STORAGE["Couche Stockage"]
        TSDB[("TimescaleDB<br/>PostgreSQL 16<br/>Hypertables")]
        REDIS[("Redis 7<br/>Cache + Pub/Sub")]
    end

    subgraph INTELLIGENCE["Couche Intelligence"]
        PRED["Predictor<br/>Go 1.24<br/>gonum/stat<br/>EWMA + LR"]
        RER["Rerouter<br/>Go 1.22<br/>Graphe d'adjacence"]
    end

    subgraph EXPOSITION["Couche Exposition"]
        API["Backend API<br/>Go/Gin 1.24<br/>JWT + GORM + WS"]
        DASH["Dashboard<br/>JS + Leaflet<br/>nginx"]
    end

    subgraph OPS["Couche Operations"]
        PROM["Prometheus"]
        GRAF["Grafana"]
        LOKI["Loki"]
        TAIL["Promtail"]
        KIALI["Kiali"]
    end

    OPENDATA -->|HTTP GET| SIM
    SIM -->|MQTT publish<br/>traffic/data| MQTT
    MQTT -->|Subscribe| COL
    COL -->|INSERT traffic_raw| TSDB
    COL -->|UPSERT roads| TSDB
    COL -->|PUBLISH cityflow:live| REDIS
    TSDB -->|SELECT time_bucket| PRED
    PRED -->|INSERT predictions| TSDB
    PRED -->|PUBLISH cityflow:predictions| REDIS
    TSDB -->|SELECT predictions > 0.5| RER
    RER -->|INSERT reroutes| TSDB
    RER -->|PUBLISH cityflow:reroutes| REDIS
    TSDB -->|GORM queries| API
    REDIS -->|Cache + Sub| API
    API -->|REST + WS| DASH
    COL & PRED & RER & API -->|/metrics| PROM
    PROM --> GRAF
    LOKI --> GRAF
    TAIL -->|logs| LOKI
    PROM --> KIALI
```

## Flux de donnees detaille

```mermaid
sequenceDiagram
    participant OD as Paris Open Data
    participant SIM as Simulateur
    participant MQTT as Mosquitto
    participant COL as Collector
    participant DB as TimescaleDB
    participant RED as Redis
    participant PRED as Predictor
    participant RER as Rerouter
    participant API as Backend API
    participant WS as WebSocket
    participant DASH as Dashboard

    loop Toutes les 2 secondes
        SIM->>OD: GET /records?select=iu_ac,q,k,geo_point_2d
        OD-->>SIM: ~100 capteurs avec GPS
        SIM->>MQTT: PUBLISH traffic/data (JSON par capteur)
    end

    MQTT->>COL: Message MQTT
    COL->>DB: INSERT INTO traffic_raw (ts, sensor_id, road_id, speed, flow, occupancy)
    COL->>DB: UPSERT INTO roads (road_id, label, lat, lng)
    COL->>RED: PUBLISH cityflow:live (JSON payload)

    RED->>API: PubSub message
    API->>WS: WriteJSON traffic_update
    WS->>DASH: onmessage → update liveState

    loop Toutes les 60 secondes
        PRED->>DB: SELECT time_bucket('5 min') FROM traffic_raw (30 min)
        PRED->>PRED: Score congestion + regression lineaire + EWMA
        PRED->>DB: INSERT INTO predictions (congestion_score, confidence)
        PRED->>RED: PUBLISH cityflow:predictions
    end

    loop Toutes les 60 secondes
        RER->>DB: SELECT FROM predictions WHERE congestion_score > 0.5
        RER->>RER: Calcul routes alternatives (graphe adjacence)
        RER->>DB: INSERT INTO reroutes (route_id, alt_route_id, co2_gain)
        RER->>RED: PUBLISH cityflow:reroutes
    end

    DASH->>API: GET /api/roads (liste capteurs + GPS)
    DASH->>API: GET /api/traffic/live (derniers releves)
    DASH->>API: GET /api/predictions?horizon=30
    DASH->>DASH: Calcul proximite route/capteurs (500m)
    DASH->>DASH: Affichage alertes + predictions
```

## Infrastructure Kubernetes + Istio

```mermaid
flowchart TB
    subgraph INTERNET["Client (Navigateur)"]
        USER["Utilisateur<br/>http://cityflow"]
    end

    subgraph ISTIO_SYSTEM["Namespace: istio-system"]
        GW["Istio IngressGateway<br/>Port 80<br/>Host: *"]
        ISTIOD["istiod<br/>Control Plane<br/>mTLS CA"]
        EF["EnvoyFilter<br/>WebSocket upgrade"]
    end

    subgraph CITYFLOW["Namespace: cityflow (istio-injection: enabled)"]
        subgraph VS["VirtualServices"]
            VS1["localhost → dashboard:80"]
            VS2["localhost /ws/* → backend:8080"]
            VS3["grafana.cityflow.local → grafana:3000"]
            VS4["prometheus.cityflow.local → prometheus:9090"]
        end

        subgraph PODS_SIDECAR["Pods avec Sidecar Envoy"]
            DASH_P["dashboard<br/>nginx:80<br/>+ envoy"]
            API_P["backend-api-auth<br/>gin:8080<br/>+ envoy"]
            COL_P["collector<br/>:8080<br/>+ envoy"]
            PRED_P["predictor<br/>:8080<br/>+ envoy"]
            RER_P["rerouter<br/>:8080<br/>+ envoy"]
            MQTT_P["mosquitto<br/>:1883<br/>+ envoy"]
            TSDB_P["timescaledb<br/>:5432<br/>+ envoy"]
            REDIS_P["redis<br/>:6379<br/>+ envoy"]
        end

        subgraph PODS_NO_SIDECAR["Pods sans Sidecar"]
            SIM_P["simulator"]
            PROM_P["prometheus<br/>:9090"]
            GRAF_P["grafana<br/>:3000"]
            LOKI_P["loki<br/>:3100"]
            TAIL_P["promtail"]
        end
    end

    subgraph ARGOCD_NS["Namespace: argocd"]
        ARGO["ArgoCD Server<br/>GitOps auto-sync"]
    end

    subgraph GITHUB["GitHub"]
        REPO["2zrhun/City-flow<br/>main branch"]
        GHCR["GHCR<br/>6 images Docker"]
        GHA["GitHub Actions<br/>CI/CD Pipeline"]
    end

    USER -->|HTTP| GW
    GW --> VS
    VS1 --> DASH_P
    VS2 --> API_P
    VS3 --> GRAF_P
    VS4 --> PROM_P
    ISTIOD -.->|mTLS certs| PODS_SIDECAR

    ARGO -->|Helm sync| CITYFLOW
    REPO -->|Watch main| ARGO
    GHA -->|Push images| GHCR
    REPO -->|Trigger| GHA
    GHCR -.->|Pull| CITYFLOW
```

## Pipeline CI/CD

```mermaid
flowchart LR
    subgraph TRIGGER["Declencheur"]
        PUSH["git push main"]
        PR["Pull Request"]
    end

    subgraph JOB1["Job: go-build (matrix x4)"]
        direction TB
        GO1["collector<br/>Go 1.22"]
        GO2["predictor<br/>Go 1.24"]
        GO3["rerouter<br/>Go 1.22"]
        GO4["backend-api-auth<br/>Go 1.24"]
        GO1 & GO2 & GO3 & GO4 --> TEST["go test ./... -v"]
    end

    subgraph JOB2["Job: node-build"]
        NODE["simulator<br/>npm install"]
    end

    subgraph JOB3["Job: docker-push (matrix x6)"]
        direction TB
        BUILD["docker/build-push-action<br/>platforms: amd64 + arm64"]
        BUILD --> TAG1[":latest"]
        BUILD --> TAG2[":sha-abc123"]
    end

    subgraph REGISTRY["GHCR"]
        IMG1["cityflow-collector"]
        IMG2["cityflow-predictor"]
        IMG3["cityflow-rerouter"]
        IMG4["cityflow-backend-api-auth"]
        IMG5["cityflow-simulator"]
        IMG6["cityflow-dashboard"]
    end

    subgraph DEPLOY["GitOps"]
        ARGO2["ArgoCD<br/>auto-sync + self-heal"]
        K8S["Kubernetes<br/>rolling update"]
    end

    PUSH & PR --> JOB1 & JOB2
    JOB1 & JOB2 -->|"if push main"| JOB3
    JOB3 --> REGISTRY
    REGISTRY --> ARGO2
    ARGO2 --> K8S
```

## Modele de prediction (ewma-lr-v2)

```mermaid
flowchart TB
    subgraph INPUT["Donnees d'entree"]
        RAW["traffic_raw<br/>30 dernieres minutes"]
    end

    subgraph AGG["1. Aggregation"]
        BUCKET["time_bucket('5 min')<br/>→ 6 points par route"]
    end

    subgraph SCORE["2. Score de congestion"]
        CALC["0.4 x (1 - vitesse/90)<br/>+ 0.4 x occupation<br/>+ 0.2 x debit/120<br/>→ score [0, 1]"]
    end

    subgraph ML["3. Regression lineaire"]
        LR["gonum LinearRegression<br/>sur les 6 scores"]
        EXTRAP["Extrapolation T+30 min<br/>score + slope x 6"]
    end

    subgraph SMOOTH["4. Lissage"]
        EWMA["EWMA<br/>0.7 x prediction<br/>+ 0.3 x score_actuel"]
    end

    subgraph ADJUST["5. Ajustements"]
        RUSH["Heure de pointe<br/>x1.15 (7-9h, 17-19h)<br/>x0.85 (21-6h)"]
        CONF["Confiance<br/>nb echantillons<br/>+ stabilite tendance"]
    end

    subgraph OUTPUT["Sortie"]
        PRED_OUT["predictions<br/>congestion_score<br/>confidence<br/>model: ewma-lr-v2"]
    end

    RAW --> BUCKET --> CALC --> LR --> EXTRAP --> EWMA --> RUSH --> CONF --> PRED_OUT
```

## Schema de la base de donnees

```mermaid
erDiagram
    TRAFFIC_RAW {
        timestamptz ts PK
        text sensor_id
        text road_id FK
        float speed_kmh
        float flow_rate
        float occupancy
    }

    ROADS {
        text road_id PK
        text label
        float lat
        float lng
        timestamptz updated_at
    }

    PREDICTIONS {
        timestamptz ts PK
        text road_id FK
        int horizon_min
        float congestion_score
        float confidence
        text model_version
    }

    REROUTES {
        timestamptz ts PK
        text route_id FK
        text alt_route_id FK
        text reason
        float estimated_co2_gain
        float eta_gain_min
    }

    USERS {
        serial id PK
        text email UK
        text password
        text role
        timestamptz created_at
        timestamptz updated_at
    }

    ROADS ||--o{ TRAFFIC_RAW : "road_id"
    ROADS ||--o{ PREDICTIONS : "road_id"
    ROADS ||--o{ REROUTES : "route_id / alt_route_id"
```

## Reseau Istio Service Mesh

```mermaid
flowchart LR
    subgraph MTLS["mTLS PERMISSIVE"]
        direction TB

        A["backend-api-auth"] <-->|"mTLS auto"| B["timescaledb"]
        A <-->|"mTLS auto"| C["redis"]
        D["collector"] <-->|"mTLS auto"| B
        D <-->|"mTLS auto"| E["mosquitto"]
        D <-->|"mTLS auto"| C
        F["predictor"] <-->|"mTLS auto"| B
        F <-->|"mTLS auto"| C
        G["rerouter"] <-->|"mTLS auto"| B
        G <-->|"mTLS auto"| C
    end

    subgraph DR["DestinationRules"]
        DR1["backend-api-auth<br/>100 TCP / no H2"]
        DR2["mosquitto<br/>50 TCP / 10s timeout"]
        DR3["timescaledb<br/>50 TCP / 10s timeout"]
        DR4["redis<br/>100 TCP / 5s timeout"]
    end
```

## Stack par service

| Service | Langage | Framework | Base | Cache | Port | Dockerfile |
|---------|---------|-----------|------|-------|------|-----------|
| simulator | Node.js 20 | mqtt.js | — | — | — | `simulator/Dockerfile` |
| collector | Go 1.22 | pgx/v5, paho.mqtt | TimescaleDB | Redis pub | 8080 | `services/collector/Dockerfile` |
| predictor | Go 1.24 | pgx/v5, gonum/stat | TimescaleDB | Redis pub | 8080 | `services/predictor/Dockerfile` |
| rerouter | Go 1.22 | pgx/v5 | TimescaleDB | Redis pub | 8080 | `services/rerouter/Dockerfile` |
| backend-api-auth | Go 1.24 | Gin, GORM, gorilla/ws | TimescaleDB | Redis cache+sub | 8080 | `Backend_API_Auth/Dockerfile` |
| dashboard | JS | Leaflet, Leaflet Routing Machine | — | — | 80 | `services/dashboard/Dockerfile` |
