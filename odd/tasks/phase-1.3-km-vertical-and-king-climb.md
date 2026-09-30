# Phase 1.3 — Km Vertical, Muros, Recovery Zones (#15)

**Epic**: #15 (Fase 1.3 — Laboratorio GPX base).
**Change OpenSpec**: `openspec/changes/phase-1-3-km-vertical-and-king-climb/` (commit `8928a2b`).
**Branch**: `feat/phase-1.3-km-vertical`.
**Strategy**: 2-PR chain, user-confirmed in proposal.

## Contexto y decisiones explícitas

- **El slice cierra la única grieta funcional de #15**: `FindKmVertical`, `FindMuros`, `FindRecoveryZones` se calculan al upload pero **nunca se persisten** (`internal/http/handlers/gpx_upload.go:142-146` los devuelve en claves top-level huérfanas y `CreateDetail` no los recibe). Tras refrescar la página, los valores desaparecen aunque los detectores siguen emitiendo lo correcto.
- **3 tablas nuevas** (`gpx_muros`, `gpx_recovery_zones`, `gpx_km_vertical`) en `00011_gpx_muros_recovery_kmvertical.sql`, NO columnas JSONB. Razón: consistencia con `gpx_climbs` y `gpx_risk_zones`; muros es lista (no singleton), recovery zones son semánticamente distintas de risk zones.
- **`StoredTrackDetail` crece 3 campos top-level**: `Muros`, `RecoveryZones`, `KmVertical`. Alineado con patrón actual (`Climbs`, `RiskZones`).
- **Wire shape del upload se refactoriza, NO es breaking**: el cuerpo de respuesta del POST deja de emitir un wrapper anónimo bespoke que duplicaba los campos top-level. Ahora serializa `StoredTrackDetail` directamente. Como `StoredTrackDetail` gana los mismos 3 campos (`Muros`, `RecoveryZones`, `KmVertical`) que el wrapper exponía, las claves top-level `muros` / `recovery_zones` / `km_vertical` siguen presentes en el body con los mismos valores — solo cambia la fuente de los datos (campos del struct vs wrapper anónimo). El frontend SPA actual solo parsea `{ id: string }` desde el upload, así que el cambio es seguro para los consumidores actuales. **No es un breaking change.**
- **Comparator (`computeDiff`) NO se extiende en este slice**: mantiene su superficie de 6 métricas. Extender a muros/km/recovery es un follow-up que consume los campos ya estables.
- **No backfill** de tracks existentes: los detectores son deterministas, pero los tracks pre-existentes mantienen su estado vacío de muros/km/recovery. Backfill es un change aparte si se necesita.
- **PR2 web** solo renderiza: tres presentacionales `RouteKmVertical` / `RouteMuros` / `RouteRecovery` siguiendo el patrón `RouteClimbs.tsx` / `RouteRisks.tsx`.

## Out of scope (explícito en proposal)

- **3D MapLibre + heatmap de pendientes + perfil de elevación en mapa**: deferido a Fase 1.6 (`docs/roadmap/roadmap.md:38`, `docs/architecture/feature-inventory.md:108`).
- **Comparador extendido**: como se indica arriba.
- **Re-run de análisis en tracks existentes**: ver backfill.
- **UX redesign del track-detail**: solo se añaden 3 paneles en el orden existente.

## PR1 — Backend persistence + wire shape (~300 LoC)

**Branch**: `feat/phase-1.3-km-vertical-pr1-backend`.

### Tareas
1. **Migración `00011_gpx_muros_recovery_kmvertical.sql`** con `goose Up` (3 tablas + índices + UNIQUE en `gpx_km_vertical.track_id`) y `goose Down` (DROP de las 3 tablas en orden inverso).
2. **SQLC queries** (3 archivos nuevos en `internal/db/queries/`):
   - `gpx_muros.sql`: `CreateGPXMuro`, `ListGPXMurosByTrack`.
   - `gpx_recovery_zones.sql`: `CreateGPXRecoveryZone`, `ListGPXRecoveryZonesByTrack`.
   - `gpx_km_vertical.sql`: `UpsertGPXKmVertical`, `GetGPXKmVerticalByTrack`.
