-- name: CreateRace :one
INSERT INTO races (user_id, name, race_date, distance_m, elevation_gain_m, priority, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetRace :one
SELECT * FROM races WHERE user_id = $1 AND id = $2;

-- name: ListRaces :many
SELECT * FROM races WHERE user_id = $1
ORDER BY race_date, id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountRaces :one
SELECT count(*) FROM races WHERE user_id = $1;

-- name: UpdateRace :one
UPDATE races SET name = $3, race_date = $4, distance_m = $5,
    elevation_gain_m = $6, priority = $7, notes = $8, updated_at = now()
WHERE user_id = $1 AND id = $2
RETURNING *;

-- name: DeleteRace :execrows
DELETE FROM races WHERE user_id = $1 AND id = $2;

-- name: AttachRaceTrack :one
WITH owned AS (
    SELECT races.id AS race_id, gpx_tracks.id AS gpx_track_id
    FROM races JOIN gpx_tracks ON gpx_tracks.user_id = races.user_id
    WHERE races.user_id = $1 AND races.id = $2 AND gpx_tracks.id = $3
), attached AS (
    INSERT INTO race_tracks (race_id, gpx_track_id)
    SELECT race_id, gpx_track_id FROM owned
    ON CONFLICT DO NOTHING
)
SELECT race_id FROM owned;

-- name: DetachRaceTrack :one
WITH owned AS (
    SELECT races.id AS race_id, gpx_tracks.id AS gpx_track_id
    FROM races JOIN gpx_tracks ON gpx_tracks.user_id = races.user_id
    WHERE races.user_id = $1 AND races.id = $2 AND gpx_tracks.id = $3
), detached AS (
    DELETE FROM race_tracks USING owned
    WHERE race_tracks.race_id = owned.race_id AND race_tracks.gpx_track_id = owned.gpx_track_id
)
SELECT race_id FROM owned;

-- name: ListRaceTracks :many
SELECT t.id, t.name FROM races AS r
JOIN race_tracks AS rt ON rt.race_id = r.id
JOIN gpx_tracks AS t ON t.id = rt.gpx_track_id AND t.user_id = r.user_id
WHERE r.user_id = $1 AND r.id = $2
ORDER BY t.name, t.id;

-- name: ListRaceSessions :many
SELECT s.id, s.title, s.scheduled_on, s.status FROM planned_sessions AS s
WHERE s.user_id = $1 AND s.race_id = $2
ORDER BY s.scheduled_on, s.id;
