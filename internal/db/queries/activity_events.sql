-- name: EnqueueActivityEvent :one
-- Return the existing row when Strava retries the same natural event key.
INSERT INTO activity_events (
    user_id, object_type, aspect_type, object_id,
    owner_id, subscription_id, event_time, raw_payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (object_id, aspect_type, event_time) DO UPDATE SET
    object_id = EXCLUDED.object_id
RETURNING *;

-- name: GetActivityEventByID :one
-- Load an event by the internal UUID passed to IngestActivityEventWorker.
SELECT * FROM activity_events WHERE id = $1;

-- name: GetUserIDByAthleteID :one
-- The scalar subquery always returns one row. An unknown athlete therefore
-- scans as pgtype.UUID{Valid:false} rather than pgx.ErrNoRows.
SELECT (
    SELECT user_id FROM strava_tokens WHERE athlete_id = $1 LIMIT 1
)::uuid AS user_id;

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