3. **`make generate`** y revisión del diff en `internal/db/sqlc/*` (esperado: nuevos métodos en `querier.go`, nuevo método en `models.go` si aplica).
4. **`internal/gpx/types.go`**: añadir `Muros []Muro`, `RecoveryZones []RecoveryZone`, `KmVertical *KmVerticalResult` a `StoredTrackDetail`.
5. **`internal/gpx/store.go`**: extender `gpxQuerier` con los 6 métodos, cambiar firma de `CreateDetail` para aceptar `muros`, `recoveryZones`, `kmVertical`, e insertar las 3 nuevas llamadas dentro de `createDetail`. Extender `GetDetail` para re-hidratar los 3 campos desde la DB.
6. **`internal/http/handlers/gpx_upload.go`**: reemplazar el wrapper `escribirJSON` huérfano por `escribirJSON(w, detail)`. Extender `UploadGPXStore` para reflejar nueva firma de `CreateDetail`.
7. **Tests** (TDD antes que código de producción, según `openspec/config.yaml`):
   - `internal/gpx/store_test.go`: extender mock `gpxQuerier` para capturar las 6 nuevas llamadas; añadir `TestSQLCStoreCreateDetailPersistsMurosAndRecoveryZonesAndKmVertical` y extender `TestSQLCStoreGetDetailHydratesChildren` para incluir los 3 nuevos campos.
   - `internal/http/handlers/gpx_upload_test.go`: mover las aserciones top-level `muros`/`recovery_zones`/`km_vertical` a aserciones sobre `store.createdTrack` / `store.createdMuros` / `store.createdKmVertical`. El mock `uploadGPXStore` gana 3 campos capturados.
   - `internal/http/handlers/gpx_get_test.go`: nuevo `TestGetGPXRehydratesMurosAndKmVerticalAndRecoveryZones` que inyecta un `detailGPXStore` con los 3 campos poblados y verifica el JSON.

### Criterio de aceptación PR1
- ✅ `make fmt vet test` verde.
- ✅ `make lint` verde.
- ✅ Migración forward + backward limpio (probado en CI con `timescaledb`).
- ✅ `StoredTrackDetail` lleva los 3 campos poblados después de un upload nuevo.
- ✅ `GET /api/v1/gpx/{id}` devuelve los 3 campos si la DB los tiene.
- ✅ Upload body ya no lleva los 3 keys huérfanas.

## PR2 — Web UI (~250 LoC)

**Branch**: `feat/phase-1.3-km-vertical-pr2-web`.

### Tareas
1. **Tipos web**: `StoredTrackDetail` (en `web/src/lib/api/gpx.ts`) gana 3 campos que espejean el Go struct.
2. **Normalización**: extender `normalizeGPXDetail` (o equivalente) para mapear los nuevos campos del wire a los tipos web.
3. **Componentes presentacionales** (3 archivos en `web/src/features/gpx/TrackDetail/`):
   - `RouteKmVertical.tsx` (panel con la distancia y ganancia del tramo).
   - `RouteMuros.tsx` (lista de muros con métricas por muro).
   - `RouteRecovery.tsx` (lista de zonas de recuperación con distancia).
   - Cada uno con su `.module.css` y `.test.tsx` (render contract).
4. **Integración en `TrackDetailPage`**: añadir los 3 paneles en el orden actual de la página (después de `RouteRisks`, antes de cualquier futura sección).
5. **Test del container** que verifica que los 3 campos llegan normalizados desde la API al presentacional.

### Criterio de aceptación PR2
- ✅ `pnpm typecheck/lint/format:check/test` verde.
- ✅ Página detalle muestra los 3 paneles cuando los datos están poblados.
- ✅ Página no rompe para tracks pre-existentes (campos vacíos → panel vacío o ausente, sin 500).

## Riesgos identificados

