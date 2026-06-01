-- name: GetUserByClerkID :one
SELECT *
FROM users
WHERE clerk_user_id = $1
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (clerk_user_id, email, display_name, invite_status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users
SET
    display_name = $2,
    updated_at   = now()
WHERE id = $1
RETURNING *;
