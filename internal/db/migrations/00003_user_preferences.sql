-- +goose Up
-- +goose StatementBegin

-- Preferencias iniciales del usuario (fase 1.1; PRD §8 "Fase 1.1" y
-- feature-inventory §3 "Actualización de perfil").
-- Las métricas fisiológicas son opcionales (el usuario puede no conocerlas) y
-- se consumen a partir de la fase 1.4 (TSS/CTL/ATL dependen de hr_max/lthr/ftp);
-- ai_enabled gobierna la IA, usada desde la fase 1.5.
ALTER TABLE users
    ADD COLUMN hr_max     SMALLINT,
    ADD COLUMN lthr       SMALLINT,
    ADD COLUMN ftp        SMALLINT,
    ADD COLUMN level      TEXT,
    ADD COLUMN timezone   TEXT    NOT NULL DEFAULT 'UTC',
    ADD COLUMN ai_enabled BOOLEAN NOT NULL DEFAULT true;

-- Rangos sanos: métricas positivas y dentro de límites fisiológicos; level
-- restringido al conjunto conocido. Permiten NULL (preferencia sin definir).
ALTER TABLE users
    ADD CONSTRAINT users_hr_max_valido CHECK (hr_max IS NULL OR (hr_max > 0 AND hr_max <= 260)),
    ADD CONSTRAINT users_lthr_valido   CHECK (lthr   IS NULL OR (lthr   > 0 AND lthr   <= 260)),
    ADD CONSTRAINT users_ftp_valido    CHECK (ftp    IS NULL OR (ftp    > 0 AND ftp    <= 2000)),
    ADD CONSTRAINT users_level_valido  CHECK (level  IS NULL OR level IN ('beginner', 'intermediate', 'advanced')),
    ADD CONSTRAINT users_timezone_valido CHECK (timezone <> '');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_timezone_valido,
    DROP CONSTRAINT IF EXISTS users_level_valido,
    DROP CONSTRAINT IF EXISTS users_ftp_valido,
    DROP CONSTRAINT IF EXISTS users_lthr_valido,
    DROP CONSTRAINT IF EXISTS users_hr_max_valido;

ALTER TABLE users
    DROP COLUMN IF EXISTS ai_enabled,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS level,
    DROP COLUMN IF EXISTS ftp,
    DROP COLUMN IF EXISTS lthr,
    DROP COLUMN IF EXISTS hr_max;

-- +goose StatementEnd