- **PR1 cambia la firma de `CreateDetail`**: afecta a todos los call-sites (handlers, tests, fixtures). Mitigación: actualizar todo en el mismo PR; CI lo cazaría si se olvida algo.
- **El mock `databaseTrack`** en `internal/gpx/store_test.go` puede tener un guard que asume la forma anterior; verificar.
- **El upload body no es breaking**: el refactor de wrapper anónimo → campos del struct preserva los nombres de los 3 keys top-level. No hace falta release note para clientes del upload (siguen recibiendo `muros` / `recovery_zones` / `km_vertical` igual que antes).

## Rollback

- **PR1**: `goose Down` de la migración + revert del commit. Las tablas son net-new, no tienen datos pre-existentes que perder.
- **PR2**: revert del commit, los tipos web vuelven a la forma anterior sin tocar backend.
- **Combinado**: revert de ambos commits en orden inverso.

## Estado

- [x] Change OpenSpec importado y commiteado (`8928a2b`).
- [x] Preflight SDD confirmado: execution=auto, store=openspec, delivery=ask-on-risk, chain=stacked-to-main, budget=400.
- [x] sdd-spec ejecutado. Outputs:
  - `openspec/changes/phase-1-3-km-vertical-and-king-climb/specs/gpx-backend/spec.md` (743 líneas, 8 ADDED requirements, 34 scenarios)
  - `openspec/changes/phase-1-3-km-vertical-and-king-climb/specs/gpx-lab/spec.md` (431 líneas, 7 ADDED requirements, 17 scenarios)
- [x] sdd-design ejecutado (`design.md`, 140 inserciones). D1–D5 decisions + data flow + file-by-file + test strategy + rollback.
- [x] sdd-tasks ejecutado (`tasks.md`, 26 tasks, 1.1–1.16 PR1, 2.1–2.10 PR2, TDD posture on every behaviour task).
- [x] sdd-apply PR1 ejecutado. Branch `feat/phase-1.3-km-vertical-pr1-backend`, 8 commits (`9073ae8` … `1dc325c`).
- [x] sdd-verify PR1 round 3 ejecutado: **approve-with-followups**. Blocadores resueltos con regression pin (commits `c617939`, `b7b04fb`, `7379d80`).
- [x] PR1 pusheado, PR #237 abierto, CI verde (Backend / Frontend / Release / GitGuardian), mergeado a main con `--squash --delete-branch`. Merge commit `3ae8102`. Rama local eliminada.
- [x] Branch `feat/phase-1.3-km-vertical` rebaseado sobre main (commits `9531fd5` → `40f0813`). `tasks.md` con 15/26 checks (PR1 done) + 11 pending (PR2).
- [x] sdd-archive PR1: pendiente (no se ejecuta hasta cerrar el change completo; el archive cubre ambos PRs). *(redundant — consolidated below)*
- [x] sdd-tasks PR2: pendiente (rebalance de tasks ya hecho en rebase). *(rebalance confirmed in rebase)*
- [x] sdd-apply PR2: rama `feat/phase-1.3-km-vertical-pr2-web`, 7 commits (`60d49dc` … `63a878b`). Tasks 2.1–2.10 done; gates typecheck + lint + format:check + build verdes localmente.
- [ ] sdd-verify PR2: pendiente.
- [ ] sdd-archive final: pendiente (cierra el change tras merge de PR2; incluye sync de deltas a `openspec/specs/{gpx-backend,gpx-lab}/spec.md`).

## Referencias

- Issue #15.
- `openspec/changes/phase-1-3-km-vertical-and-king-climb/exploration.md` (615 líneas, validated against repo).
- `openspec/changes/phase-1-3-km-vertical-and-king-climb/proposal.md` (818 líneas, user-confirmed 2-PR chain).
- `docs/architecture/feature-inventory.md` §6.
- `docs/roadmap/roadmap.md`.
- `internal/gpx/climbs.go:108,142,183` (los 3 detectores).
- `internal/http/handlers/gpx_upload.go:142-146` (wrapper huérfano actual).
- `internal/gpx/types.go:151-156` (StoredTrackDetail a extender).
- `internal/db/migrations/00006_gpx_tracks.sql` (patrón de tablas para climbs/risk).
