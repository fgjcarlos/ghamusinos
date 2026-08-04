-- name: CreateGPXClimb :one
INSERT INTO gpx_climbs (
    track_id, is_king_climb, start_idx, end_idx, gain_m, distance_m, avg_slope_pct
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListGPXClimbsByTrack :many
SELECT * FROM gpx_climbs WHERE track_id = $1 ORDER BY start_idx;
