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
ORDER BY scheduled_on ASC, id ASC
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
    status = $11,
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

-- name: LinkPlannedSessionActivity :one
-- Vincula una actividad real a una sesion, comprobando en la misma
-- sentencia que la sesion Y la actividad son del usuario (user_id = $2
-- en ambas): una ajena o inexistente no produce filas (pgx.ErrNoRows,
-- 404 sin distinguir el motivo). El UNIQUE(activity_id) de la migracion
-- 00017 rechaza una actividad ya enlazada a otra sesion (23505, 409).
-- Vincular completa la sesion, salvo que ya este marcada modified.
UPDATE planned_sessions AS s
SET activity_id = $3,
    status = CASE WHEN s.status = 'modified' THEN 'modified' ELSE 'completed' END,
    updated_at = now()
FROM activities AS a
WHERE s.id = $1
  AND s.user_id = $2
  AND a.id = $3
  AND a.user_id = $2
RETURNING s.*;

-- name: UnlinkPlannedSessionActivity :one
-- Desvincula la actividad de una sesion del usuario: activity_id a NULL
-- sin tocar el estado. 0 filas (pgx.ErrNoRows) si la sesion no existe o
-- es de otro usuario. La actividad nunca se borra aqui.
UPDATE planned_sessions
SET activity_id = NULL,
    updated_at = now()
WHERE id = $1
  AND user_id = $2
RETURNING *;

-- name: ListCalendarActivities :many
-- Bandeja de actividades del calendario (#313, T3): actividades del
-- usuario en una ventana temporal con la marca de vinculadas. La
-- proyeccion es estrecha (id, nombre, instante, linked) y la ventana
-- llega acolchada en UTC desde Go (-1/+2 dias), que recorta despues por
-- fecha civil exacta de la zona del usuario (mismo patron que Compare,
-- #312). El EXISTS cualifica a los DOS duenos (sesion y actividad del
-- mismo usuario): una sesion ajena nunca marca linked, y la marca es
-- global, no del rango pedido.
SELECT a.id,
       a.name,
       a.started_at,
       EXISTS (
           SELECT 1
           FROM planned_sessions AS s
           WHERE s.activity_id = a.id
             AND s.user_id = a.user_id
       ) AS linked
FROM activities AS a
WHERE a.user_id = $1
  AND a.started_at >= $2
  AND a.started_at <= $3
ORDER BY a.started_at ASC;
