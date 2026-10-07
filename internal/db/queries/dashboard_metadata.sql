-- name: GetDashboardMetadata :one
-- Lee la fila de metadata del usuario. Si nunca se ejecutó un recalc,
-- la fila no existe; el handler /healthz extendido debe distinguir "sin
-- metadata" (status=`never`) de "último recalc falló" (status=`error`).
SELECT *
FROM dashboard_metadata
WHERE user_id = $1;

-- name: UpsertDashboardMetadata :one
-- Inserta o actualiza la fila completa. El campo `training_load_rows`
-- lo mantiene el caller (lo cuenta antes o lo deja en su valor previo).
INSERT INTO dashboard_metadata (
    user_id, last_recalc_at, last_recalc_status, last_recalc_error,
    training_load_rows, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE SET
    last_recalc_at     = EXCLUDED.last_recalc_at,
    last_recalc_status = EXCLUDED.last_recalc_status,
    last_recalc_error  = EXCLUDED.last_recalc_error,
    training_load_rows = EXCLUDED.training_load_rows,
    updated_at         = EXCLUDED.updated_at
RETURNING *;