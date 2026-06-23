# Slice 3a: User Resolver

**Branch**: `feat/phase-1.1-auth-invites-resolver`

## Summary
Atomic user creation from Clerk identity; handles race conditions with INSERT ON CONFLICT + re-fetch pattern.

## Changes
- `internal/auth/resolver.go`: UserResolver interface and dbUserResolver implementation
  - GetOrCreateUser(ctx, clerkUserID) → User: upserts user based on Clerk ID
  - Race-safe: catches code 23505 (UNIQUE violation) and re-fetches
  - Creates user with empty fields (email, display_name) when first seen; filled via later profile update

## Tests
- 4 tests: existing user fetch, unknown user creation, race condition handling, create error propagation

## Implementation Notes
- No email verification at this stage (deferred to profile update)
- Invite status defaults to "pending" via schema
- Race condition: if another request wins the INSERT, we catch constraint violation and fetch the winner's result
- User is returned with correct UUID regardless of who created it

---

