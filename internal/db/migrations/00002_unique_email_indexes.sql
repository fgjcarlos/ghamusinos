-- +goose Up
-- +goose StatementBegin

-- Guardia: si ya existen emails duplicados (p. ej. por webhooks reintentados
-- antes de esta migración), aborta con un mensaje accionable en lugar del
-- error críptico del CREATE UNIQUE INDEX. Deduplicar manualmente y reintentar.
DO $$
DECLARE
    duplicados integer;
BEGIN
    SELECT count(*) INTO duplicados FROM (
        SELECT email FROM users GROUP BY email HAVING count(*) > 1
    ) d;
    IF duplicados > 0 THEN
        RAISE EXCEPTION 'users tiene % emails duplicados; deduplica antes de migrar (SELECT email, count(*) FROM users GROUP BY email HAVING count(*) > 1)', duplicados;
    END IF;
END $$;

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
