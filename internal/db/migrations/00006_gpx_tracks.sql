-- +goose Up
-- +goose StatementBegin

CREATE TABLE gpx_tracks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    coordinates JSONB NOT NULL,
    distance_m NUMERIC NOT NULL,
    moving_time_s INTEGER NOT NULL,
    d_plus_m NUMERIC NOT NULL,
    d_minus_m NUMERIC NOT NULL,
    max_elevation_m NUMERIC,
    min_elevation_m NUMERIC,
    avg_slope_pct NUMERIC,
    max_slope_pct NUMERIC,
    effort_index NUMERIC,
    itra_points NUMERIC,
    leg_breaker_index NUMERIC,
    estimated_vam NUMERIC,
    difficulty_score INTEGER NOT NULL CHECK (difficulty_score BETWEEN 0 AND 100),
    difficulty_label TEXT NOT NULL CHECK (difficulty_label IN ('beginner','intermediate','advanced','pro')),
    runnability_pct NUMERIC,
    king_climb JSONB,
    track_type TEXT NOT NULL CHECK (track_type IN ('circular','point-to-point')),
    direction TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, file_hash)
);

CREATE TABLE gpx_climbs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id UUID NOT NULL REFERENCES gpx_tracks(id) ON DELETE CASCADE,
    is_king_climb BOOLEAN NOT NULL DEFAULT false,
    start_idx INTEGER NOT NULL,
    end_idx INTEGER NOT NULL,
    gain_m NUMERIC NOT NULL,
    distance_m NUMERIC NOT NULL,
    avg_slope_pct NUMERIC NOT NULL
);

CREATE TABLE gpx_risk_zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id UUID NOT NULL REFERENCES gpx_tracks(id) ON DELETE CASCADE,
    start_idx INTEGER NOT NULL,
    end_idx INTEGER NOT NULL,
    risk_type TEXT NOT NULL CHECK (risk_type IN ('steep','technical','exposure')),
    severity TEXT NOT NULL CHECK (severity IN ('medium','high'))
);

CREATE INDEX idx_gpx_tracks_user_created ON gpx_tracks (user_id, created_at DESC);
CREATE INDEX idx_gpx_tracks_difficulty ON gpx_tracks (user_id, difficulty_score DESC);
CREATE INDEX idx_gpx_climbs_track ON gpx_climbs (track_id);
CREATE INDEX idx_gpx_risk_zones_track ON gpx_risk_zones (track_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS gpx_risk_zones;
DROP TABLE IF EXISTS gpx_climbs;
DROP TABLE IF EXISTS gpx_tracks;

-- +goose StatementEnd
