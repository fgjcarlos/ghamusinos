-- name: ListHeartRateStreamsInRange :many
-- Streams de frecuencia cardíaca (`activity_streams.data` JSONB) de las
-- actividades del usuario dentro del rango, para el endpoint de cardiac
-- drift (DA-007, issue #16).
--
-- El rango cubre días completos: `$2` es el inicio (00:00 UTC del día
-- `from`) y `$3` es el límite EXCLUSIVO (00:00 UTC del día siguiente a
-- `to`), de forma que la actividad del último día del rango entra
-- completa (misma semántica inclusiva que `day BETWEEN` de DA-002).
--
-- Solo se devuelven los streams `heartrate`; el handler parsea el JSONB
-- (array plano de números o pares `[time, value]` de la API de Strava)
-- y calcula el drift con `internal/metrics.CardiacDriftSeries`.
SELECT s.activity_id,
       s.data
FROM activity_streams s
JOIN activities a ON a.id = s.activity_id
WHERE a.user_id = $1
  AND s.stream_type = 'heartrate'
  AND a.started_at >= $2
  AND a.started_at < $3
ORDER BY a.started_at ASC, s.activity_id ASC;
