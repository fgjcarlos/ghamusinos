# Slice 4b: Router Wiring + Clerk Config

**Branch**: `feat/phase-1.1-auth-invites-router-config`

## Summary
Configuration management for Clerk integration and middleware wiring into HTTP router.

## Changes
- `internal/config/config.go`: Add ClerkJWKSURL (required) and ClerkAudience (optional)
  - Validation: CLERK_JWKS_URL is mandatory (fails if empty)
- `internal/config/config_test.go`: 4 new tests for Clerk config validation and optional audience
- `internal/http/router.go`:
  - Update Server struct: add queries (sqlc.Querier) and cfg (*config.Config)
  - NewServer signature: NewServer(pool, queries, cfg)
  - /api route group wired with auth middleware chain:
    - AuthMiddleware validates tokens
    - ResolveMiddleware converts to User context
    - InviteGateMiddleware enforces invite requirement
  - GET /api/me handler mounted
- `internal/app/app.go`: Instantiate sqlc.Queries from pool; pass to NewServer
- `internal/http/router_test.go`: Updated test helper; 3 tests: /healthz public, /readyz public, /api/* requires auth

## Tests
- 17 config tests (including 4 new Clerk tests)
- 4 router tests (public routes, auth requirement)
- All 69 tests pass (auth, config, db, frontend, http, handlers, invites)

## Notes
- Router is minimal orchestration; auth logic tested elsewhere
- Clerk URL must be provided via CLERK_JWKS_URL env var (no defaults)
- Audience optional (useful for multi-tenant Clerk instances)

---

