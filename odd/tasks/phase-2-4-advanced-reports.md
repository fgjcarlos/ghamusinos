# Phase 2.4 — Advanced reports (#304)
Delivery: ask-on-risk; local slices by behavior, likely well over 400 authored lines across the epic.

## Specs

### S1 — #304

### Objetivo
Resumir la evolución del corredor por semana, mes y bloque de entrenamiento.

### Entregables
- Reportes semanales y mensuales: evolución de carga, fatiga, desnivel y rendimiento, reutilizando `training_load_daily` e `internal/metrics`.
- Bloques de entrenamiento: organizar semanas en bloques (PRD §10) y resumen por bloque.
- Un único modelo de snapshot en `internal/reports`, que unifica los legacy `runner_profile_snapshots` y `training_analysis_snapshots` (feature-inventory §12).
- Cron de reporte semanal con `PeriodicJobs` de River. Hoy no hay ningún job periódico; es infraestructura nueva con fan-out por usuario.
- Pantalla de reportes.

### Criterio de cierre
Cada semana se genera automáticamente el reporte del usuario, y en la web se consultan los reportes semanales, mensuales y por bloque.

### Tareas
- [ ] #317
- [ ] #318
- [ ] #319
- [ ] #320

### Dependencias
Fases 2.1–2.3.

### Referencias
- `docs/roadmap/roadmap.md` (Fase 2.4)
- `docs/architecture/feature-inventory.md` §10 y §12
- `internal/db/queries/training_load.sql`: `ListUserIDsForTrainingLoadRecalc`, ya pensado para un fan-out futuro



### S2 — #317

### Problema / objetivo

El PRD (§10, «Bloques») pide organizar semanas en bloques de entrenamiento, y la 2.4 pide resúmenes por bloque. No existe el concepto de bloque.

### Propuesta

**Migración** (up/down) de `training_blocks`:

- `user_id`
- `name`
- `start_date`, `end_date`, con `CHECK end >= start`
- `race_id`: nullable, FK a `races` con `SET NULL`
- `notes`
- timestamps

**CRUD** `/api/v1/blocks`. Hay que decidir en el change si se permiten bloques solapados. Propuesta: no se permiten, con 409.

**UI mínima** para crear y editar bloques, en la pantalla de reportes o en el calendario.

### Fase del roadmap

2.4

### Criterios de aceptación

- [ ] La migración es reversible y pasa el smoke de la CI.
- [ ] Tests de handler: CRUD, rango inválido, solapamiento y 404 entre usuarios.
- [ ] Se puede crear un bloque desde la web.

### Contexto / referencias

- Epic: #304. Depende de #303 (`races`).


### S3 — #318

### Problema / objetivo

Hacen falta reportes semanales, mensuales y por bloque. El legacy tenía dos tablas de snapshot (`runner_profile_snapshots` y `training_analysis_snapshots`), y `docs/architecture/feature-inventory.md` §12 exige **unificarlas** en un único modelo.

### Propuesta

**Migración** `report_snapshots`:

- `user_id`
- `kind`: `week`, `month` o `block`
- `period_start`, `period_end`
- `block_id`: nullable
- `payload jsonb`, con versión de schema
- `generated_at`
- `UNIQUE (user_id, kind, period_start)`

**Generador** en `internal/reports`. Es una función pura más una capa de persistencia idempotente: regenerar sustituye el snapshot.

**Contenido del payload:**

- Evolución de carga, fatiga y forma (CTL/ATL/TSB) desde `training_load_daily` e `internal/metrics`.
- Volumen, D+ y número de actividades.
- Rendimiento: EF y GAP medios.
- Cumplimiento del plan, de la 2.2.
- Comparación con el periodo anterior.

### Fase del roadmap

2.4

### Criterios de aceptación

- [ ] Tests de tabla del generador: periodo sin actividades, semana que cruza un mes y bloque con carrera.
- [ ] Regenerar el mismo periodo no duplica filas (test de integración).
- [ ] El schema del payload está versionado y documentado.

### Contexto / referencias

- Epic: #304. Depende de #317, #302 y #303.


