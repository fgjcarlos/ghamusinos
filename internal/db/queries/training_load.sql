-- name: UpsertTrainingLoadDaily :one
-- Idempotente. Inserta o actualiza la fila diaria de training load para
-- (user_id, day). El caller calcula CTL/ATL/TSB; aquí solo persiste TSS +
-- KPI crudos.
INSERT INTO training_load_daily (
    user_id, day, tss, activity_count, distance_m, elevation_gain_m, computed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, day) DO UPDATE SET
    tss              = EXCLUDED.tss,
    activity_count   = EXCLUDED.activity_count,
    distance_m       = EXCLUDED.distance_m,
    elevation_gain_m = EXCLUDED.elevation_gain_m,
    computed_at      = EXCLUDED.computed_at
RETURNING *;

-- name: ListTrainingLoadRange :many
-- Serie diaria en orden cronológico ascendente. Útil para CTL/ATL/TSB
-- dashboards: si el rango es anterior a la primera actividad, devuelve 0
-- filas (el handler rellena con FillMissingDays, issue #16).
SELECT *
FROM training_load_daily
WHERE user_id = $1
  AND day BETWEEN $2 AND $3
ORDER BY day ASC;

-- name: ListTrainingLoadFromFirstActivity :many
-- Serie completa desde la primera actividad hasta hoy. Usada por el job
-- RecalcTrainingLoad cuando no hay `last_recalc_at` registrado.
SELECT *
FROM training_load_daily tld
WHERE tld.user_id = $1
  AND tld.day >= COALESCE(
        (SELECT MIN(tld2.day) FROM training_load_daily tld2 WHERE tld2.user_id = $1),
        '0001-01-01'::date
    )
ORDER BY tld.day ASC;

-- name: ListUserIDsForTrainingLoadRecalc :many
-- Para un job fan-out (futuro): lista todos los user_id con actividades
-- que requieren recálculo. PR2 no la usa; queda cableada para PR3 o un
-- job cron. Mantenerla aquí evita un nuevo PR cuando se encienda.
SELECT DISTINCT activities.user_id
FROM activities;