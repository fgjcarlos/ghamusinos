-- name: UpsertGPXKmVertical :one
INSERT INTO gpx_km_vertical (track_id, start_idx, end_idx, gain_m, distance_m)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (track_id) DO UPDATE SET
    start_idx = EXCLUDED.start_idx,
    end_idx = EXCLUDED.end_idx,
    gain_m = EXCLUDED.gain_m,
    distance_m = EXCLUDED.distance_m
RETURNING *;

-- name: GetGPXKmVerticalByTrack :one
SELECT * FROM gpx_km_vertical WHERE track_id = $1;
