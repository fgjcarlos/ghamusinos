# feat(api): RFC 9457 ProblemDetail + /api/v1 versioning (#38)

## Objetivo
Estandarizar el formato de errores de la API (RFC 9457 Problem Details for HTTP APIs) y versionar las rutas bajo `/api/v1`.

## Criterios de aceptación
- [x] Todos los errores devuelven `application/problem+json` con type, title, status, detail, instance (RFC 9457)
- [x] Las rutas v1 viven bajo `/api/v1/*` (no más `/api` raíz)
- [x] `/api/me` movido a `/api/v1/me` con formato ProblemDetail
- [x] Handlers 404/405 devuelven ProblemDetail
- [x] `instance` field populated con request_id del context (de #32)

## Cambios
- `internal/http/handlers/errors.go` (nuevo) — struct `ProblemDetail` + constructores (`Problem`, `NewUnauthorized`, `NewForbidden`, `NewNotFound`, `NewBadRequest`, `NewInternalError`) + `WriteProblem`
- `internal/http/router.go` (modificado) — prefijo `/api/v1`, handlers 404/405 con ProblemDetail
- `internal/http/handlers/me.go` (modificado) — usa `WriteProblem`
- Tests: errors_test.go, me_test.go, router_test.go, middleware_test.go

## Notas
- `internal/auth/middleware.go:jsonError` queda con formato `{"error": "..."}` porque migrar a `WriteProblem` requeriría import cycle (auth → internal/http). Diferido.
- Después de merge: en futuras PRs, cualquier handler nuevo debe usar `WriteProblem` para errores.

## Tests
- `go test ./...` — pasa (incluye 14 tests del paquete http + auth + handlers)
- `go vet ./...` — clean
- `gofmt -l .` — clean

## Issue
Cierra #38.