### S4 — #319

### Problema / objetivo

El reporte semanal debe generarse solo. Hoy River no tiene ningún job periódico (`internal/jobs/river.go`): es infraestructura nueva.

### Propuesta

- Configurar `PeriodicJobs` de River con un job semanal de fan-out que encola un job de reporte por usuario. Se puede reutilizar o generalizar `ListUserIDsForTrainingLoadRecalc` de `internal/db/queries/training_load.sql`, que ya se pensó para un fan-out futuro.
- El job por usuario genera la semana cerrada en su zona horaria y, el día 1, el mes cerrado. Es idempotente gracias al `UNIQUE` de los snapshots.
- La periodicidad es configurable y el cron se puede desactivar por config. Documentar la variable.
- Reintentos con el patrón existente de `internal/jobs`.

### Fase del roadmap

2.4

### Criterios de aceptación

- [ ] Tests del fan-out y del cálculo de «semana cerrada» en una zona distinta de UTC.
- [ ] Ejecutar el job dos veces no duplica reportes.
- [ ] Con el cron desactivado no se registra el periodic job (test).
- [ ] La config está documentada en `docs/development.md` y en el `.env.example`.

### Contexto / referencias

- Epic: #304. Depende de #318.


### S5 — #320

### Problema / objetivo

Los reportes generados no tienen ni API ni pantalla.

### Propuesta

**API:**

- `GET /api/v1/reports?kind=&from=&to=`, paginado.
- `GET /api/v1/reports/{id}`.
- `POST /api/v1/reports/generate` para generar a demanda un periodo o bloque. Encola un job y no bloquea la petición.

**Ruta `/reportes`** en `web/src/features/reports`:

- Selector semana / mes / bloque.
- Lista de periodos.
- Detalle con gráficas ECharts (carga y fatiga, volumen, D+) y comparación con el periodo anterior, reutilizando los componentes del dashboard.
- Aviso de métricas orientativas.

### Fase del roadmap

2.4

### Criterios de aceptación

- [ ] Tests de handler: filtros, paginación inválida y 404 entre usuarios.
- [ ] La pantalla tiene estados vacío, cargando y error, y tests web.
- [ ] Generar un periodo a demanda muestra su estado hasta que el reporte está listo.

### Contexto / referencias

- Epic: #304. Depende de #318 y #319.


## Tasks

- [x] T1 · S1–S2 · Training blocks: reversible migration, store, CRUD `/api/v1/blocks` with 409 on overlap · parent · commit 6f1c832; high / independent verifier PASS
- [ ] T2 · S1, S3 · Unified `report_snapshots` model and pure, idempotent report generator · parent · commit —
- [ ] T3 · S1, S4 · Weekly report cron with River PeriodicJobs, fan-out per user and config toggle · parent · commit —
- [ ] T4 · S1, S5 · Reports API: filtered paginated list, detail and on-demand generate · parent · commit —
- [ ] T5 · S1–S2, S5 · `/reportes` screen with week/month/block selector, ECharts detail and block management · parent · commit —

## Log

- L1: Resuelve ahora la issue #304
- L2 (T1; slice `6f1c832`, 1170 diff lines including SQLC and tracking): RED the integration package did not compile because `internal/blocks` did not exist. GREEN blocks handler integration (CRUD, invalid range, six overlap cases, foreign race and block 404s, rejected writes leave data unchanged, eight concurrent creates give one 201), route-table tests gained exactly the five block routes, migration down and up on a live database, unit suite, CI integration commands, vet and golangci-lint 2.12.2 (0 issues), all with `TZ=UTC`. Risk: item 2 (ownership checks for blocks and their race link).
- L3 (T1): Independent verifier met S1 (T1 subset) and S2. Fixed its advisory: the overlap query comment now says a zero UUID is passed when creating. Open bug #310 makes `internal/jobs` fail in CET; it passes with `TZ=UTC` and is unrelated to this phase.
- L4: Local verification runs against a throwaway embedded PostgreSQL 16 without TimescaleDB (migration 00007 marked applied only there); the CI Timescale image covers the real extension.
