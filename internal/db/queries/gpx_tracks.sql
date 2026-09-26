-- name: CreateGPXTrack :one
-- Issue #171, A8: d_plus_m / d_minus_m admiten NULL cuando la cobertura
-- de elevación es insuficiente (ver migración 00010). elevation_coverage
-- (0..1) se persiste junto al track para que la API pueda avisar al
-- frontend. NULL d+/d- con elevation_coverage conocido es el contrato.
INSERT INTO gpx_tracks (
    user_id, name, file_hash, file_size_bytes, coordinates,
    distance_m, moving_time_s, d_plus_m, d_minus_m, elevation_coverage,
    max_elevation_m, min_elevation_m, avg_slope_pct, max_slope_pct,
    effort_index, itra_points, leg_breaker_index, estimated_vam,
    difficulty_score, difficulty_label, runnability_pct,
    king_climb, track_type, direction
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20, $21,
    $22, $23, $24
)
RETURNING *;

-- name: GetGPXTrackByID :one
SELECT *
FROM gpx_tracks
WHERE id = $1 AND user_id = $2
LIMIT 1;

-- name: GetGPXTrackByHash :one
SELECT *
FROM gpx_tracks
WHERE user_id = $1 AND file_hash = $2
LIMIT 1;

-- name: ListGPXTracksByUser :many
-- Misma corrección que ListActivitiesByUser (issue #172, M6):
-- COUNT(*) OVER() añade el total real a cada fila para que el handler
-- pueda devolverlo en `total` sin engañarse con offset+len+1.
SELECT *, COUNT(*) OVER() AS total_count
FROM gpx_tracks
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteGPXTrack :exec
DELETE FROM gpx_tracks
WHERE id = $1 AND user_id = $2;
