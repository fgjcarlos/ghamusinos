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

-- name: GetActiveInviteByEmail :one
-- Devuelve la invitación vigente para un email dado.
-- "Vigente" significa: status pending o accepted, y no expirada
-- (expires_at es NULL o está en el futuro).
-- Usado en el flujo de bloqueo por invitación (v1-phase-1-plan.md §4.7).
-- Puede haber varias vigentes a la vez (una accepted histórica + una pending
-- reenviada): se devuelve la más reciente de forma determinista.
SELECT id, email, status, expires_at, accepted_at, created_at
FROM invites
WHERE email     = $1
  AND status    IN ('pending', 'accepted')
  AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC
LIMIT 1;
