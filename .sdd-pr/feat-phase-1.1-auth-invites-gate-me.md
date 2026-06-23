# Slice 4a: Me Handler + UpdateUserInviteStatus

**Branch**: `feat/phase-1.1-auth-invites-gate-me`

## Summary
First protected endpoint (`GET /api/me`) returning authenticated user; UpdateUserInviteStatus query for invite promotion.

## Changes
- `internal/http/handlers/me.go`: Me handler returns JSON user profile (id, clerk_user_id, email, display_name, invite_status)
- `internal/http/handlers/me_test.go`: 2 tests (valid user, missing context)
- `internal/db/queries/users.sql`: UpdateUserInviteStatus(id, invite_status) → User
- sqlc regenerated with new query method

## Tests
- 2 tests: GET /api/me with valid user returns 200+JSON, missing auth context returns 401

## Integration Notes
- Handler requires user in context (injected by ResolveMiddleware)
- Returns user object as-is from context (no DB fetch)
- Query method used by InviteGateMiddleware to promote pending → active

---

