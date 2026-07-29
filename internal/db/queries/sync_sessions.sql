-- name: CreateSyncSession :one
-- Crea una nueva sesión de sincronización en estado 'pending'. El caller la
-- transita a 'running' con UpdateSyncSessionStatus cuando empieza a procesar.
INSERT INTO sync_sessions (user_id, status, window_days)
VALUES ($1, 'pending', $2)
RETURNING *;

-- name: UpdateSyncSessionStatus :one
-- Cambia el estado de la sesión. Si falla el job, se guarda el error.
UPDATE sync_sessions
SET
    status      = $2,
    error       = $3,
    finished_at = CASE WHEN $2 IN ('completed', 'failed', 'cancelled') THEN now() ELSE finished_at END
WHERE id = $1
RETURNING *;

-- name: UpdateSyncSessionProgress :one
-- Actualiza los contadores de progreso sin tocar status/finished_at.
UPDATE sync_sessions
SET
    total_activities = $2,
    imported         = $3,
    skipped          = $4
WHERE id = $1
RETURNING *;

-- name: ListSyncSessionsByUser :many
-- Lista las sesiones de sincronización del usuario, más recientes primero.
SELECT *
FROM sync_sessions
WHERE user_id = $1
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;