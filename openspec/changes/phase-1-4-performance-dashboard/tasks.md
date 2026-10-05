# Tasks: phase-1-4-performance-dashboard

> Cadena 4-PR stacked-to-main. Delivery=ask-on-risk. Budget=400 LoC por PR.
> TDD posture: strict_tdd=false a nivel workspace; RED-GREEN-REFACTOR
> por comportamiento al inicio de cada slice; documentar comando de test
> por slice.
>
> Issue epic: #16. Specs referenciadas: M-001..M-010, TL-001..TL-010,
> DA-001..DA-006, DW-001..DW-008 (30 reqs, 61 scenarios).

## Tareas de coherencia previas (P0-P5)

> Bloquean PR1. Resuelven los gaps de coherencia G-α..G-ζ detectados entre
> exploration, proposal, specs y design. Sin estas, las fórmulas divergen
> entre docs y código.

- [x] **P0** — Fijar rango `users.running_threshold_sec_per_km` a 120..1800 (G-α)
- Acción: editar `openspec/changes/phase-1-4-performance-dashboard/exploration.md`, `proposal.md`, `specs/training-load/spec.md` (TL-003) y `design.md` (D2) para alinear 120..1800 (reemplazar menciones 180..1800 en exploration).
- Comando de test: solo `edit` (sin ejecución).
- Resultado esperado: `grep -RE "running_threshold_sec_per_km.*(12[0-9]|18[0-9])\.\.1800" openspec/changes/phase-1-4-performance-dashboard/` lista exactamente los cuatro archivos y todos dicen 120..1800.
- Work-unit commit: `docs(phase-1-4): align running_threshold_sec_per_km range to 120..1800 (#16)`

- [x] **P1** — Fijar TSSRunning canónica en proposal+spec+design (G-β)
- Acción: documentar la fórmula canónica en `internal/metrics/SPEC.md` (creado en PR1 task 1.0) y referenciarla en `specs/metrics-go/spec.md` (M-002) y `proposal.md`. Fórmula: `TSS_running = (durationSec / 3600) × (running_pace_threshold_sec_per_km / actual_pace_sec_per_km)^2 × 100`.
- Comando de test: scenario M-002 alineado en spec; no se ejecuta código aquí.
- Resultado esperado: spec M-002 contiene la fórmula exacta y un scenario "pace faster than threshold → IF > 1".
- Work-unit commit: `docs(phase-1-4): fix canonical TSSRunning formula and cross-references (#16)`

- [x] **P2** — Fijar Cardiac Drift canónica (G-γ)
- Acción: documentar la fórmula canónica en `internal/metrics/SPEC.md` (PR1 task 1.0), `specs/metrics-go/spec.md` (M-006) y `proposal.md`. Fórmula: `drift_pct = (HR_end - HR_start) / HR_start × 100`. Edge: `HR_start == 0` → `drift_pct = 0` (no panic).
- Comando de test: sin código aún; validar que la spec describe el edge case.
- Resultado esperado: M-006 contiene la fórmula, un scenario nominal y un scenario `HR_start=0 → drift=0`.
- Work-unit commit: `docs(phase-1-4): canonicalize cardiac drift formula and HR_start=0 edge (#16)`

- [x] **P3** — Resolver `/healthz` degradado (G-δ)
- Acción: alinear `proposal.md`, `specs/dashboard-api/spec.md` (DA-005, DA-006) y `design.md` (D4). Forma: si DB caída → HTTP 503 con `{status:"degraded", db:{ok:false}}` siempre, con o sin header `X-Internal-Health`. Forma sana con header: `{status:"ok", db:{ok:true}, strava:{...}, last_recalc_at, training_load_rows}`. Forma sana sin header: `{status:"ok"}` con 200.
- Comando de test: sin código; validar shape de spec DA-005/DA-006.
- Resultado esperado: ambos scenarios (con y sin header) recogen `db.ok=false` cuando DB caída.
- Work-unit commit: `docs(phase-1-4): align /healthz degraded semantics across proposal and specs (#16)`

- [x] **P4** — Cardiac Drift UI: mensaje cuando panel oculto (G-ε)
- Acción: documentar en `specs/dashboard-web/spec.md` (DW-005) y `proposal.md` que si `CardiacDriftCard` se oculta del DOM, el `DashboardSummaryCard` o el wrapper muestra un texto "necesita streams HR".
- Comando de test: sin código; validar copy en spec.
- Resultado esperado: DW-005 incluye scenario "no HR streams → card oculta + texto informativo presente".
- Work-unit commit: `docs(phase-1-4): add cardiac-drift fallback message contract (#16)`

- [x] **P5** — EMA sin warm-up (G-ζ)
- Acción: documentar en `specs/metrics-go/spec.md` (M-007) y `proposal.md` que la EMA arranca con cero hasta tener datos (`today_with_warmup = today + max(0, n_days-1) × 0`); los días previos a la primera actividad no influyen en el cálculo.
- Comando de test: sin código; validar descripción en spec.
- Resultado esperado: M-007 incluye scenario "primera actividad en día N → CTL en N se calcula solo con TSS de N".
- Work-unit commit: `docs(phase-1-4): document EMA cold-start with no warm-up (#16)`

---

## PR1 — `feat/phase-1.4-pr1-metrics-pure-go` (TDD)

> Slice puro-Go. Budget ~280 LoC. TDD posture RED→GREEN→REFACTOR por
> comportamiento. Cobertura `internal/metrics` ≥ 90% (criterion merge).
> Branch base: `main`. Merge strategy: squash al trunk de la cadena (stacked).

