# Slice 1: Auth Errors + Context

**Branch**: `feat/phase-1.1-auth-invites`

## Summary
Foundational auth package with error types and context helpers for carrying JWT claims and user identity through request lifecycle.

## Changes
- `internal/auth/errors.go`: 6 sentinel errors for auth failures (InvalidToken, InvalidSignature, MissingClaims, NoActiveInvite, etc.)
- `internal/auth/context.go`: Claims struct with Clerk user ID; User struct with UUID, email, display name, invite status; context helpers (WithAuthClaims, WithAuthUser, AuthClaimsFromContext, AuthUserFromContext)

## Tests
- 8 tests covering error sentinel values and context type safety

## Notes
- Errors follow Go idiom: `errors.Is(err, auth.ErrInvalidToken)` for error checking
- Claims and User types are distinct; middleware converts between them
- User type carries UUID for DB operations; Claims carries only Clerk ID

---

