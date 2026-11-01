# Apply progress: phase-1-4-performance-dashboard (archived)

> Estado final del change, reconciliado en el momento del archivado
> (2026-11-01). Este documento sustituye al progress de PR1.

## Session and status

- Change `phase-1-4-performance-dashboard` (epic #16): **aplicado y archivado**.
- Artifact store: `openspec`. Cuatro delta specs promovidos a specs canónicos
  (`openspec/specs/{metrics-go,training-load,dashboard-api,dashboard-web}/spec.md`).
- Todas las tareas de `tasks.md` quedan marcadas (54/54) tras reconciliarlas
  con las PRs realmente mergeadas.
- `delivery = auto`, `chain = stacked-to-main`, `review budget = 400 LoC/PR`
  (tres slices lo superaron; se partieron en commits revisables).

## PRs entregadas

| PR | Merge | Alcance |
|---|---|---|
| #240 | `bf1db5c` | PR1 — paquete `internal/metrics` puro (M-001..M-010) |
| #242 | `d8e920f` | PR2 — TimescaleDB `training_load_daily` + worker de recálculo (TL-001..TL-010) |
| #252 | `7455684` | PR3 — API `/api/v1/dashboard/*` + `/healthz` extendido (DA-001..DA-006) |
| #267 | `549c3e4` | Desbloqueo de CI: el toolchain de Go pasa a 1.26.10 (`govulncheck` verde) |
| #268 | `01b6e8f` | Prerrequisito web: React 19.3.1 y la CI ejecuta `pnpm test:run` |
| #269 | `5299bdb` | DA-007 — `GET /api/v1/dashboard/cardiac-drift` |
| #270 | `283faf6` | PR4 — pantalla `/dashboard` con ECharts (DW-001..DW-008) |

## Verificación

- Backend: `go build`, `go vet`, `gofmt`, `golangci-lint` (0 issues) y
  `go test ./...` verdes en la CI de cada PR, más el smoke de migraciones y
  los tests de integración.
- Web: `pnpm --dir web test:run` (54 ficheros / 276 tests), `typecheck`,
  `lint`, `format:check` y `build` verdes.
- Revisión nativa RDD (3 lentes) sobre #269 y #270: **approved** en ambos.

## Desviaciones y follow-ups

- El drift cardíaco no se inventa desde `avg_hr`: se calcula sobre los
  streams HR crudos de `activity_streams` (decisión del usuario), lo que
 obligó a añadir el endpoint DA-007 antes que el frontend.
- El rango de `cardiac-drift` es exclusivo por arriba (`to + 1 día` a las
  00:00 UTC) para incluir completo el último día; `/hr-zones` sigue con el
  límite inclusivo a medianoche.
- El chunk de ECharts (185 kB gzip) va lazy: el bundle inicial no crece.
- `internal/jobs.TestBackfillWindow_CalculatesCorrectSpan` es flaky fuera de
  UTC (bug de DST preexistente); pasa en la CI.

## Artefactos

- `proposal.md`, `design.md`, `exploration.md`, `tasks.md` (54/54 marcadas).
- `specs/{metrics-go,training-load,dashboard-api,dashboard-web}/spec.md`.
- `archive-report.md` con el cierre del epic y los riesgos abiertos.
