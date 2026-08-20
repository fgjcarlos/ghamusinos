-- +goose Up
-- +goose StatementBegin
-- Habilita la extensión timescaledb. Es idempotente (IF NOT EXISTS).
-- docker-compose y CI usan timescale/timescaledb:2.27.2-pg16; la
-- extensión viene pre-instalada en la imagen pero no se activa por
-- defecto. Sin esta línea, las futuras hypertables (fase 1.4+
-- training_load_daily) fallarían con "extension timescaledb is not
-- loaded". Issue #28.
CREATE EXTENSION IF NOT EXISTS timescaledb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No eliminamos la extensión en Down porque otras migraciones futuras
-- podrían depender de ella (hypertable, políticas de retención, etc.).
-- Para reset destructivo real, usar `migrate reset --allow-destructive`
-- con limpieza manual de la extensión.
SELECT;
-- +goose StatementEnd