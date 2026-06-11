-- +goose Up
-- +goose StatementBegin

-- Reemplaza el índice normal en users(email) por un índice único.
-- Garantiza que no pueda existir más de un usuario con el mismo email,
-- incluso si un webhook de Clerk se reintenta.
DROP INDEX IF EXISTS idx_users_email;
CREATE UNIQUE INDEX idx_users_email ON users (email);

-- Índice parcial único en invites(email) para invitaciones pendientes.
-- Evita invitar al mismo email dos veces mientras la invitación esté activa.
CREATE UNIQUE INDEX idx_invites_email_pending ON invites (email) WHERE status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_invites_email_pending;

-- Restaura el índice normal (sin UNIQUE) para no romper datos en rollback.
DROP INDEX IF EXISTS idx_users_email;
CREATE INDEX idx_users_email ON users (email);

-- +goose StatementEnd
