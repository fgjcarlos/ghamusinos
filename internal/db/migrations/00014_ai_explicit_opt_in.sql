-- +goose Up
-- +goose StatementBegin

-- Phase 1.5 (issue #17, T2) — explicit AI opt-in.
--
-- users.ai_enabled used to default to TRUE, so a stored TRUE cannot
-- prove that the user ever accepted the external AI transfer notice.
-- The product requires a fresh acceptance from every user, so this
-- migration:
--
--   1. changes the column default to FALSE, so a user created from now
--      on is never opted in by default;
--   2. resets every currently stored TRUE to FALSE, so no legacy
--      default-true value survives as a consent.
--
-- The reset is irreversible on purpose: the information needed to tell
-- a real consent apart from the old default no longer exists. Users
-- opt in again explicitly (PATCH /api/v1/me/preferences with
-- ai_enabled=true) after reading the transfer notice.
ALTER TABLE users
    ALTER COLUMN ai_enabled SET DEFAULT false;

UPDATE users
SET ai_enabled = false
WHERE ai_enabled = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- The rollback is deliberately conservative: it never restores consent.
--
--   - No stored value is rewritten, so the users reset by the Up
--     section stay reset (the invalidation is irreversible).
--   - The default stays FALSE: restoring DEFAULT TRUE would opt in
--     every user created after the rollback without any action from
--     them, which is exactly the implicit consent this migration
--     removes.
ALTER TABLE users
    ALTER COLUMN ai_enabled SET DEFAULT false;

-- +goose StatementEnd
