-- name: GetActivityByExternalID :one
-- Búsqueda idempotente por (usuario, fuente, id externo). Es la operación
-- central de la deduplicación: si existe, se actualiza; si no, se inserta.
SELECT *
FROM activities
WHERE user_id = $1
  AND external_source = $2
  AND external_id = $3
LIMIT 1;

-- name: UpsertActivity :one
-- Inserción idempotente de actividad normalizada. Devuelve la fila final
-- (insertada o actualizada) junto con xmax=0 → insert, xmax<>0 → update,
-- expuesto vía RETURNING para que el caller distinga el caso.
INSERT INTO activities (
    user_id, external_source, external_id, name, sport_type,
    started_at, elapsed_seconds, moving_seconds,
    distance_meters, elevation_gain_m,
    avg_hr, max_hr, avg_power,
    raw_payload, updated_at
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10,
    $11, $12, $13,
    $14, now()
)
ON CONFLICT (user_id, external_source, external_id) DO UPDATE SET
    name            = EXCLUDED.name,
    sport_type      = EXCLUDED.sport_type,
    started_at      = EXCLUDED.started_at,
    elapsed_seconds = EXCLUDED.elapsed_seconds,
    moving_seconds  = EXCLUDED.moving_seconds,
    distance_meters = EXCLUDED.distance_meters,
    elevation_gain_m = EXCLUDED.elevation_gain_m,
    avg_hr          = EXCLUDED.avg_hr,
    max_hr          = EXCLUDED.max_hr,
    avg_power       = EXCLUDED.avg_power,
    raw_payload     = EXCLUDED.raw_payload,
    updated_at      = now()
RETURNING *;

-- name: ListActivitiesByUser :many
-- Lista paginada de actividades del usuario, ordenadas de más reciente a
-- más antigua. El LIMIT es por la query (no cursor) porque el uso esperado
-- es UI paginada con offset; cuando se necesite cursor, se añadirá en su
-- propia query sin tocar esta.
SELECT *
FROM activities
WHERE user_id = $1
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: UpsertActivityStream :one
-- Guarda/actualiza un stream concreto de una actividad (HR, watts, ...).
-- PRIMARY KEY (activity_id, stream_type) hace el upsert natural.
INSERT INTO activity_streams (activity_id, stream_type, data)
VALUES ($1, $2, $3)
ON CONFLICT (activity_id, stream_type) DO UPDATE SET
    data = EXCLUDED.data
RETURNING *;