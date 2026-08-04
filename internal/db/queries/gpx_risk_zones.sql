-- name: CreateGPXRiskZone :one
INSERT INTO gpx_risk_zones (
    track_id, start_idx, end_idx, risk_type, severity
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGPXRiskZonesByTrack :many
SELECT * FROM gpx_risk_zones WHERE track_id = $1 ORDER BY start_idx;
