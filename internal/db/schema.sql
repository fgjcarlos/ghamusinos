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

-- Inbox idempotente de webhooks Strava (AUD-03, issue #165).
-- Migración 00008 (PR A) añade event_time, owner_id, subscription_id
-- para reflejar el payload real de Strava:
--   (object_type, object_id, aspect_type, owner_id, event_time,
--    subscription_id[, updates]).
-- external_id y la UNIQUE (external_id) son reliquia del modelo
-- anterior (Strava no envía external_id); las borra la migración 00009
-- que llega con el PR B (handler reescrito).
--   - owner_id: athlete_id de Strava; el handler resuelve user_id
--     vía strava_tokens.athlete_id.
--   - subscription_id: identifica la suscripción activa; útil para
--     rotación de tokens/suscripciones.
CREATE TABLE activity_events (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     TEXT        NOT NULL UNIQUE,
    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,
    object_type     TEXT        NOT NULL,
    aspect_type     TEXT        NOT NULL,
    object_id       BIGINT      NOT NULL,
    owner_id        BIGINT,
    subscription_id BIGINT,
    event_time      TIMESTAMPTZ,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ,
    raw_payload     JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_activity_events_pending ON activity_events (received_at) WHERE processed_at IS NULL;
CREATE INDEX idx_activity_events_owner_id ON activity_events (owner_id) WHERE owner_id IS NOT NULL;

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
-- Calculadas a partir de streams HR usando el método Friel/Coggan con
-- los umbrales efectivos en el código: 60/70/80/90% del hrMax del usuario
-- (Z1 <60%, Z2 [60%,70%), Z3 [70%,80%), Z4 [80%,90%), Z5 >=90%).
-- El comentario original decía 50/60/70/80/90; AUD-05 (issue #166) corrige
-- el comentario para que el contrato documente lo que el código hace.
CREATE TABLE hr_zones (
    activity_id     UUID        PRIMARY KEY REFERENCES activities(id) ON DELETE CASCADE,
    z1_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z1_seconds >= 0),
    z2_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z2_seconds >= 0),
    z3_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z3_seconds >= 0),
    z4_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z4_seconds >= 0),
    z5_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z5_seconds >= 0),
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fase 1.3 — Laboratorio GPX. Mirrors migration 00006 for sqlc generation.
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
