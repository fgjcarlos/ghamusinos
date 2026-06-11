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
