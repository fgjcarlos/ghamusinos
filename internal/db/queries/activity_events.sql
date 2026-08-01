-- name: EnqueueActivityEvent :one
-- Encola un evento de Strava (webhook). Devuelve la fila; si external_id
-- ya existe, devuelve la fila previa sin error (idempotencia). Los callers
-- pueden distinguir "nuevo" vs "ya visto" comparando received_at, o vía
-- INSERT ... ON CONFLICT DO NOTHING RETURNING (cuando lo necesitemos).
INSERT INTO activity_events (
    external_id, user_id, object_type, aspect_type, object_id, raw_payload
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (external_id) DO UPDATE SET
    external_id = EXCLUDED.external_id
RETURNING *;

-- name: MarkActivityEventProcessed :exec
-- Marca un evento como procesado una vez consumido por el job.
UPDATE activity_events
SET processed_at = now()
WHERE id = $1;

-- name: ListPendingActivityEvents :many
-- Lista los eventos pendientes (processed_at IS NULL) para alimentar un
-- job de procesamiento. LIMIT defensivo para evitar scans descontrolados.
SELECT *
FROM activity_events
WHERE processed_at IS NULL
ORDER BY received_at
LIMIT $1;

-- name: GetActivityEventByExternalID :one
-- Obtiene un evento de actividad por su external_id (usado por IngestActivityEventWorker).
SELECT *
FROM activity_events
WHERE external_id = $1
LIMIT 1;