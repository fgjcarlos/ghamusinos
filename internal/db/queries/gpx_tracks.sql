-- name: CreateGPXTrack :one
INSERT INTO gpx_tracks (
    user_id, name, file_hash, file_size_bytes, coordinates,
    distance_m, moving_time_s, d_plus_m, d_minus_m,
    max_elevation_m, min_elevation_m, avg_slope_pct, max_slope_pct,
    effort_index, itra_points, leg_breaker_index, estimated_vam,
    difficulty_score, difficulty_label, runnability_pct,
    king_climb, track_type, direction
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19, $20,
    $21, $22, $23
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
SELECT *
FROM gpx_tracks
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteGPXTrack :exec
DELETE FROM gpx_tracks
WHERE id = $1 AND user_id = $2;
