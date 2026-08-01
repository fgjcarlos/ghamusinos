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
