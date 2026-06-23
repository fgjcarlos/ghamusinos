# Slice 4c: CLI Invite Creation

**Branch**: `feat/phase-1.1-auth-invites-cli`

## Summary
Admin CLI for creating invite tokens: generates cryptographically secure token, stores hash in DB, prints raw token for sharing.

## Changes
- `cmd/invites/main.go`: CLI entry point with 'create' subcommand
  - Flags: --email (required), --expires-in (default "7d"), --token-length (default 32)
  - Reads DATABASE_URL and CLERK_JWKS_URL for config
  - Prints raw token to stdout (only once)
- `cmd/invites/create.go`: Core logic
  - generateTokenAndHash(length): cryptographic token + SHA-256 hash
  - parseDuration(s): parses "7d", "24h", etc. (converts days to hours)
  - createInvite: stores invite with pending status, configurable expiry
- `cmd/invites/create_test.go`: 8 comprehensive tests
  - Token generation: validity, uniqueness, length respect
  - Hash determinism and SHA-256 correctness
  - Duration parsing: days, hours, invalid formats
  - Benchmark: token generation performance

## Usage
```
DATABASE_URL=postgres://... CLERK_JWKS_URL=https://... \
  go run ./cmd/invites create --email user@example.com [--expires-in 7d] [--token-length 32]
```

## Design Decisions
- Token: cryptographically random bytes (via crypto/rand)
- Storage: only SHA-256 hash stored (token verified via hash-compare on invitation flow)
- Expiry: stored as timestamptz in DB; checked by GetActiveInviteByEmail
- Status: always "pending" on creation
- Output: raw token printed to stdout once for admin to copy and share

## Tests
- 8 unit tests: token generation, hashing, duration parsing, edge cases
- All 69 tests passing (with 8 new CLI tests)

---

