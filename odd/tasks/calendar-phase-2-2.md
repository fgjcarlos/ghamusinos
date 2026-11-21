# Calendar — Phase 2.2 (#302)
Implementation branch: `feat/phase-2-2-calendar`; tracker: `feat/phase-2-2-calendar-tracker`; selected delivery: `feature-branch-chain`.
Gates: `make test`, live migration/integration tests, `pnpm -C web typecheck`, `lint`, `test:run`, `build`.

## Specs

S1 — #302 (scope and close criteria):
> - Vistas semanal y mensual en `web/src/features/calendar`. Los límites de cada día van en la zona del usuario (`users.timezone`).
> - Mover sesiones: cambio de fecha y drag & drop.
> - Estado de la sesión: planificada, completada, omitida o modificada.
> - Vincular una actividad real (`activities`, importada de Strava) a una sesión planificada, con drag & drop.
> - Comparación entre carga planificada y carga realizada (PRD §10, «Comparación»).
>
> El usuario ve sus sesiones en semana y mes, las mueve, marca su estado y vincula una actividad real a una sesión.

S2 — #311 (backend state and linking):
> **Migración** (up/down) sobre `planned_sessions`:
>
> - `status` con `CHECK`: `planned`, `completed`, `skipped` o `modified`. Por defecto `planned`.
> - `activity_id`: nullable, FK a `activities` con `ON DELETE SET NULL`. `UNIQUE`: una actividad se vincula como mucho a una sesión.
>
> **API:**
>
> - `PATCH /api/v1/workouts/{id}` acepta `status`. Mover una sesión es un `PATCH` de `scheduled_on`, que ya existe por #299.
> - `PUT /api/v1/workouts/{id}/activity` con `{ "activity_id": … }` y `DELETE /api/v1/workouts/{id}/activity`.
> - Vincular una actividad pone el estado en `completed`, salvo que el usuario elija `modified`.
> - La actividad debe ser del mismo usuario. Si no, 404, sin distinguir el motivo.
>
> - [ ] La migración es reversible y pasa el smoke de la CI.
> - [ ] Rechaza estados desconocidos y una actividad ya vinculada a otra sesión (tests).
> - [ ] Test de integración: no se puede vincular una actividad de otro usuario.
> - [ ] Desvincular deja `activity_id` a null y no borra la actividad.

S3 — #312 (comparison endpoint):
> **Endpoint** `GET /api/v1/workouts/compare?from=&to=&group=day|week`. Para cada periodo devuelve:
> - **Planificado:** suma de duración, distancia y D+ objetivo de las sesiones que no están en `skipped`.
> - **Realizado:** suma de las `activities` del periodo, estén vinculadas o no.
> - **Cumplimiento:** sesiones `completed`, `skipped` y `modified` frente al total.
>
> Los periodos se agrupan en la zona horaria del usuario (`users.timezone`).
>
> Fuera de alcance: estimar TSS de sesiones planificadas. Si hace falta, se decide en el change de la fase.
>
> - [ ] Tests del cálculo con días sin plan, días sin actividad y sesiones `skipped`.
> - [ ] Test de límites de semana en una zona distinta de UTC.
> - [ ] Rango inválido → 400 con `ProblemDetail`.

S4 — #313 (calendar and activity tray):
> **Ruta `/calendario`** en `web/src/features/calendar`, con enlace en la navegación.
>
> - **Vistas** semanal y mensual con navegación anterior / siguiente / hoy. Los días se calculan en la zona horaria del usuario.
> - **Mover sesiones** con drag & drop (`PATCH scheduled_on`). Hay una alternativa por teclado o menú, porque la accesibilidad no se recorta.
> - **Marcar estado** de una sesión: completada, omitida o modificada.
> - **Vincular** una actividad real arrastrándola desde una bandeja de actividades del periodo sin vincular sobre una sesión, o eligiéndola en un selector.
> - Crear una sesión desde un día vacío reutiliza el formulario de #300.
> - Se prefiere drag & drop nativo antes que añadir una dependencia. Si hace falta una, se justifica en el PR.
>
> - [ ] Las vistas semana y mes muestran sesiones y actividades del rango.
> - [ ] Mover, marcar estado y vincular persisten tras recargar.
> - [ ] Todas las acciones son posibles sin ratón.
> - [ ] Tests web de las vistas y de las tres acciones. `typecheck`, `lint` y `test:run` en verde.

S5 — #312 (comparison panel):
> **Panel web** «plan vs. real» en el calendario, con un gráfico de barras en ECharts que reutiliza los componentes del dashboard.
>
> - [ ] El panel tiene estados vacío, cargando y error, y tests web.

## Tasks

- T1 · S1–S2 · #311 migration and API · bounded writer + independent verification · done locally · commit `00f81c7`
- T2 · S1, S3 · #312 comparison API · bounded writer · done · commit `30b3be1`
- T3 · S1, S4 · #313 interactive calendar · bounded writer, parent follow-through · done · commits `df921aa`, `8b5b063`
- T4 · S1, S5 · #312 comparison panel · parent follow-through · done · commit `ace5301`

