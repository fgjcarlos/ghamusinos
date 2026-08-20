-- +goose Up
-- +goose StatementBegin

-- AUD-03 (issue #165), parte A: ampliar el inbox de webhooks de Strava
-- con las columnas del payload real. La idempotencia sigue siendo por
-- external_id (la columna inventada) hasta que el PR B la cambie por
-- (object_id, aspect_type, event_time). Hacerlo en dos pasos
-- (columnas + unicidad) preserva la build verde de `main` mientras
-- cada PR pasa por review.
--
-- Strava no envía external_id; el modelo previo asumía una API que no
-- existe. Las columnas nuevas (event_time, owner_id, subscription_id)
-- reflejan el payload real (object_id, aspect_type, event_time,
-- owner_id, subscription_id, updates).

ALTER TABLE activity_events
    ADD COLUMN event_time       TIMESTAMPTZ,
    ADD COLUMN owner_id         BIGINT,
    ADD COLUMN subscription_id  BIGINT;

-- Backfill defensivo: si la tabla ya tiene filas (staging, dev, etc.)
-- y event_time es NULL, las nuevas inserciones podrían colisionar
-- por (object_id, aspect_type, NULL). Rellenamos con received_at,
-- monotónico y aproximado. Si no hay filas, es no-op.
UPDATE activity_events
   SET event_time = received_at
 WHERE event_time IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE activity_events
    DROP COLUMN event_time,
    DROP COLUMN owner_id,
    DROP COLUMN subscription_id;

-- +goose StatementEnd
