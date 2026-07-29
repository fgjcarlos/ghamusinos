-- name: GetStravaTokensByUserID :one
-- Recupera los tokens OAuth de Strava de un usuario (cifrados).
SELECT *
FROM strava_tokens
WHERE user_id = $1
LIMIT 1;

-- name: UpsertStravaTokens :one
-- Inserta o reemplaza los tokens OAuth cifrados de un usuario.
-- Strava solo permite un set de tokens activo por usuario; ON CONFLICT cubre
-- el caso "usuario reconecta" y el refresh que rota access_token.
INSERT INTO strava_tokens (
    user_id, access_cipher, refresh_cipher, expires_at, athlete_id, scopes, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (user_id) DO UPDATE SET
    access_cipher  = EXCLUDED.access_cipher,
    refresh_cipher = EXCLUDED.refresh_cipher,
    expires_at     = EXCLUDED.expires_at,
    athlete_id     = EXCLUDED.athlete_id,
    scopes         = EXCLUDED.scopes,
    updated_at     = now()
RETURNING *;

-- name: DeleteStravaTokensByUserID :exec
-- Limpia los tokens del usuario (logout / desvinculación).
DELETE FROM strava_tokens
WHERE user_id = $1;