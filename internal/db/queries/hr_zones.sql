-- name: UpsertHRZones :one
-- Idempotente por PK activity_id. Upsert de zonas HR calculadas.
INSERT INTO hr_zones (activity_id, z1_seconds, z2_seconds, z3_seconds, z4_seconds, z5_seconds, computed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (activity_id) DO UPDATE SET
    z1_seconds  = EXCLUDED.z1_seconds,
    z2_seconds  = EXCLUDED.z2_seconds,
    z3_seconds  = EXCLUDED.z3_seconds,
    z4_seconds  = EXCLUDED.z4_seconds,
    z5_seconds  = EXCLUDED.z5_seconds,
    computed_at = EXCLUDED.computed_at
RETURNING *;

-- name: GetHRZonesByActivity :one
SELECT * FROM hr_zones WHERE activity_id = $1 LIMIT 1;

-- name: SumHRZonesInRange :one
-- Suma los segundos por zona HR sobre las actividades del usuario que
-- tienen fila en `hr_zones` y cuyo `started_at` cae dentro del rango.
-- Devuelve 0 cuando no hay filas (los handlers usan eso para distinguir
-- "sin datos HR" → fallback a avg_hr × elapsed).
--
-- PR3 DA-003 (issue #16): la fuente primaria es la tabla `hr_zones`
-- precomputada por StravaStreams worker. La ruta "activity_streams
-- heartrate JSONB" documentada en el spec se sustituye por `hr_zones`
-- (decisión de usuario 2026-10-01) por simplicidad y porque ya está
-- alineada con la arquitectura actual.
SELECT COALESCE(SUM(z1_seconds), 0)::bigint  AS z1_seconds,
       COALESCE(SUM(z2_seconds), 0)::bigint  AS z2_seconds,
       COALESCE(SUM(z3_seconds), 0)::bigint  AS z3_seconds,
       COALESCE(SUM(z4_seconds), 0)::bigint  AS z4_seconds,
       COALESCE(SUM(z5_seconds), 0)::bigint  AS z5_seconds,
       COUNT(*)::bigint                      AS activities_with_zones
FROM hr_zones hz
JOIN activities a ON a.id = hz.activity_id
WHERE a.user_id = $1
  AND a.started_at >= $2
  AND a.started_at <= $3;

-- name: SumActivitiesDurationInRange :one
-- Suma `elapsed_seconds` y cuenta actividades del usuario en un rango
-- (sin filtrar por `hr_zones`). DA-003 fallback: cuando no hay zonas
-- precomputadas, repartimos `avg_hr × elapsed` por zonas usando
-- `SumActivitiesDurationInRange` como denominador.
--
-- Mantiene `activities_with_avg_hr` separado para no contar filas
-- con `avg_hr = NULL` (que aportan 0 al cálculo y son ruido).
SELECT COUNT(*)::bigint                                       AS activities_total,
       COALESCE(SUM(elapsed_seconds), 0)::bigint             AS elapsed_seconds_total,
       COALESCE(SUM(elapsed_seconds) FILTER (WHERE avg_hr IS NOT NULL), 0)::bigint
                                                                              AS elapsed_seconds_with_avg_hr,
       COALESCE(COUNT(*) FILTER (WHERE avg_hr IS NOT NULL), 0)::bigint
                                                                              AS activities_with_avg_hr
FROM activities
WHERE user_id = $1
  AND started_at >= $2
  AND started_at <= $3;