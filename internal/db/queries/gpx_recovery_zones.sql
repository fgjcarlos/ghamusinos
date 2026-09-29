-- name: CreateGPXRecoveryZone :one
INSERT INTO gpx_recovery_zones (track_id, start_idx, end_idx, distance_m)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListGPXRecoveryZonesByTrack :many
SELECT * FROM gpx_recovery_zones WHERE track_id = $1 ORDER BY start_idx;
