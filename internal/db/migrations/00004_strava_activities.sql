-- +goose Up
-- +goose StatementBegin

-- Tokens OAuth de Strava del usuario (fase 1.2; ADR 0001).
-- Se almacenan cifrados con AES-256-GCM (internal/crypto): la columna guarda
-- el sobre (nonce || ciphertext || tag) codificado en base64.
CREATE TABLE strava_tokens (
    user_id          UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    access_cipher    TEXT        NOT NULL,
    refresh_cipher   TEXT        NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    athlete_id       BIGINT      NOT NULL,
    scopes           TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_strava_tokens_expires_at ON strava_tokens (expires_at);

-- Modelo canónico de actividad (fase 1.2; feature-inventory §5).
-- external_id es el id de Strava; la unicidad por usuario+external_id es la
-- base de la deduplicación (re-ingesta idempotente).
CREATE TABLE activities (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_source     TEXT        NOT NULL DEFAULT 'strava'
                                    CHECK (external_source IN ('strava')),
    external_id         BIGINT      NOT NULL,
    name                TEXT        NOT NULL,
    sport_type          TEXT        NOT NULL,
    started_at          TIMESTAMPTZ NOT NULL,
    elapsed_seconds     INTEGER     NOT NULL CONSTRAINT activities_elapsed_no_negativo CHECK (elapsed_seconds >= 0),
    moving_seconds      INTEGER     NOT NULL CONSTRAINT activities_moving_no_negativo CHECK (moving_seconds >= 0),
    distance_meters     NUMERIC(12,2) CONSTRAINT activities_distance_no_negativo CHECK (distance_meters IS NULL OR distance_meters >= 0),
    elevation_gain_m    NUMERIC(8,2)  CONSTRAINT activities_elevation_no_negativo CHECK (elevation_gain_m IS NULL OR elevation_gain_m >= 0),
    avg_hr              SMALLINT    CONSTRAINT activities_avg_hr_valido CHECK (avg_hr IS NULL OR (avg_hr > 0 AND avg_hr <= 260)),
    max_hr              SMALLINT    CONSTRAINT activities_max_hr_valido CHECK (max_hr IS NULL OR (max_hr > 0 AND max_hr <= 260)),
    avg_power           SMALLINT    CONSTRAINT activities_avg_power_valido CHECK (avg_power IS NULL OR (avg_power > 0 AND avg_power <= 2000)),
    raw_payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, external_source, external_id)
);

CREATE INDEX idx_activities_user_started ON activities (user_id, started_at DESC);

-- Streams de Strava (HR, potencia, cadencia, altitud, latlng): los guardamos
-- como JSONB para no explosion en columnas y poder evolucionar el conjunto
-- de claves sin migraciones. Una fila por (actividad, tipo) — tipos
-- habituales en Strava: heartrate, watts, cadence, altitude, latlng, ...
CREATE TABLE activity_streams (
    activity_id     UUID        NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
    stream_type     TEXT        NOT NULL,
    data            JSONB       NOT NULL,
    PRIMARY KEY (activity_id, stream_type)
);

-- Inbox idempotente de webhooks Strava (fase 1.2; feature-inventory §5).
-- external_id es el id que Strava asigna al evento; UNIQUE evita reproceso.
-- Procesados = NULL significa "pendiente de procesar".
CREATE TABLE activity_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     TEXT        NOT NULL UNIQUE,
    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,
    object_type     TEXT        NOT NULL,
    aspect_type     TEXT        NOT NULL,
    object_id       BIGINT      NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ,
    raw_payload     JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_activity_events_pending ON activity_events (received_at) WHERE processed_at IS NULL;

-- Sesión de sincronización con progreso (fase 1.2; feature-inventory §5).
-- status: pending → running → completed | failed | cancelled.
CREATE TABLE sync_sessions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    window_days     INTEGER     NOT NULL CONSTRAINT sync_sessions_window_valido CHECK (window_days > 0 AND window_days <= 365),
    total_activities INTEGER    NOT NULL DEFAULT 0,
    imported        INTEGER     NOT NULL DEFAULT 0,
    skipped         INTEGER     NOT NULL DEFAULT 0,
    error           TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ
);

CREATE INDEX idx_sync_sessions_user_started ON sync_sessions (user_id, started_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS sync_sessions;
DROP TABLE IF EXISTS activity_events;
DROP TABLE IF EXISTS activity_streams;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS strava_tokens;

-- +goose StatementEnd