# Archive Report: phase-1-6-gpx-advanced-lab

> **Status: archived on 2026-11-07.** The GB-001..GB-006 requirements were
> appended to `openspec/specs/gpx-backend/spec.md` and the GL-001..GL-008
> requirements to `openspec/specs/gpx-lab/spec.md`; this change now lives at
> `openspec/changes/archive/2026-11-07-phase-1-6-gpx-advanced-lab/`.

## Delivered

Phase 1.6 closes GitHub epic #18 ("Laboratorio GPX avanzado") with the eight
deliverables of the issue plus the backend needed by Post-Activity.

| Slice | Unit | Commits |
| --- | --- | --- |
| 1 | Strava `time` stream + activity streams read/refresh + enqueue fix | `c7144f8` |
| 2 | Track weather proxy | `1fad7d7` |
| 3 | Track terrain proxy + route wiring | `572833a`, `6bff217` |
| 4 | Pure web model: geometry, splits/cutoffs, race day, solar, gear | `ce4b2bb`, `825ee6d`, `9cafe97`, `21b0aa9`, `6e7c06d` |
| 5 | API client, hooks, race-planning UI | `93e07dd`, `2dd47a0` |
| 6 | Post-Activity model/panel and documentation | `aa41061`, docs |

Local diff against `main@2725d2e`: 63 files, +7.113 / −4 lines.

## Spec promotion

| Delta | Canonical spec | Requirements |
| --- | --- | --- |
| `specs/gpx-backend/spec.md` | `openspec/specs/gpx-backend/spec.md` | GB-001..GB-006 |
| `specs/gpx-lab/spec.md` | `openspec/specs/gpx-lab/spec.md` | GL-001..GL-008 |

The canonical files already existed from phase 1.3; the promotion appends the
new requirements and records the phase 1.6 provenance in each header. It is
an append, not an overwrite.

## Verification

- `TZ=UTC GOTOOLCHAIN=go1.26.10 go test -race -count=1 ./...` → green.
- `go vet ./...` → clean.
- `golangci-lint run ./...` → `0 issues`.
- `pnpm -C web test:run` → 68 files / 435 tests passed.
- `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check` →
  clean.
- No live third-party request in any test: Go uses `httptest`/fakes and the
  web client stubs `fetch`.

## Known warnings and limitations

- **W1 — Existing activities lack `time`.** The per-activity refresh action
  fixes them one by one; a bulk backfill is a follow-up.
- **W2 — Weather horizon.** Open-Meteo only covers its forecast window; an
  all-out-of-range instant returns `422` and the panel degrades.
- **W3 — OSM completeness.** `unknown` and `available:false` are valid
  outcomes, not errors.
- **W4 — Solar shade heuristic.** `shade` means the segment faces away from
  the sun; there is no cast-shadow terrain model.
- **W5 — Cutoff required pace.** Arrival is elevation-aware, but the required
  pace is the flat `timeLimit / km` estimate.
- **W6 — Per-process caches.** Multiple replicas each call the upstream once;
  there is no cross-replica coordination.
- **W7 — Post-Activity picker.** The Strava external id is typed by the user;
  a recent-activity picker is a deliberate follow-up.
- **W8 — Runtime review.** The native review is unavailable in this OpenCode
  runtime (`unassessable`); no PASS was invented. Verification is the
  observed test/vet/lint evidence above.

## Follow-ups

1. Bulk re-import of `time` for activities created before phase 1.6.
2. Recent-activity picker for Post-Activity.
3. Server-side cache across replicas if the deployment scales horizontally.
4. Live-doc verification of the Open-Meteo/Overpass request shapes against
   their current official documentation.