- [x] **1.0** (pre) — Crear `internal/metrics/SPEC.md` con fórmulas y referencias literarias
- Acción: crear `internal/metrics/SPEC.md` referenciando M-001..M-010 y citando las fórmulas canónicas fijadas en P1, P2 y P5. Documentar unidades, bordes y orden de precondición de cada función pública.
- Comando de test: `git diff --stat internal/metrics/SPEC.md` (archivo nuevo, ~60 líneas).
- Resultado esperado: SPEC.md enlaza cada fórmula a su requirement (M-002 → TSSRunning, M-006 → CardiacDrift, M-007 → CTL/ATL/TSB).
- Work-unit commit: `docs(metrics): add SPEC.md anchoring canonical formulas (#16)`

- [x] **1.1** — TSSCycling con tests golden
- Acción: implementar `internal/metrics/cycling.go` con `TSSCycling(durationSec, ftp, np)` y tests `cycling_test.go` con tres casos golden: 1h@FTP→100, 30min@FTP→50, duración 0→0. RED primero (tests fallan), luego GREEN.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run TSSCycling -v`.
- Resultado esperado: tres tests verdes; `go test -cover ./internal/metrics/...` ≥ 80% provisional.
- Work-unit commit: `feat(metrics): TSSCycling with golden tests (RED→GREEN) (#16)`

- [x] **1.2** — TSSRunning (fórmula canónica de P1)
- Acción: implementar `internal/metrics/running.go` con `TSSRunning(durationSec, thresholdSecPerKm, actualPaceSecPerKm)` y tests: 1h@umbral→100, pace más rápido que umbral→TSS>100, pace más lento→TSS<100, duración 0→0. RED→GREEN.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run TSSRunning -v`.
- Resultado esperado: cuatro tests verdes; alineado con M-002 scenario "pace faster than threshold → IF > 1".
- Work-unit commit: `feat(metrics): TSSRunning canonical formula with tests (#16)`

- [x] **1.3** — IntensityFactor
- Acción: `internal/metrics/if.go` con `IntensityFactor(np, ftp)` y `IntensityFactorRunning(velocity, thresholdVelocity)`. Tests: NP=FTP→1.0, NP=0.7*FTP→0.7, divide-by-zero→IF=0.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run IntensityFactor -v`.
- Resultado esperado: tests verdes; edge case divide-by-zero manejado sin panic.
- Work-unit commit: `feat(metrics): IntensityFactor for cycling and running (#16)`

- [x] **1.4** — GradeAdjustedPace (Minetti-normalized)
- Acción: `internal/metrics/gap.go` con `GradeAdjustedPace(distanceM, elapsedSec, grade)` aplicando la curva de coste metabólico de Minetti. Tests: llano→pace original, subida 5%→pace equivalente más lento, llano 0%→sin cambio.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run GradeAdjusted -v`.
- Resultado esperado: tres tests verdes; alinea con M-004.
- Work-unit commit: `feat(metrics): GradeAdjustedPace Minetti-normalized (#16)`

- [x] **1.5** — EfficiencyFactor
- Acción: `internal/metrics/ef.go` con `EfficiencyFactor(np, avgHr)`. Tests: NP=200W, HR=150→EF=1.333; HR=0→EF=0 (no panic).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run EfficiencyFactor -v`.
- Resultado esperado: dos tests verdes; alinea con M-005.
- Work-unit commit: `feat(metrics): EfficiencyFactor with HR=0 edge case (#16)`

- [x] **1.6** — CardiacDrift y CardiacDriftSeries (fórmula canónica de P2, alineado con M-006)
- Acción: `internal/metrics/health.go` con dos funciones: `CardiacDrift(hrStart, hrEnd float64) float64` (Pauley convention: `(hrEnd - hrStart) / hrStart * 100`; si `hrStart <= 0` devuelve `0.0` sin panic, sin NaN) y `CardiacDriftSeries(samples []int) float64` (compara la media del primer cuartil vs la media del tercer cuartil — más conservador que first-half vs second-half; evita el warm-up bias). Si `samples` está vacío o tiene menos de 2 muestras, devuelve `0.0`. **NO usar `CardioDriftSeries` con params `(hrStream []int, t, l, h int) (float64, error)`** — esa API estaba en una versión anterior de esta task y NO está en M-006.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run CardiacDrift -v`.
- Resultado esperado: al menos 4 tests verdes: HR 100→110 yields 10%; `CardiacDriftSeries([]int{})` returns `0.0` sin panic; primer cuartil = tercer cuartil yields 0%; HR drift normal ~5%.
- Work-unit commit: `feat(metrics): CardiacDrift and CardiacDriftSeries canonical (RED→GREEN) (#16)`

- [x] **1.7** — CTL / ATL / TSB con constantes TauCTL=42, TauATL=7 (alineado con M-007, M-008, M-009)
- Acción: `internal/metrics/fatigue.go` define el tipo `DailyLoad struct { Day time.Time; TSS float64 }` y las funciones `CTL(daily []DailyLoad) float64`, `ATL(daily []DailyLoad) float64`, `TSB(ctl, atl float64) float64`. EMA exponencial con τ=42 (CTL) y τ=7 (ATL): `CTL_t = CTL_{t-1} + (TSS_t − CTL_{t-1}) × (1 − exp(−1/τ))`. Constantes `TauCTL = 42` y `TauATL = 7` **EXPORTADAS** (PR2 las reutiliza). `CTL/ATL([]DailyLoad{})` → `0.0`. **NO usar `EBalance(dailyTSS []float64) (ctl, atl, tsb []float64)`** — esa API estaba en una versión anterior y NO está en M-007/M-008/M-009 (que reciben `[]DailyLoad`, no `[]float64`).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run 'CTL|ATL|TSB' -v`.
- Resultado esperado: al menos 6 tests verdes: TSS=100 constante sobre 42 días → CTL≈100; TSS=100 sobre 7 días → ATL≈100; CTL=42.1, ATL=31.0 → TSB≈11.1; CTL=ATL → TSB=0; serie vacía → 0; TSS=0 cae ATL asintóticamente.
- Work-unit commit: `feat(metrics): CTL/ATL/TSB with DailyLoad and Tau constants (RED→GREEN) (#16)`

- [x] **1.8** — FillMissingDays con `[]DailyLoad` y UTC-day granularity (alineado con M-010 y P5)
- Acción: `internal/metrics/fatigue.go` añade `FillMissingDays(from, to time.Time, raw []DailyLoad) []DailyLoad`. Devuelve exactamente `(to.Sub(from).Hours()/24) + 1` filas en UTC-day granularity. Días presentes en `raw` se trasladan con su TSS original; días ausentes → TSS=0 (P5: EMA cold-start sin warm-up, la serie resultante va tal cual a CTL/ATL). Si `from > to`, devuelve `nil` o slice vacío sin panic. **NO usar `FillMissingDays(..., map[string]float64) []Day`** — la API canónica recibe `[]DailyLoad` (tipo de M-007), no `map`.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run FillMissingDays -v`.
- Resultado esperado: al menos 3 tests verdes: 7-day range con 3 missing → 7 filas con TSS 0 en huecos; `from > to` returns empty sin panic; serie vacía `raw=[]DailyLoad{}` → todas TSS=0.
- Work-unit commit: `feat(metrics): FillMissingDays UTC-day granularity with no warm-up (#16)`

- [x] **1.9** — Cobertura `internal/metrics` ≥ 90% (criterion)
- Acción: añadir tests adicionales si la cobertura no llega; revisar ramas no cubiertas. Sin código nuevo más allá de tests.
- Comando de test: `GOTOOLCHAIN=local go test -cover ./internal/metrics/...` (debe mostrar `coverage: ≥ 90.0%`).
- Resultado esperado: línea de cobertura ≥ 90%; todas las funciones públicas tienen al menos un test.
- Work-unit commit: `test(metrics): raise coverage to ≥ 90% (#16)`

- [x] **1.10** — `make fmt/vet/test/lint` verde
- Acción: ejecutar toolchain completo; resolver cualquier warning de `go vet` o `golangci-lint`. Sin código nuevo.
- Comando de test: `GOTOOLCHAIN=local make fmt && GOTOOLCHAIN=local make vet && GOTOOLCHAIN=local make test && GOTOOLCHAIN=local make lint`.
- Resultado esperado: los cuatro comandos exit 0; sin diffs residuales de `gofmt`.
- Work-unit commit: `chore(metrics): make fmt/vet/test/lint clean (#16)`

- [ ] **1.11** — Commit con conventional commit y push branch
- Acción: revisar historial; squash si hay commits de "wip"; push a `origin/feat/phase-1.4-pr1-metrics-pure-go`.
- Comando de test: `git log --oneline -10` (mensajes conventional), `git push -u origin feat/phase-1.4-pr1-metrics-pure-go`.
- Resultado esperado: branch remoto actualizado; todos los commits del PR1 forman un narrativa coherente (SPEC → TSSCycling → TSSRunning → IF → GAP → EF → CardiacDrift → CTL/ATL/TSB → FillMissingDays → coverage → fmt/vet).
- Work-unit commit: (no se crea commit nuevo; es la publicación del conjunto).

- [ ] **1.12** — PR abierto contra `main`, CI verde, merge
- Acción: `gh pr create --base main --head feat/phase-1.4-pr1-metrics-pure-go --title "feat(metrics): pure-Go training metrics package (PR1/4)" --body-file .github/pr-templates/phase-1-4-pr1.md`. Esperar CI verde. Squash & merge a `main`.
- Comando de test: `gh pr checks` (todos en verde), `gh pr view --json state` (`MERGED`).
- Resultado esperado: PR mergeado; `main` contiene `internal/metrics/` con cobertura ≥ 90%.
- Work-unit commit: (merge commit, no individual).

---

## PR2 — `feat/phase-1.4-pr2-timescaledb-recalc` (schema + worker)

> Budget ~360 LoC. Si excede, split de contingencia en PR2a (schema+queries)
> y PR2b (worker+hook). TDD para worker (mock DB); SQL validado con
> `make migrate` local + tests integration contra el contenedor timescaledb.
> Branch base: `main` (post-PR1).

### Ajustes al plan original detectados en la exploración previa (2026-10-01)

Antes de ejecutar PR2 se confirman estos detalles contra `main`:

- **Tipo de `user_id`**: el codebase ya consolidó `user_id UUID` (no BIGINT como en una versión inicial del plan). PR2 usa UUID.
- **`schema.sql` es la fuente de sqlc**; las migraciones Goose son la fuente en runtime. Hay que tocar ambos para que `make generate` funcione y las migraciones produzcan el mismo esquema.
- **`migrations.go` no tiene guard de timescaledb**: la extensión se asume activa por el `docker-compose` y el CI (imagen `timescale/timescaledb:2.27.2-pg16`). Si quisiéramos soportar Postgres "vanilla", añadiríamos un guard. Para PR2, alineamos con el patrón actual: `create_hypertable` envuelto en `DO $$ ... EXCEPTION WHEN undefined_function THEN ...` que cae a tabla normal con `RAISE NOTICE`.
- **No existe query `ListActivitiesInRange` ni `FirstActivityForUser`** en `internal/db/queries/activities.sql`. Hay que añadirlas.
- **No existe target `db-test-migrations` en Makefile**; usamos `make migrate` + tests Go integration contra el `docker-compose` o `testcontainers-go`.
- **`internal/metrics` PR1 firma todas las funciones con `int`** (no float). `ComputeDailyLoad` debe respetar esto: TSS se acumula en `float64` desde los inputs ya en el wrapper.

### Tareas

- [ ] **2.0** (pre) — Verificar contenedor timescaledb arriba y congelar la rama base
- Acción: `docker compose up -d postgres` (imagen timescale/timescaledb:2.27.2-pg16). `git checkout -b feat/phase-1.4-pr2-timescaledb-recalc main`. Snapshot del HEAD antes del primer commit (anotado en `odd/tasks/phase-1-4-performance-dashboard.md`).
- Comando de test: `docker compose ps` (running) y `git rev-parse HEAD` (registrado).
- Resultado esperado: rama creada; Postgres+timescales operativo.
- Work-unit commit: (sin commit, es un setup).

- [ ] **2.1** — Migración `00012_training_load_daily.sql` (hypertable + dashboard_metadata)
- Acción: crear `internal/db/migrations/00012_training_load_daily.sql`. Esquema:
  - `CREATE TABLE training_load_daily (user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE, day DATE NOT NULL, tss DOUBLE PRECISION NOT NULL DEFAULT 0, activity_count INT NOT NULL DEFAULT 0, distance_m DOUBLE PRECISION NOT NULL DEFAULT 0, elevation_gain_m DOUBLE PRECISION NOT NULL DEFAULT 0, computed_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY (user_id, day));`
  - `CREATE INDEX IF NOT EXISTS training_load_daily_user_day_desc_idx ON training_load_daily (user_id, day DESC);`
  - `do $$ begin perform create_hypertable('training_load_daily', 'day', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE); exception when undefined_function then raise notice 'timescaledb absent, training_load_daily stays plain'; end $$;` (mantiene portabilidad: si un día se ejecuta contra Postgres vanilla, la tabla sigue funcionando como tabla plana).
  - `CREATE TABLE dashboard_metadata (user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, last_recalc_at TIMESTAMPTZ, last_recalc_status TEXT, last_recalc_error TEXT, training_load_rows INT NOT NULL DEFAULT 0, updated_at TIMESTAMPTZ NOT NULL DEFAULT now());`
  - Down simétrico: `DROP TABLE`, `DROP INDEX`, `DROP TABLE`.
- Comando de test: `docker compose up -d postgres && make migrate && make migrate-down && make migrate` (round-trip up/down/up). Verificación adicional: `psql ... -c "\d training_load_daily"` muestra la hypertable o el fallback plano; `docker compose exec postgres psql -U ghamusinos -d ghamusinos -c "SELECT count(*) FROM _timescaledb_catalog.hypertable WHERE hypertable_name='training_load_daily';"` (debe devolver 1 si timescale está activa).
- Resultado esperado: migración forward/backward limpia; tabla existe y la hypertable se crea (o se loggea el notice si la extensión no está).
- Work-unit commit: `feat(db): migration 00012 training_load_daily hypertable + dashboard_metadata (#16)`

- [ ] **2.2** — Migración `00013_users_running_threshold.sql`
- Acción: crear `internal/db/migrations/00013_users_running_threshold.sql` con `ALTER TABLE users ADD COLUMN running_threshold_sec_per_km SMALLINT NULL CONSTRAINT users_running_threshold_chk CHECK (running_threshold_sec_per_km IS NULL OR (running_threshold_sec_per_km BETWEEN 120 AND 1800));`. Down: `DROP CONSTRAINT`, `DROP COLUMN`.
- Comando de test: `make migrate && docker compose exec postgres psql -U ghamusinos -d ghamusinos -c "INSERT INTO users (clerk_user_id, email, running_threshold_sec_per_km) VALUES ('test', 'test@example.com', 100);"` debe fallar con CHECK violation; valor 240 debe pasar.
- Resultado esperado: constraint 120..1800 activa; UP/DOWN reversibles.
- Work-unit commit: `feat(db): migration 00013 users.running_threshold_sec_per_km 120..1800 (#16)`

- [ ] **2.3** — Sincronizar `internal/db/schema.sql` con migraciones 00012 + 00013
- Acción: añadir a `internal/db/schema.sql` las definiciones equivalentes de `training_load_daily`, `dashboard_metadata` y la columna `users.running_threshold_sec_per_km`. `schema.sql` es la fuente que sqlc lee para generar bindings, por lo que debe coincidir con el estado migrado.
- Comando de test: `grep -n "training_load_daily\|dashboard_metadata\|running_threshold_sec_per_km" internal/db/schema.sql` lista las 3 ocurrencias.
- Resultado esperado: sqlc generará los modelos y queries esperados.
- Work-unit commit: `feat(db): sync schema.sql with 00012/00013 for sqlc (#16)`

- [ ] **2.4** — Queries sqlc: `internal/db/queries/training_load.sql` + `dashboard_metadata.sql` + `activities_ext.sql`
- Acción: crear tres archivos de queries:
  - `training_load.sql`: `UpsertTrainingLoadDaily :one`, `ListTrainingLoadRange :many` (user_id, from, to orden ascendente), `ListTrainingLoadFromFirstActivity :many` (user_id; para la primera actividad del usuario hasta hoy), `ListUserIDsForTrainingLoadRecalc :many` (user_id distinto).
  - `dashboard_metadata.sql`: `UpsertDashboardMetadata :one`, `GetDashboardMetadata :one`, `IncrementDashboardMetadataRows :exec` (SUM +1 por recalc), `SetDashboardMetadataRecalcStatus :exec`.
  - `activities_ext.sql`: `ListActivitiesInRange :many` (user_id, from, to — solo campos necesarios para TSS), `FirstActivityForUser :one` (MIN(started_at) por user_id).
- Comando de test: `make generate` (regenera `internal/db/sqlc/`); `git diff --stat internal/db/sqlc/` debe listar los nuevos archivos `training_load.sql.go`, `dashboard_metadata.sql.go`, `activities_ext.sql.go` con los nombres correctos.
- Resultado esperado: sqlc genera bindings; tests de sqlc existentes siguen verdes.
- Work-unit commit: `feat(db): sqlc queries for training_load + dashboard_metadata + activities_ext (#16)`

- [ ] **2.5** — `internal/metrics/training_load.go` (ComputeDailyLoad — orquestador)
- Acción: capa de servicio, no pura. Tipo `ComputeDailyLoadDeps { Queries ActivityRangeQuerier; ActivityLoader ...; ... }`. Función `ComputeDailyLoad(ctx context.Context, deps ComputeDailyLoadDeps, userID pgtype.UUID, from, to time.Time) ([]metrics.DailyLoad, error)`:
  1. Carga actividades del usuario en el rango.
  2. Por cada actividad: detecta sport_type (cycling/running/otro); calcula TSS usando `metrics.TSSCycling(ftp, durationSec, np)` o `metrics.TSSRunning(thresholdSecPerKm, durationSec, actualSecPerKm)`. `np` ≈ `avg_power` si existe; si no, `IF = distance/(elapsed/3600)` y se deriva `np ≈ FTP * IF` (fallback documentado). Para running, `actualSecPerKm = elapsedSec / (distanceKm)` si hay distancia; si no, omite la actividad (sin pace no se calcula TSS running).
  3. Para "otros" deportes (Walk, Hike, Swim): TSS no se calcula en PR2; se cuentan en `activity_count` pero no en `tss` total (devuelve 0 TSS diario). Documentado en godoc.
  4. Agrupa por día UTC usando `metrics.FillMissingDays` (PR1).
- Tests:
  - `TestComputeDailyLoad_CyclingTSS` con 2 actividades cycling mismo día: TSS agregado correcto.
  - `TestComputeDailyLoadRunningWithoutFTP` con 1 actividad running con `users.ftp=NULL` → TSS=0 sentinel (no panic).
  - `TestComputeDailyLoadGroupsByDay` con 3 actividades distribuidas en 2 días: 2 filas.
  - `TestComputeDailyLoadSkipsOtherSports` (Walk): TSS=0, activity_count>0.
  - Mock de las queries (interface `ActivityRangeQuerier`).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run ComputeDailyLoad -v`.
- Resultado esperado: 4 tests verdes; cobertura de la rama.
- Work-unit commit: `feat(metrics): ComputeDailyLoad service aggregator with mocked queries (#16)`

- [ ] **2.6** — `internal/jobs/recalc_training_load.go` (River worker)
- Acción: tipos `RecalcTrainingLoadArgs{UserID string; From *time.Time; To *time.Time}` con `Kind() = "recalc_training_load"`. Worker:
  1. Parsea `userID` a `pgtype.UUID`.
  2. Lee `dashboard_metadata`; si no hay `last_recalc_at`, llama `FirstActivityForUser` y usa esa fecha como `from`. Si tampoco hay primera actividad, termina con status=`ok` (sin filas).
  3. `ComputeDailyLoad` desde `from` hasta `now`.
  4. Aplica `FillMissingDays` + `CTL/ATL` (PR1).
  5. Upsert por día en `training_load_daily`.
  6. Upsert `dashboard_metadata` con `last_recalc_at=now`, `status='ok'`, `training_load_rows=SUM(count)`.
  7. En caso de error, `status='error'` + `last_recalc_error=err.Error()`.
  8. `UniqueOpts{ByArgs: true, Period: 1 * time.Minute}` (E7-ish, evita runs concurrentes del mismo user).
- Tests:
  - `TestRecalcTrainingLoadWorker_FirstRun` (sin metadata → usa primera actividad).
  - `TestRecalcTrainingLoadWorker_NoActivities` (sin actividades → status ok, 0 filas).
  - `TestRecalcTrainingLoadWorker_Idempotent` (segunda ejecución: no duplica filas; `last_recalc_at` se actualiza; `training_load_rows` estable si no hay nuevas actividades).
  - `TestRecalcTrainingLoadWorker_RecordsError`.
- Mock: `TrainingLoadStore`, `ActivityRangeQuerier`, `DashboardMetadataStore`, `MetricsComputer` (interface; el wrapper de `ComputeDailyLoad`).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/jobs/... -run RecalcTraining -v`.
- Resultado esperado: 4 tests verdes; CTL/ATL/TSB coherentes con PR1.
- Work-unit commit: `feat(jobs): RecalcTrainingLoad River worker with UniqueOpts (#16)`

- [ ] **2.7** — Registrar worker en `NewRiverWorkers`
- Acción: en `internal/jobs/workers.go`, instanciar `NewRecalcTrainingLoadWorker` con las deps (queries + config); añadir `river.AddWorker(workers, w)` y pasar la nueva `RiverArgs` al registrar en `cmd/ghamusinos` (la fila "IngestActivityEvent" ya hace lo mismo). Sin Strava-bound: este se registra siempre (no depende de Strava).
- Comando de test: `GOTOOLCHAIN=local go build ./cmd/ghamusinos` (compila); `grep -n "recalc_training_load\|RecalcTrainingLoad" internal/jobs/workers.go` debe listar el binding.
- Resultado esperado: worker registrado; startup log lo muestra.
- Work-unit commit: `feat(jobs): register RecalcTrainingLoad worker in NewRiverWorkers (#16)`

- [ ] **2.8** — Encolar recalc desde post-`UpsertActivity`
- Acción: en `internal/jobs/workers.go` (función `Work` del `BackfillStravaActivitiesWorker` y `IngestActivityEventWorker`), tras un upsert exitoso, encolar `RecalcTrainingLoadArgs{UserID: userID.String()}` con `UniqueOpts{Period: 1 * time.Minute}` para evitar ráfagas (E7). Esto cubre backfill inicial e ingesta por webhook.
- Tests: añadir a `backfill_test.go` y/o nuevo `strava_test.go` un test que verifique que tras upsert se llama al river inserter con los args correctos. Usar el mock `fakeRiverJobInserter` (ya existe en `river_test.go`) extendido.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/jobs/... -run TestEnqueuesRecalc -v`.
- Resultado esperado: 2 tests verdes (uno por worker que upsertea); idempotente bajo ráfaga.
- Work-unit commit: `feat(jobs): enqueue RecalcTrainingLoad on activity upsert (webhook + backfill) (#16)`

- [ ] **2.9** — Tests integration: idempotencia contra Postgres+timescale
- Acción: test integration `internal/jobs/recalc_training_load_integration_test.go` con `//go:build integration` (mismo patrón que `webhook_integration_test.go`) que:
  1. Crea user + 2 actividades.
  2. Ejecuta `RecalcTrainingLoad` 2 veces.
  3. Verifica `COUNT(*) FROM training_load_daily WHERE user_id=$1` igual entre ambas; valores CTL/ATL/TSB idénticos.
- Comando de test: `GOTOOLCHAIN=local go test -tags=integration ./internal/jobs/... -run TestRecalcIdempotency -v`.
- Resultado esperado: idempotente bajo runs repetidos.
- Work-unit commit: `test(jobs): recalc_training_load idempotency integration (#16)`

- [ ] **2.10** — Tests del worker (mock DB + job runner, sin Postgres)
- Acción: ya cubiertos por 2.6 (4 tests con mocks). Verificar que `TestWorkerSequence` (mock) cubre la cadena: leer metadata → ComputeDailyLoad → FillMissingDays+CTL/ATL → upsert training_load_daily → upsert dashboard_metadata.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/jobs/... -run TestWorkerSequence -v`.
- Resultado esperado: secuencia validada con mocks.
- Work-unit commit: (parte de 2.6; commit adicional solo si se necesita otro test independiente).

- [ ] **2.11** — `make fmt/vet/test/lint` verde
- Acción: ejecutar toolchain completo; resolver warnings.
- Comando de test: `GOTOOLCHAIN=local make fmt && GOTOOLCHAIN=local make vet && GOTOOLCHAIN=local make test && GOTOOLCHAIN=local make lint`.
- Resultado esperado: los cuatro exit 0.
- Work-unit commit: `chore(db): make fmt/vet/test/lint clean (#16)`

- [ ] **2.12** — PR abierto contra `main`, CI verde, merge
- Acción: `gh pr create --base main --head feat/phase-1.4-pr2-timescaledb-recalc --title "feat(db+jobs): training_load_daily + RecalcTrainingLoad worker (PR2/4)" --body-file .github/pr-templates/phase-1-4-pr2.md`. Esperar CI verde (Postgres+timescale). Squash & merge a `main`.
- Comando de test: `gh pr checks` (todos en verde), `gh pr view --json state` (`MERGED`).
- Resultado esperado: PR mergeado; `main` contiene migraciones 00012 y 00013, sqlc bindings, worker y hook.
- Work-unit commit: (merge commit).

> **Contingencia split PR2a/PR2b**: si el diff de PR2 > 400 LoC, dividir en
> PR2a (`feat/phase-1.4-pr2a-schema-queries`: tasks 2.0–2.4) y PR2b
> (`feat/phase-1.4-pr2b-worker-hook`: tasks 2.5–2.12), ambos stacked
> sobre `main`. Si se activa, las tasks 2.5–2.12 se renumeran 2b.1–2b.7 y
> 2.5–2.11 siguen el patrón 1.10/1.12.
- Resultado esperado: compilación verde; esqueleto listo para TDD por endpoint.
- Work-unit commit: `feat(http): dashboard handlers skeleton (#16)`

- [ ] **3.2** — DA-001: `GET /api/v1/dashboard/summary`
- Acción: handler devuelve `{user, kpis:{ctl, atl, tsb, last7_tss, last30_tss}, flags:{has_hr_streams, last_recalc_at}}`. Auth requerida. TDD: RED con `httptest` → 401 sin auth, 200 con shape correcto.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -run DashboardSummary -v`.
- Resultado esperado: dos tests verdes; alineado con DA-001 (3 scenarios).
- Work-unit commit: `feat(http): GET /dashboard/summary handler with auth (#16)`

- [ ] **3.3** — DA-002: `GET /api/v1/dashboard/load`
- Acción: handler con query params `with`, `from`, `to` (default with=90); lee `training_load_daily` y aplica `FillMissingDays` + shape `[{day, ctl, atl, tsb, tss}]`. TDD: ventana válida, ventana vacía, rango inválido (400).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -run DashboardLoad -v`.
- Resultado esperado: tres tests verdes; alinea con DA-002.
- Work-unit commit: `feat(http): GET /dashboard/load with FillMissingDays (#16)`

- [ ] **3.4** — DA-003: `GET /api/v1/dashboard/hr-zones`
- Acción: handler que resume tiempo en zonas HR (z1..z5) desde streams HR; fallback a `avg_hr × elapsed` si no hay streams. TDD: con streams → buckets exactos; sin streams → fallback; sin HR → 204.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -run DashboardHRZones -v`.
- Resultado esperado: tres tests verdes; alinea con DA-003.
- Work-unit commit: `feat(http): GET /dashboard/hr-zones with streams and fallback (#16)`

- [ ] **3.5** — DA-004: `POST /api/v1/dashboard/recalc`
- Acción: handler encola `RecalcTrainingLoadArgs{UserID}` con River; responde 202 con `{job_id, status:"queued"}`. TDD: 202 con job_id, 401 sin auth, 503 si River no disponible.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -run DashboardRecalc -v`.
- Resultado esperado: tres tests verdes; alinea con DA-004.
- Work-unit commit: `feat(http): POST /dashboard/recalc enqueues River job (#16)`

- [ ] **3.6** — DA-005 + DA-006: `/healthz` extendido (P3)
- Acción: modificar handler `/healthz`:
  - Sin header `X-Internal-Health: 1`: 200 `{status:"ok"}` salvo DB caída.
  - Con header: 200 con shape completo `{status, db:{ok, latency_ms}, strava:{...}, last_recalc_at, training_load_rows}`.
  - DB caída: 503 con `{status:"degraded", db:{ok:false}}` siempre, con o sin header.
  TDD: cuatro scenarios (sano sin header, sano con header, DB caída sin header, DB caída con header).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -run Healthz -v`.
- Resultado esperado: cuatro tests verdes; alinea con DA-005, DA-006 y P3.
- Work-unit commit: `feat(http): extended /healthz with X-Internal-Health header (DA-005, DA-006) (#16)`

- [ ] **3.7** — `internal/http/router.go` (montaje de rutas nuevas)
- Acción: registrar `/api/v1/dashboard/{summary,load,hr-zones,recalc}` con middleware de auth existente. Sin tests nuevos (cubierto en 3.2–3.5).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -v` (suite completa).
- Resultado esperado: todas las rutas registradas; suite verde.
- Work-unit commit: `feat(http): register dashboard routes in router (#16)`

- [ ] **3.8** — Tests `httptest` por endpoint
- Acción: añadir tests de contrato adicionales: validación de content-type (`application/json`), validación de query params (`with` fuera de rango → 400), validación de body en POST `/recalc`.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/http/... -v -count=1`.
- Resultado esperado: cobertura de handlers ≥ 85%.
- Work-unit commit: `test(http): contract tests for dashboard endpoints (#16)`

- [ ] **3.9** — Sanity check: `nilza` `auth/internal/resolver_test.go` (mocks ripplen si cambian interfaces)
- Acción: ejecutar `GOTOOLCHAIN=local go test ./... -count=1` completo; si hay fallos en `auth` o `internal/resolver`, reparar mocks (no se cambia la lógica de auth, solo se actualizan los stubs).
- Comando de test: `GOTOOLCHAIN=local go test ./... -count=1`.
- Resultado esperado: toda la suite verde; sin tests skipped por error de mock.
- Work-unit commit: `chore(test): refresh mocks after handler additions (#16)`

- [ ] **3.10** — `make fmt/vet/test/lint` verde
- Acción: toolchain completo.
- Comando de test: `GOTOOLCHAIN=local make fmt && GOTOOLCHAIN=local make vet && GOTOOLCHAIN=local make test && GOTOOLCHAIN=local make lint`.
- Resultado esperado: exit 0 en los cuatro.
- Work-unit commit: `chore(http): make fmt/vet/test/lint clean (#16)`

- [ ] **3.11** — PR merge
- Acción: `gh pr create --base main --head feat/phase-1.4-pr3-api-dashboard`; CI verde; squash & merge.
- Comando de test: `gh pr checks`; `gh pr view --json state`.
- Resultado esperado: PR mergeado; endpoints disponibles en `main`.
- Work-unit commit: (merge commit).

---

## PR4 — `feat/phase-1.4-pr4-web-dashboard` (SPA + ECharts)

> Budget ~330 LoC. Vitest por componente (render contract). Branch base:
> `main` (post-PR3). Tree-shaking ECharts para evitar bundle bloat (E1).

- [ ] **4.1** — `pnpm add echarts` + tree-shaking setup (E1)
- Acción: instalar `echarts`; configurar import explícito `import { use } from 'echarts/core'; use(LineChart, BarChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent)`. Verificar bundle size.
- Comando de test: `pnpm -C web typecheck && pnpm -C web build`; revisar `dist/assets/*.js` < 250KB gzipped para el chunk de dashboard.
- Resultado esperado: build verde; bundle del dashboard dentro de presupuesto.
- Work-unit commit: `chore(web): add echarts with tree-shaking (#16)`

- [ ] **4.2** — `web/src/lib/api/dashboard.ts` (tipos espejo de DA-001..DA-004)
- Acción: definir tipos TypeScript `DashboardSummary`, `TrainingLoadPoint`, `HRZonesSummary` y funciones `fetchDashboardSummary`, `fetchTrainingLoad`, `fetchHRZones`, `postRecalc` usando el cliente HTTP existente.
- Comando de test: `pnpm -C web typecheck`.
- Resultado esperado: tipos compilan; sin `any` en el módulo.
- Work-unit commit: `feat(web): dashboard API client types (#16)`

- [ ] **4.3** — Hooks `useDashboardSummary`, `useTrainingLoad`, `useHRZones`
- Acción: hooks con SWR o react-query (lo que el repo ya use); manejan loading/error/empty. TDD: tests Vitest con renderHook + mocks de fetch.
- Comando de test: `pnpm -C web test:run src/lib/api/dashboard.test.ts src/features/dashboard/hooks.test.ts`.
- Resultado esperado: tres hooks con tests verdes; sin warnings de React.
- Work-unit commit: `feat(web): dashboard hooks with loading/error states (#16)`

- [ ] **4.4** — `web/src/features/dashboard/DashboardSummaryCard/` (KPIs + sparkline)
- Acción: componente que muestra CTL/ATL/TSB actuales, TSS últimas 7 y 30 días, y sparkline mínimo (reutilizar ECharts ya tree-shaken). Tests: render con datos, render con flags vacíos, render con error.
- Comando de test: `pnpm -C web test:run src/features/dashboard/DashboardSummaryCard`.
- Resultado esperado: tres tests verdes; alinea con DW-001..DW-003.
- Work-unit commit: `feat(web): DashboardSummaryCard with KPIs and sparkline (#16)`

- [ ] **4.5** — `web/src/features/dashboard/TrainingLoadChart/` (línea CTL/ATL/TSB)
- Acción: gráfico de líneas con tres series (CTL, ATL, TSB) usando `LineChart` de ECharts. Tooltip y leyenda. Tests: render con N puntos, render con 0 puntos, leyenda visible.
- Comando de test: `pnpm -C web test:run src/features/dashboard/TrainingLoadChart`.
- Resultado esperado: tres tests verdes; alinea con DW-004.
- Work-unit commit: `feat(web): TrainingLoadChart CTL/ATL/TSB line series (#16)`

- [ ] **4.6** — `web/src/features/dashboard/HRZonesChart/` (barras apiladas)
- Acción: barras apiladas z1..z5 con tiempo en minutos. Tests: render con todas las zonas, render con zonas vacías (z4/z5 a 0), tooltip funcional.
- Comando de test: `pnpm -C web test:run src/features/dashboard/HRZonesChart`.
- Resultado esperado: tres tests verdes; alinea con DW-006.
- Work-unit commit: `feat(web): HRZonesChart stacked bars z1..z5 (#16)`

- [ ] **4.7** — `web/src/features/dashboard/CardiacDriftCard/` (condicional a streams HR + mensaje P4)
- Acción: componente que muestra el drift solo si `flags.has_hr_streams === true`; si no, NO renderiza el panel pero el `DashboardSummaryCard` o wrapper muestra un texto "necesita streams HR" (G-ε, P4). Tests: con streams → render, sin streams → card ausente + texto presente.
- Comando de test: `pnpm -C web test:run src/features/dashboard/CardiacDriftCard`.
- Resultado esperado: dos tests verdes; alinea con DW-005 y P4.
- Work-unit commit: `feat(web): CardiacDriftCard conditional with fallback message (#16)`

- [ ] **4.8** — `web/src/pages/Dashboard/` (integración + ruta + nav)
- Acción: página que monta los cuatro componentes en grid responsive; añade ruta `/dashboard` y entrada en el menú principal. Tests: smoke test que verifica que la página monta sin error con datos mock.
- Comando de test: `pnpm -C web test:run src/pages/Dashboard`.
- Resultado esperado: test verde; ruta navegable; layout no rompe en mobile.
- Work-unit commit: `feat(web): Dashboard page integration and route (#16)`

- [ ] **4.9** — Tests Vitest por componente (render contract)
- Acción: añadir tests de contrato: la página muestra todos los KPIs, el sparkline, la línea CTL/ATL/TSB, las barras HR y (condicionalmente) el drift. Sin tests visuales (estos quedan para QA manual).
- Comando de test: `pnpm -C web test:run --coverage src/features/dashboard src/pages/Dashboard`.
- Resultado esperado: cobertura del feature `dashboard` ≥ 75%.
- Work-unit commit: `test(web): dashboard render contract coverage (#16)`

- [ ] **4.10** — `pnpm typecheck/lint/format:check/test/build` verde
- Acción: ejecutar toolchain web completo; resolver warnings de ESLint y Prettier.
- Comando de test: `pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test:run && pnpm -C web build && pnpm -C web format:check`.
- Resultado esperado: los cinco exit 0.
- Work-unit commit: `chore(web): make typecheck/lint/test/build/format clean (#16)`

- [ ] **4.11** — PR merge
- Acción: `gh pr create --base main --head feat/phase-1.4-pr4-web-dashboard`; CI verde; squash & merge.
- Comando de test: `gh pr checks`; `gh pr view --json state`.
- Resultado esperado: PR mergeado; dashboard navegable en `main`.
- Work-unit commit: (merge commit).

---

## Forecast de carga por PR (LoC + verificación)

| PR | Branch | Budget LoC | Work units | Verificación clave |
|----|--------|-----------:|-----------:|--------------------|
| PR1 | `feat/phase-1.4-pr1-metrics-pure-go` | ~280 | 12 (1.0–1.12) | `go test -cover ./internal/metrics/...` ≥ 90% |
| PR2 | `feat/phase-1.4-pr2-timescaledb-recalc` | ~360 (split 2a/2b) | 13 (2.0–2.12) | `make db-test-migrations` + suite integration |
| PR3 | `feat/phase-1.4-pr3-api-dashboard` | ~350 | 11 (3.1–3.11) | `httptest` por endpoint; `/healthz` matrix |
| PR4 | `feat/phase-1.4-pr4-web-dashboard` | ~330 | 11 (4.1–4.11) | `pnpm test:run` + bundle size check |

**Total chain ~1320 LoC** distribuidos en 4 PRs stacked-to-main.

## Riesgo de review workload
- Por PR: < 400 LoC ✓ (cada uno respeta el budget).
- Por chain: ~1320 LoC en 4 PRs → review sostenible por PR, sin necesidad de size:exception.
- Cobertura `internal/metrics` ≥ 90% es criterion duro de merge en PR1.
- Contingencia activa: si PR2 > 400 LoC, split en PR2a (schema+queries) y PR2b (worker+hook) sin renegociar strategy (sigue stacked).

## Decision needed before apply

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low (por PR), Medium (PR2 puede requerir split de contingencia ya planificado)
Delivery strategy: ask-on-risk
```

> **Punto de decisión**: confirmar antes de ejecutar la cadena que el plan
> 4-PR stacked-to-main y la contingencia de split PR2a/PR2b son aceptables
> sin renegociar `delivery_strategy`. Si se prefiere `auto-chain`, el
> orquestador puede ejecutar PR1→PR2→PR3→PR4 sin pausa; si se prefiere
> pausa entre PRs, mantener `ask-on-risk`.

## Comandos de verificación por slice
- **PR1**: `GOTOOLCHAIN=local make fmt vet test lint` + `GOTOOLCHAIN=local go test -cover ./internal/metrics/...`.
- **PR2**: `GOTOOLCHAIN=local make db-test-migrations` + `GOTOOLCHAIN=local make fmt vet test lint`.
- **PR3**: `GOTOOLCHAIN=local make fmt vet test lint` + `GOTOOLCHAIN=local go test ./internal/http/... -v`.
- **PR4**: `pnpm -C web typecheck && pnpm -C web lint && pnpm -C web test:run && pnpm -C web build && pnpm -C web format:check`.

## Out of scope (confirmación)
- IA opcional (Fase 1.5).
- Planificación (V2).
- Laboratorios avanzados 1.6.
- `/healthz` con bearer token (rechazado en G3; header `X-Internal-Health: 1` es el contrato vigente).
- Cardiac Drift UI con fallback a `avg_hr` (rechazado en G5; sin streams → card oculta + mensaje).
- Renombrar `dashboard_metadata.last_recalc_at` o cambiar el shape del body de `/healthz` fuera de DA-005/DA-006.
- Internacionalización de textos de UI del dashboard (queda para Fase 2; copy en español provisional).
