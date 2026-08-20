-- name: EnqueueActivityEvent :one
-- Encola un evento de Strava (webhook). Devuelve la fila; si external_id
-- ya existe, devuelve la fila previa sin error (idempotencia). Los callers
-- pueden distinguir "nuevo" vs "ya visto" comparando received_at, o vía
-- INSERT ... ON CONFLICT DO NOTHING RETURNING (cuando lo necesitemos).
--
-- AUD-03 (issue #165), parte A: la UNIQUE todavía es por external_id; la
-- migración a (object_id, aspect_type, event_time) llega con el PR B. Aquí
-- solo añadimos las columnas del payload real y las dos queries nuevas
-- (GetActivityEventByID, GetUserIDByAthleteID) que el handler y el worker
-- usarán cuando el modelo completo esté en pie.
INSERT INTO activity_events (
    external_id, user_id, object_type, aspect_type, object_id,
    owner_id, subscription_id, event_time, raw_payload
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (external_id) DO UPDATE SET
    external_id = EXCLUDED.external_id
RETURNING *;

-- name: GetActivityEventByID :one
-- Recupera un evento por su UUID interno. Lo usará
-- IngestActivityEventWorker.Work en el PR C: el handler encola por
-- POST y el worker carga por el ID que se pasó en IngestActivityEventArgs.
SELECT * FROM activity_events WHERE id = $1;

-- name: GetUserIDByAthleteID :one
-- Resuelve el user_id interno desde el athlete_id (BIGINT) de Strava.
-- Si el atleta no ha vinculado Strava con nuestra app, sqlc genera
-- un método que devuelve pgtype.UUID sin Valid=true (el handler lo
-- trata como "no conozco a este atleta" y responde 200 sin encolar).
SELECT user_id FROM strava_tokens WHERE athlete_id = $1 LIMIT 1;

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
