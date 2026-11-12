-- name: CreatePlannedSession :one
-- Crea una sesion planificada para el usuario. El `user_id` viene del
-- token, nunca del cuerpo de la peticion.
INSERT INTO planned_sessions (
    user_id, scheduled_on, title, notes, workout_type,
    target_duration_sec, target_distance_m, target_elevation_gain_m, target_intensity
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPlannedSession :one
-- Devuelve una sesion del usuario. El filtro por user_id es la frontera
-- de aislamiento: la sesion de otro usuario no existe para esta consulta
-- (pgx.ErrNoRows).
SELECT *
FROM planned_sessions
WHERE id = $1
  AND user_id = $2
LIMIT 1;

-- name: ListPlannedSessionsByRange :many
-- Lista las sesiones del usuario en una ventana de fechas (inclusiva).
--
-- COUNT(*) OVER() da el total real de filas que cumplen el WHERE (no el
-- LIMIT), igual que en ListActivitiesByUser (issue #172, M6).
--
-- Los parametros de fecha van nombrados: con $2/$3 posicionales sobre dos
-- columnas DATE, SQLC no puede inferir los tipos y la generacion falla.
SELECT *, COUNT(*) OVER() AS total_count
FROM planned_sessions
WHERE user_id = sqlc.arg('user_id')
  AND scheduled_on >= sqlc.arg('from_date')
  AND scheduled_on <= sqlc.arg('to_date')
ORDER BY scheduled_on ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: UpdatePlannedSession :one
-- Actualiza una sesion del usuario. Solo el dueno de la fila (user_id)
-- puede modificarla; si no existe o es de otro usuario, pgx.ErrNoRows.
UPDATE planned_sessions
SET scheduled_on = $3,
    title = $4,
    notes = $5,
    workout_type = $6,
    target_duration_sec = $7,
    target_distance_m = $8,
    target_elevation_gain_m = $9,
    target_intensity = $10,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING *;

-- name: DeletePlannedSession :execrows
-- Borra una sesion del usuario. Devuelve el numero de filas afectadas: 0
-- significa que no existia o que era de otro usuario (misma respuesta).
DELETE FROM planned_sessions
WHERE id = $1
  AND user_id = $2;
