-- name: ListActivitiesInRange :many
-- Actividades de un usuario en un rango temporal, ordenadas ascendente.
-- Solo columnas necesarias para ComputeDailyLoad: cada actividad se
-- convierte en una entrada de TSS diario. Mantener la proyección
-- estrecha evita cargar JSONB en el worker de recálculo.
SELECT id,
       user_id,
       sport_type,
       started_at,
       elapsed_seconds,
       moving_seconds,
       distance_meters,
       elevation_gain_m,
       avg_hr,
       avg_power
FROM activities
WHERE user_id = $1
  AND started_at >= $2
  AND started_at <= $3
ORDER BY started_at ASC;

-- name: FirstActivityForUser :one
-- Fecha de la primera actividad del usuario. RecalcTrainingLoad la usa
-- cuando dashboard_metadata.last_recalc_at es NULL para acotar la
-- ventana inicial del recálculo.
SELECT MIN(started_at)::timestamptz AS first_started_at
FROM activities
WHERE user_id = $1;