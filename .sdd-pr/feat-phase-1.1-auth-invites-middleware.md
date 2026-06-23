# Slice 3b: Auth Middleware

**Branch**: `feat/phase-1.1-auth-invites-middleware`

## Summary
Three-layer middleware: token validation, user resolution, and invite gating with automatic promotion.

## Changes
- `internal/auth/middleware.go`:
  - AuthMiddleware(validator): extracts Bearer token, validates JWT, injects Claims
  - ResolveMiddleware(resolver): converts Claims to User, injects into context
  - InviteGateMiddleware(queries): enforces invite requirement
    - Active users pass through
    - Pending users checked for valid invite; if found, marked accepted + status updated to 'active' in DB
    - Pending without invite → 403 Forbidden
  - UpdateUserInviteStatus added to sqlc query interface
- `internal/db/queries/users.sql`: UpdateUserInviteStatus query
- Tests: 5 tests covering auth failure, valid token, user resolution, pending → active promotion, active user pass-through

## Design Decisions
- Middleware chain order: Auth → Resolve → InviteGate (each builds on previous)
- Invite acceptance is synchronous (blocks request); keeps state consistent
- Promotion happens in middleware, not in application code
- Claims type separate from User; only middleware knows both

## Tests
- Auth failure (401), valid token flow (200)
- Resolve: context injection, missing context detection
- InviteGate: pending without invite (403), promotion to active (200), active user pass (200)

---

