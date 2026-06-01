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
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users (email);

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