## Log

L1 — Resuelve la issue #302

L2 — Epic and all three child issues are open and have no comments; no competing PR was found. Phase 2.1 is already on `main` at `8a90e2b`. Use Monday-based weeks. Keep status unchanged on unlink; keep `modified` when linking, otherwise set `completed`. Dates on the wire are local calendar dates (`YYYY-MM-DD`); actual activity timestamps are interpreted in the user's zone. No TSS or new drag-and-drop dependency. Keep linkage/API, comparison API, calendar and comparison panel as individually reviewable slices. Rollback #311 by reverting dependent application slices before the down migration; dropping the new columns loses calendar status and link metadata, so preserve them before rollback if already populated.

L3 — T1 scratch generation verified that the new SQL methods require compatibility stubs in three existing `sqlc.Querier` test doubles. Include only those stubs; keep dedicated atomic queries instead of widening the shared update behavior. Go 1.26.0, sqlc v1.31.1, and pnpm 10 are ready in `/tmp/ghamusinos-302-tools.Ln0bky`. No source edits or RED/GREEN yet.

L4 — T1: `00f81c7`. RED observed for missing migrations and workout/link behavior; focused Go packages and live planned-session integration are GREEN. Independent verifier confirmed cross-user 404, atomic ownership, duplicate conflict and unlink behavior. `TZ=UTC make test`, `make vet`, `git diff --check` exit 0 (the unmodified backfill test fails over DST on base under the host local zone; no code or checks changed). Actual 00017 UP/DOWN/UP passed on isolated PostgreSQL 18; 00007 Timescale was excluded from local setup and CI smoke remains pending. Risk: item 2 (ownership checks for link writes), also item 1 for loss of new metadata if rolling back after use. Native committed-range assessment: medium, `reviewDue=false` (`under_budget`); independently verified as item 2. Keep the slice boundary for delivery. This slice exceeds the advisory 400-line budget due to integration and compatibility coverage; do not combine it with later slices for review.

L5 — T2: `30b3be1`. API returns `period_start` local date (Monday for weeks), `planned`, `actual`, and `compliance`; uses moving duration, no numeric date-range cap, no TSS. `go test` before implementation failed on missing behavior; focused unit/handler tests then passed. Parent added RED for a skipped midnight in São Paulo (`Compare() = [], <nil>`): normalized civil dates as UTC carriers and filter padded database rows by local date; the focused regression passed. Live comparison integration, `TZ=UTC make test`, `make vet` and `git diff --check` exit 0. Native committed-range assessment medium/not due (`under_budget`). Risk: none (new read endpoint using existing authenticated scopes).

L6 — T3: `df921aa` activity feed and `8b5b063` UI. Writer quota ended with source and tests in place but no final report; no worker retry. Parent verified live feed/ownership, Go race suite and vet (all exit 0), then addressed test fixture/typecheck and formatter failures. Observed RED for dragging onto an occupied day and refreshed status after link; focused tests GREEN after DnD routing and controlled status fix. Full web run resurfaced two load-timing assertions; waits now synchronize with rendered date cells. `pnpm -C web typecheck`, `lint`, `test:run` (79 files / 506 tests after corrections), `format:check` and `build` passed. Native committed-range assessments medium/not due (`under_budget`) for each slice. Risk: none (new UI and narrow user-scoped read route; existing behavior only touched by additive TS types/props).

L7 — T4: `ace5301`. Writer quota refused this distinct task before any edits (no retry); implemented inline with existing dashboard chart components. RED: focused Vitest could not resolve the missing panel; GREEN: 40 focused tests passed. Final web typecheck/lint/format/build and 510 tests (80 files) passed; Go race suite, go vet, and isolated PostgreSQL integration passed. `make lint` initially rejected a non-exhaustive status switch; explicit `planned` branch plus focused race test and linter are GREEN (0 issues). Chromium smoke with mocked API passed: user timezone across midnight, month/week ranges, actual ECharts canvas, accessible table, native drag onto occupied day, activity link, keyboard status, reload. Native committed-range assessment medium/not due (`under_budget`). Risk: none. Existing Vite large-chunk warning persists; ECharts is lazy, shared by calendar and dashboard. No push, PR, issue closure or Timescale-based remote CI run performed.

L8 — The user authorized PR creation and chose a tracker branch chain. The tracker starts at the verified `main` base and remains draft until the children are integrated. Child review boundaries (original local commit → slice): `00f81c7` → session status/link; `30b3be1` → comparison API; `df921aa` → activity feed; `8b5b063` → interactive calendar; `ace5301` → comparison panel. Children replay those work units onto this tracker; no implementation content changes. Review each child against its immediate predecessor.
