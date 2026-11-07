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

-- name: GetActivityForUser :one
-- Actividad concreta de un usuario. El filtro por user_id es la frontera
-- de aislamiento: la actividad de otro usuario no existe para esta
-- consulta (pgx.ErrNoRows), así que un job nunca puede analizar la
-- actividad de otro usuario.
--
-- Proyección estrecha: sólo las columnas numéricas deterministas que
-- necesita el análisis de IA. Sin nombre, sin raw_payload y sin streams.
SELECT id,
       user_id,
       sport_type,
       started_at,
       elapsed_seconds,
       moving_seconds,
       distance_meters,
       elevation_gain_m,
       avg_hr
FROM activities
WHERE id = $1
  AND user_id = $2;

-- name: FirstActivityForUser :one
-- Fecha de la primera actividad del usuario. RecalcTrainingLoad la usa
-- cuando dashboard_metadata.last_recalc_at es NULL para acotar la
-- ventana inicial del recálculo.
SELECT MIN(started_at)::timestamptz AS first_started_at
FROM activities
WHERE user_id = $1;