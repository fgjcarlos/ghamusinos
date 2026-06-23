# Slice 2: JWKS Cache + JWT Validation

**Branch**: `feat/phase-1.1-auth-invites-jwks-jwt`

## Summary
Clerk JWKS caching layer and JWT validation using lestrrat-go/jwx v2; offline-first with configurable TTL.

## Changes
- `internal/auth/jwks.go`: JWKSCache type with in-memory storage, TTL (default 1h), and thread-safe reads
- `internal/auth/jwt.go`: JWTValidator checking token expiry, signature, audience claim, and required sub claim
- Tests validate cache refresh, TTL expiry, missing KID errors, invalid signatures

## Dependencies
- `github.com/lestrrat-go/jwx/v2` (v2.1.6+) for JOSE/JWE/JWS operations
- No per-request Clerk API calls; JWKS fetched once per TTL period

## Tests
- 12 tests covering cache behavior, token validation edge cases (expired, wrong aud, missing sub)

## Notes
- Cache uses sync.RWMutex for thread-safe reads; write lock only on refresh
- TTL is Unix epoch comparison; refresh happens on first access after expiry
- Audience validation optional if ClerkAudience empty in config
- Uses standard `jwx.ParseSerialized` for token parsing

---

