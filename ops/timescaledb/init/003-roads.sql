CREATE TABLE IF NOT EXISTS roads (
    road_id    TEXT PRIMARY KEY,
    label      TEXT NOT NULL DEFAULT '',
    lat        DOUBLE PRECISION,
    lng        DOUBLE PRECISION,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
