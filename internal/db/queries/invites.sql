-- name: GetInviteByTokenHash :one
SELECT *
FROM invites
WHERE token_hash = $1
LIMIT 1;

-- name: CreateInvite :one
INSERT INTO invites (email, token_hash, status, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: MarkInviteAccepted :exec
UPDATE invites
SET
    status      = 'accepted',
    accepted_at = now()
WHERE id = $1;
