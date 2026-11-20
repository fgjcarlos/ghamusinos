# Phase 2.1 — Planned training sessions (epic #301)

Objective: ship planned training sessions end to end — model, authenticated
CRUD and the «Entrenos» screen — and close the epic with the canonical
specs, the archived change and the verification evidence.
Base: `main@108adef`. Branch: `chore/phase-2-1-close-epic`.
Checks: `go test -race ./...`, `go vet ./...`, `pnpm -C web test:run`.
Delivery: one PR with five documentation work units; the commit hook counts the branch at
roughly 800 authored changed lines (renames and lock files excluded), over the advisory
~400-line budget of `openspec/config.yaml`. The units are markdown-only and coherent
(promotion, API contract, web contract, review fixes, this document), so a chained PR would
add ceremony without shrinking the review surface; the boundary is recorded here.

## Specs

S1. "Crear y gestionar sesiones de entrenamiento planificadas. Es la primera fase de V2: la V1 analiza, la V2 planifica."
S2. "Modelo `planned_sessions` (migración 00016 + SQLC) en `internal/workouts`."
S3. "CRUD `/api/v1/workouts` con errores RFC 9457 y paginación `limit`/`offset`."
S4. "Tipo de entrenamiento y objetivos de duración, distancia, desnivel e intensidad."
S5. "Pantalla «Entrenos» en la navegación del AppShell."
S6. "Asociación de la sesión a un objetivo. La FK a la carrera se añade cuando exista `races`, en la fase 2.3."
S7. "Un usuario crea, edita, lista y borra sus sesiones planificadas desde la web. Nadie ve ni modifica las sesiones de otro usuario."

## Tasks

T1 | S2 | delegated writer (slice 1 of 3, PR chain) | done | migration `00016_planned_sessions`, SQLC queries, `internal/workouts` domain and store | commit: d82080f.
T2 | S3,S4,S6 | delegated writer (slice 2 of 3) | done | authenticated CRUD `/api/v1/workouts`, RFC 9457 errors, `limit`/`offset` pagination, contract in `docs/api/workouts.md` | commit: 7da655e.
T3 | S5,S7 | delegated writer (slice 3 of 3) | done | «Entrenos» screen with list and form, AppShell navigation entry, TS API client | commit: 80b25b9.
T4 | S1-S7 | inline (parent) | done | canonical specs `planned-sessions-backend`, `planned-sessions-api` and `planned-sessions-web`; change archived at `openspec/changes/archive/2026-11-20-phase-2-1-planned-sessions-model/` with its archive report; epic closed | commit: see the branch head.

## Log

L1. Original request verbatim: "Resuelve la issue #301".

L2. Issue #301 body verbatim:

### Objetivo
Crear y gestionar sesiones de entrenamiento planificadas. Es la primera fase de V2: la V1 analiza, la V2 planifica.

### Entregables
- Modelo `planned_sessions` (migración 00016 + SQLC) en `internal/workouts`.
- CRUD `/api/v1/workouts` con errores RFC 9457 y paginación `limit`/`offset`.
- Tipo de entrenamiento y objetivos de duración, distancia, desnivel e intensidad.
- Pantalla «Entrenos» en la navegación del AppShell.
- Asociación de la sesión a un objetivo. La FK a la carrera se añade cuando exista `races`, en la fase 2.3.

### Criterio de cierre
Un usuario crea, edita, lista y borra sus sesiones planificadas desde la web. Nadie ve ni modifica las sesiones de otro usuario.

### Tareas
- [x] #298
- [x] #299
- [x] #300

### Dependencias
V1 completa (#13–#18). Recomendado antes: #306 (auth real con Clerk en el frontend).

### Referencias
- `docs/roadmap/roadmap.md` (Fase 2.1)
- `docs/product/ghamusinos-prd.md` §10–§11
- `docs/architecture/feature-inventory.md` §10
- Legacy: módulo workouts de `ghamusinos_/__`, como spec a portar a Go

L3. Evidence (2026-11-20, `main@108adef`): the three sub-tasks of the epic are merged — model `d82080f`, API `7da655e`, web `80b25b9` — and the explorer confirmed the model, the routes and the «Entrenos» screen are in the tree.

L4. Verification (2026-11-20): `go test -race ./...` green in every package, `go vet ./...` clean, `pnpm -C web test:run` 76 files / 469 tests passed. `golangci-lint` is not installed in this environment, so no lint verdict is claimed.

L5. Decisions: no runtime change. The three sub-tasks were already merged, so the remaining gap of the epic was SDD/ODD bookkeeping and this session changed documentation only. The list window stays browser-local (the API requires `from` and `to`); the divergence with the design of slice 1, which reserved the resolution of the day to the server from `users.timezone`, is recorded as W1 of the archive report and as a phase 2.2 follow-up instead of changing behaviour inside the epic.

L6. The Engram mirror of this document was not written: no memory tool is exposed in this session, so only the file under `odd/tasks/` exists.

L7. Native review (readability lens, MiniMax-M3.1-Flash) approved the candidate and closed with five advisory findings, none blocking: the leap-day rule of the one-year window was undefined, the line-budget exception did not explain its metric, the placeholder token was called the token of the current user, the title and note limits were expressed in bytes on the web and in characters on the backend, and the ownership of the calendar-day resolution was contradictory. All five were applied in the canonical specs, in the archive report and in this document; the API error message that says `characters` while counting UTF-8 bytes stays as a follow-up, because changing it is a runtime change outside this epic.
