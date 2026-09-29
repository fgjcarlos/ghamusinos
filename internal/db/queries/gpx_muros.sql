-- name: CreateGPXMuro :one
INSERT INTO gpx_muros (track_id, start_idx, end_idx, gain_m, distance_m, avg_slope_pct)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListGPXMurosByTrack :many
SELECT * FROM gpx_muros WHERE track_id = $1 ORDER BY start_idx;
