-- Schema de base de datos de Ghamusinos.
-- Este fichero es usado exclusivamente por SQLC para generar código tipado.
-- Las migraciones reales se gestionan con Goose en internal/db/migrations/.

CREATE TABLE users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    clerk_user_id   TEXT        NOT NULL UNIQUE,
    email           TEXT        NOT NULL,
    display_name    TEXT,
    invite_status   TEXT        NOT NULL DEFAULT 'pending'
                                CHECK (invite_status IN ('pending', 'active', 'blocked')),
    -- Preferencias iniciales del usuario (fase 1.1). Métricas fisiológicas
    -- opcionales (consumidas desde la fase 1.4); ai_enabled gobierna la IA.
    hr_max          SMALLINT    CONSTRAINT users_hr_max_valido CHECK (hr_max IS NULL OR (hr_max > 0 AND hr_max <= 260)),
    lthr            SMALLINT    CONSTRAINT users_lthr_valido   CHECK (lthr   IS NULL OR (lthr   > 0 AND lthr   <= 260)),
    ftp             SMALLINT    CONSTRAINT users_ftp_valido    CHECK (ftp    IS NULL OR (ftp    > 0 AND ftp    <= 2000)),
    level           TEXT        CONSTRAINT users_level_valido  CHECK (level  IS NULL OR level IN ('beginner', 'intermediate', 'advanced')),
    timezone        TEXT        NOT NULL DEFAULT 'UTC'
                                CONSTRAINT users_timezone_valido CHECK (timezone <> ''),
    ai_enabled      BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON users (email);

CREATE TABLE invites (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT        NOT NULL,
    token_hash  TEXT        NOT NULL UNIQUE,
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    expires_at  TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_invites_email_pending ON invites (email) WHERE status = 'pending';

-- Fase 1.2 — Ingesta Strava (issue #14).
-- strava_tokens: un único set de credenciales OAuth por usuario, cifrado.
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

-- Modelo canónico de actividad. La UNIQUE (user, source, external_id) es
-- la base de la deduplicación.
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

-- Streams por (actividad, tipo) en JSONB.
CREATE TABLE activity_streams (
    activity_id     UUID        NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
    stream_type     TEXT        NOT NULL,
    data            JSONB       NOT NULL,
    PRIMARY KEY (activity_id, stream_type)
);

-- Inbox idempotente de webhooks Strava.
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

-- Sesión de sincronización con progreso.
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

-- Zonas de entrenamiento de frecuencia cardíaca (HR zones).
-- Calculadas a partir de streams HR usando el método Friel/Coggan
-- (5 zonas a 50/60/70/80/90% del hrMax del usuario).
CREATE TABLE hr_zones (
    activity_id     UUID        PRIMARY KEY REFERENCES activities(id) ON DELETE CASCADE,
    z1_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z1_seconds >= 0),
    z2_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z2_seconds >= 0),
    z3_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z3_seconds >= 0),
    z4_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z4_seconds >= 0),
    z5_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z5_seconds >= 0),
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);