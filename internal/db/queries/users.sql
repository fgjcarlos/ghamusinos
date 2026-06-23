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

-- name: UpdateUserInviteStatus :one
UPDATE users
SET
    invite_status = $2,
    updated_at    = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPreferences :one
-- Actualiza las preferencias de entrenamiento e IA del usuario (fase 1.1).
-- Las métricas pueden ir a NULL si el usuario no las conoce; timezone y
-- ai_enabled siempre llevan valor (tienen default en el schema).
UPDATE users
SET
    hr_max     = $2,
    lthr       = $3,
    ftp        = $4,
    level      = $5,
    timezone   = $6,
    ai_enabled = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;
