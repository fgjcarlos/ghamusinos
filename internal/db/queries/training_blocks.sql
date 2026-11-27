-- name: CreateTrainingBlock :one
INSERT INTO training_blocks (user_id, name, start_date, end_date, race_id, notes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTrainingBlock :one
SELECT * FROM training_blocks WHERE user_id = $1 AND id = $2;

-- name: ListTrainingBlocks :many
SELECT * FROM training_blocks WHERE user_id = $1
ORDER BY start_date, end_date, id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountTrainingBlocks :one
SELECT count(*) FROM training_blocks WHERE user_id = $1;

-- name: UpdateTrainingBlock :one
UPDATE training_blocks SET name = $3, start_date = $4, end_date = $5, race_id = $6, notes = $7, updated_at = now()
WHERE user_id = $1 AND id = $2
RETURNING *;

-- name: DeleteTrainingBlock :execrows
DELETE FROM training_blocks WHERE user_id = $1 AND id = $2;

-- name: CountOverlappingTrainingBlocks :one
-- Two blocks share a day when each one starts on or before the other's end
-- and ends on or after the other's start. exclude_id is the block being
-- updated, or a zero UUID when creating.
SELECT count(*) FROM training_blocks
WHERE user_id = sqlc.arg('user_id')
    AND (sqlc.arg('exclude_id')::uuid IS NULL OR id <> sqlc.arg('exclude_id'))
    AND start_date <= sqlc.arg('end_date') AND end_date >= sqlc.arg('start_date');

-- name: LockUserTrainingBlocks :exec
-- Serializes the overlap check per user inside the caller's transaction, so two
-- concurrent writes cannot both pass it.
SELECT pg_advisory_xact_lock(hashtext($1::text));
