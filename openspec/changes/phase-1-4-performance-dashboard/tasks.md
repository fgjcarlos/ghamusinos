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

- [ ] **P1** — Fijar TSSRunning canónica en proposal+spec+design (G-β)
- Acción: documentar la fórmula canónica en `internal/metrics/SPEC.md` (creado en PR1 task 1.0) y referenciarla en `specs/metrics-go/spec.md` (M-002) y `proposal.md`. Fórmula: `TSS_running = (durationSec / 3600) × (running_pace_threshold_sec_per_km / actual_pace_sec_per_km)^2 × 100`.
- Comando de test: scenario M-002 alineado en spec; no se ejecuta código aquí.
- Resultado esperado: spec M-002 contiene la fórmula exacta y un scenario "pace faster than threshold → IF > 1".
- Work-unit commit: `docs(phase-1-4): fix canonical TSSRunning formula and cross-references (#16)`

- [ ] **P2** — Fijar Cardiac Drift canónica (G-γ)
- Acción: documentar la fórmula canónica en `internal/metrics/SPEC.md` (PR1 task 1.0), `specs/metrics-go/spec.md` (M-006) y `proposal.md`. Fórmula: `drift_pct = (HR_end - HR_start) / HR_start × 100`. Edge: `HR_start == 0` → `drift_pct = 0` (no panic).
- Comando de test: sin código aún; validar que la spec describe el edge case.
- Resultado esperado: M-006 contiene la fórmula, un scenario nominal y un scenario `HR_start=0 → drift=0`.
- Work-unit commit: `docs(phase-1-4): canonicalize cardiac drift formula and HR_start=0 edge (#16)`

- [ ] **P3** — Resolver `/healthz` degradado (G-δ)
- Acción: alinear `proposal.md`, `specs/dashboard-api/spec.md` (DA-005, DA-006) y `design.md` (D4). Forma: si DB caída → HTTP 503 con `{status:"degraded", db:{ok:false}}` siempre, con o sin header `X-Internal-Health`. Forma sana con header: `{status:"ok", db:{ok:true}, strava:{...}, last_recalc_at, training_load_rows}`. Forma sana sin header: `{status:"ok"}` con 200.
- Comando de test: sin código; validar shape de spec DA-005/DA-006.
- Resultado esperado: ambos scenarios (con y sin header) recogen `db.ok=false` cuando DB caída.
- Work-unit commit: `docs(phase-1-4): align /healthz degraded semantics across proposal and specs (#16)`

- [ ] **P4** — Cardiac Drift UI: mensaje cuando panel oculto (G-ε)
- Acción: documentar en `specs/dashboard-web/spec.md` (DW-005) y `proposal.md` que si `CardiacDriftCard` se oculta del DOM, el `DashboardSummaryCard` o el wrapper muestra un texto "necesita streams HR".
- Comando de test: sin código; validar copy en spec.
- Resultado esperado: DW-005 incluye scenario "no HR streams → card oculta + texto informativo presente".
- Work-unit commit: `docs(phase-1-4): add cardiac-drift fallback message contract (#16)`

- [ ] **P5** — EMA sin warm-up (G-ζ)
- Acción: documentar en `specs/metrics-go/spec.md` (M-007) y `proposal.md` que la EMA arranca con cero hasta tener datos (`today_with_warmup = today + max(0, n_days-1) × 0`); los días previos a la primera actividad no influyen en el cálculo.
- Comando de test: sin código; validar descripción en spec.
- Resultado esperado: M-007 incluye scenario "primera actividad en día N → CTL en N se calcula solo con TSS de N".
- Work-unit commit: `docs(phase-1-4): document EMA cold-start with no warm-up (#16)`

---

## PR1 — `feat/phase-1.4-pr1-metrics-pure-go` (TDD)

> Slice puro-Go. Budget ~280 LoC. TDD posture RED→GREEN→REFACTOR por
> comportamiento. Cobertura `internal/metrics` ≥ 90% (criterion merge).
> Branch base: `main`. Merge strategy: squash al trunk de la cadena (stacked).

- [ ] **1.0** (pre) — Crear `internal/metrics/SPEC.md` con fórmulas y referencias literarias
- Acción: crear `internal/metrics/SPEC.md` referenciando M-001..M-010 y citando las fórmulas canónicas fijadas en P1, P2 y P5. Documentar unidades, bordes y orden de precondición de cada función pública.
- Comando de test: `git diff --stat internal/metrics/SPEC.md` (archivo nuevo, ~60 líneas).
- Resultado esperado: SPEC.md enlaza cada fórmula a su requirement (M-002 → TSSRunning, M-006 → CardiacDrift, M-007 → CTL/ATL/TSB).
- Work-unit commit: `docs(metrics): add SPEC.md anchoring canonical formulas (#16)`

- [ ] **1.1** — TSSCycling con tests golden
- Acción: implementar `internal/metrics/cycling.go` con `TSSCycling(durationSec, ftp, np)` y tests `cycling_test.go` con tres casos golden: 1h@FTP→100, 30min@FTP→50, duración 0→0. RED primero (tests fallan), luego GREEN.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run TSSCycling -v`.
- Resultado esperado: tres tests verdes; `go test -cover ./internal/metrics/...` ≥ 80% provisional.
- Work-unit commit: `feat(metrics): TSSCycling with golden tests (RED→GREEN) (#16)`

- [ ] **1.2** — TSSRunning (fórmula canónica de P1)
- Acción: implementar `internal/metrics/running.go` con `TSSRunning(durationSec, thresholdSecPerKm, actualPaceSecPerKm)` y tests: 1h@umbral→100, pace más rápido que umbral→TSS>100, pace más lento→TSS<100, duración 0→0. RED→GREEN.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run TSSRunning -v`.
- Resultado esperado: cuatro tests verdes; alineado con M-002 scenario "pace faster than threshold → IF > 1".
- Work-unit commit: `feat(metrics): TSSRunning canonical formula with tests (#16)`

- [ ] **1.3** — IntensityFactor
- Acción: `internal/metrics/if.go` con `IntensityFactor(np, ftp)` y `IntensityFactorRunning(velocity, thresholdVelocity)`. Tests: NP=FTP→1.0, NP=0.7*FTP→0.7, divide-by-zero→IF=0.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run IntensityFactor -v`.
- Resultado esperado: tests verdes; edge case divide-by-zero manejado sin panic.
- Work-unit commit: `feat(metrics): IntensityFactor for cycling and running (#16)`

- [ ] **1.4** — GradeAdjustedPace (Minetti-normalized)
- Acción: `internal/metrics/gap.go` con `GradeAdjustedPace(distanceM, elapsedSec, grade)` aplicando la curva de coste metabólico de Minetti. Tests: llano→pace original, subida 5%→pace equivalente más lento, llano 0%→sin cambio.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run GradeAdjusted -v`.
- Resultado esperado: tres tests verdes; alinea con M-004.
- Work-unit commit: `feat(metrics): GradeAdjustedPace Minetti-normalized (#16)`

- [ ] **1.5** — EfficiencyFactor
- Acción: `internal/metrics/ef.go` con `EfficiencyFactor(np, avgHr)`. Tests: NP=200W, HR=150→EF=1.333; HR=0→EF=0 (no panic).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run EfficiencyFactor -v`.
- Resultado esperado: dos tests verdes; alinea con M-005.
- Work-unit commit: `feat(metrics): EfficiencyFactor with HR=0 edge case (#16)`

- [ ] **1.6** — CardioDriftSeries (fórmula canónica de P2)
- Acción: `internal/metrics/cardiac_drift.go` con `CardioDriftSeries(hrStream []int, t, l, h int) (driftPct float64, err error)`. Aplica la fórmula P2: `(HR_end - HR_start) / HR_start × 100`. Edge: `HR_start == 0` → devuelve `0, nil` (no panic, no error). Tests: nominal (drift ~5%), HR_start=0, longitud de stream < h+l → error sentinela.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run CardioDrift -v`.
- Resultado esperado: tres tests verdes; alinea con M-006.
- Work-unit commit: `feat(metrics): CardioDriftSeries with HR_start=0 edge (#16)`

- [ ] **1.7** — CTL / ATL / TSB (TauCTL=42, TauATL=7)
- Acción: `internal/metrics/load.go` con `EBalance(dailyTSS []float64) (ctl, atl, tsb []float64)`. EMA exponencial con τ=42 y τ=7. Tests: TSS constante 100 durante 42 días → CTL≈63.21; TSS 0 → ATL→0 asintótico; TSB = CTL - ATL.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run EBalance -v`.
- Resultado esperado: tres tests verdes; valores numéricos validados con tolerancia 0.5.
- Work-unit commit: `feat(metrics): CTL/ATL/TSB EBalance with TauCTL=42, TauATL=7 (#16)`

- [ ] **1.8** — FillMissingDays (P5: sin warm-up)
- Acción: `internal/metrics/load.go` añade `FillMissingDays(from, to time.Time, daily map[string]float64) []Day`. Días sin entrada → 0. La serie resultante se pasa tal cual a EBalance; sin shift ni relleno previo. Tests: hueco de 5 días en medio → 5 ceros; rango vacío → serie vacía; primera actividad en día N → ceros hasta N-1.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run FillMissing -v`.
- Resultado esperado: tres tests verdes; alinea con M-007 (sin warm-up).
- Work-unit commit: `feat(metrics): FillMissingDays with no-warm-up contract (#16)`

- [ ] **1.9** — Cobertura `internal/metrics` ≥ 90% (criterion)
- Acción: añadir tests adicionales si la cobertura no llega; revisar ramas no cubiertas. Sin código nuevo más allá de tests.
- Comando de test: `GOTOOLCHAIN=local go test -cover ./internal/metrics/...` (debe mostrar `coverage: ≥ 90.0%`).
- Resultado esperado: línea de cobertura ≥ 90%; todas las funciones públicas tienen al menos un test.
- Work-unit commit: `test(metrics): raise coverage to ≥ 90% (#16)`

- [ ] **1.10** — `make fmt/vet/test/lint` verde
- Acción: ejecutar toolchain completo; resolver cualquier warning de `go vet` o `golangci-lint`. Sin código nuevo.
- Comando de test: `GOTOOLCHAIN=local make fmt && GOTOOLCHAIN=local make vet && GOTOOLCHAIN=local make test && GOTOOLCHAIN=local make lint`.
- Resultado esperado: los cuatro comandos exit 0; sin diffs residuales de `gofmt`.
- Work-unit commit: `chore(metrics): make fmt/vet/test/lint clean (#16)`

- [ ] **1.11** — Commit con conventional commit y push branch
- Acción: revisar historial; squash si hay commits de "wip"; push a `origin/feat/phase-1.4-pr1-metrics-pure-go`.
- Comando de test: `git log --oneline -10` (mensajes conventional), `git push -u origin feat/phase-1.4-pr1-metrics-pure-go`.
- Resultado esperado: branch remoto actualizado; todos los commits del PR1 forman un narrativa coherente (SPEC → TSSCycling → TSSRunning → IF → GAP → EF → Drift → EBalance → FillMissingDays → coverage → fmt/vet).
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
> `make db-test-migrations` local. Branch base: `main` (post-PR1).
> Riesgo E9: guard timescaledb en migrations.go.

- [ ] **2.0** (pre) — Verificar guard timescaledb en `internal/db/migrations.go` (E9)
- Acción: leer `internal/db/migrations.go` (sí, es código de proyecto, no codebase exploration); confirmar que la lógica de detección de la extensión timescaledb registra warning y no aborta si la extensión no está disponible. Si falta, añadir guard antes de aplicar 00012.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/db/... -run Migration -v`; inspección manual del guard.
- Resultado esperado: si la extensión no está, las migraciones se aplican igualmente (tablas planas) y se loggea un warning estructurado.
- Work-unit commit: `chore(db): harden timescaledb absence guard in migrations (#16)`

- [ ] **2.1** — Migración `00012_training_load_daily.sql`
- Acción: crear `internal/db/migrations/00012_training_load_daily.sql` con:
  - `CREATE TABLE training_load_daily (user_id BIGINT NOT NULL, day DATE NOT NULL, ctl DOUBLE PRECISION NOT NULL DEFAULT 0, atl DOUBLE PRECISION NOT NULL DEFAULT 0, tsb DOUBLE PRECISION NOT NULL DEFAULT 0, tss DOUBLE PRECISION NOT NULL DEFAULT 0, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (user_id, day));`
  - `SELECT create_hypertable('training_load_daily', 'day', chunk_time_interval => INTERVAL '7 days', if_not_exists => TRUE);` envuelto en `DO $$ BEGIN ... EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'timescaledb absent, training_load_daily stays plain'; END $$;`
  - `CREATE INDEX IF NOT EXISTS training_load_daily_user_day_idx ON training_load_daily (user_id, day DESC);`
  - `CREATE TABLE dashboard_metadata (user_id BIGINT PRIMARY KEY, last_recalc_at TIMESTAMPTZ, last_recalc_status TEXT, last_recalc_error TEXT);`
  - Sección `Down` simétrica: `DROP TABLE`, `DROP INDEX`, `DROP TABLE dashboard_metadata`.
- Comando de test: `GOTOOLCHAIN=local make db-test-migrations` (sube Postgres, aplica, verifica esquema, hace rollback, reaplica).
- Resultado esperado: `db-test-migrations` exit 0; sin warnings críticos; hypertable creada o aviso de timescaledb ausente registrado.
- Work-unit commit: `feat(db): migration 00012 training_load_daily + dashboard_metadata (#16)`

- [ ] **2.2** — Migración `00013_users_running_threshold.sql` (G-α, G1)
- Acción: crear `internal/db/migrations/00013_users_running_threshold.sql` con `ALTER TABLE users ADD COLUMN running_threshold_sec_per_km SMALLINT NULL; ALTER TABLE users ADD CONSTRAINT users_running_threshold_chk CHECK (running_threshold_sec_per_km IS NULL OR (running_threshold_sec_per_km BETWEEN 120 AND 1800));`. Down: `DROP CONSTRAINT`, `DROP COLUMN`.
- Comando de test: `GOTOOLCHAIN=local make db-test-migrations`; `psql ... -c "INSERT INTO users (..., running_threshold_sec_per_km) VALUES (..., 100);"` debe fallar con CHECK violation; valor 200 debe pasar.
- Resultado esperado: constraint 120..1800 activa; UP y DOWN reversibles.
- Work-unit commit: `feat(db): migration 00013 users.running_threshold_sec_per_km 120..1800 (#16)`

- [ ] **2.3** — `make generate` (sqlc); commit `internal/db/sqlc/*`
- Acción: actualizar `internal/db/queries.sql` o equivalente para que sqlc incluya las nuevas tablas; ejecutar `make generate`; revisar diff de `internal/db/sqlc/` y `internal/db/sqlc.json` (o el nombre vigente en el repo).
- Comando de test: `GOTOOLCHAIN=local make generate`; `git diff --stat internal/db/sqlc/`.
- Resultado esperado: métodos `UpsertTrainingLoadDaily`, `ListTrainingLoadRange`, `ListTrainingLoadFromFirstActivity`, `ListUserIDsForRecalc`, `UpsertDashboardMetadata`, `GetDashboardMetadata` disponibles.
- Work-unit commit: `feat(db): sqlc generated bindings for training_load + dashboard_metadata (#16)`

- [ ] **2.4** — `internal/db/queries/training_load.sql`
- Acción: crear queries sqlc: `UpsertTrainingLoadDaily`, `ListTrainingLoadRange (user_id, from, to)`, `ListTrainingLoadFromFirstActivity (user_id)`, `ListUserIDsForRecalc`.
- Comando de test: `GOTOOLCHAIN=local make generate` + `GOTOOLCHAIN=local go test ./internal/db/... -v`.
- Resultado esperado: queries compilan; tests de sqlc integration verdes.
- Work-unit commit: `feat(db): training_load queries (upsert/list/listFromFirst/listUsers) (#16)`

- [ ] **2.5** — `internal/db/queries/dashboard_metadata.sql`
- Acción: crear queries sqlc: `UpsertDashboardMetadata`, `GetDashboardMetadata (user_id)`.
- Comando de test: `GOTOOLCHAIN=local make generate`; test de round-trip insert+get.
- Resultado esperado: dos queries compiladas; round-trip verde.
- Work-unit commit: `feat(db): dashboard_metadata queries (upsert/get) (#16)`

- [ ] **2.6** — `internal/metrics/training_load.go` (ComputeDailyLoad — servicio)
- Acción: implementar `ComputeDailyLoad(ctx, userID, since time.Time) (map[time.Time]float64, error)` que agrega TSS por día (cycling+running) usando `internal/db/queries/activity.sql` existente. Es un orquestador, no función pura: depende de DB.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/metrics/... -run TestComputeDailyLoad -v` (con sqlc mock o testcontainers; el patrón de 1.3 indica preferencia por sqlc con DB de tests).
- Resultado esperado: función verde con fixture de 3 actividades distribuidas en 2 días; TSS agregado por día.
- Work-unit commit: `feat(metrics): ComputeDailyLoad service aggregator (#16)`

- [ ] **2.7** — `internal/jobs/recalc_training_load.go`
- Acción: worker River con `RecalcTrainingLoadArgs{UserID int64}`, lógica:
  1. Lee `dashboard_metadata` para `last_recalc_at`.
  2. Llama `ComputeDailyLoad` desde `last_recalc_at` (o desde la primera actividad si nulo).
  3. Llama `FillMissingDays` + `EBalance` (de PR1).
  4. Upsert en `training_load_daily`.
  5. Upsert `dashboard_metadata.last_recalc_at = NOW()` y status=`ok`.
  6. UniqueOpts por `user_id` (E7) para evitar jobs concurrentes.
  Tests: mock DB; job idempotente; segunda ejecución con mismo input no duplica filas.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/jobs/... -run RecalcTrainingLoad -v`.
- Resultado esperado: tests verdes; UniqueOpts configurado.
- Work-unit commit: `feat(jobs): RecalcTrainingLoad River worker with UniqueOpts (#16)`

- [ ] **2.8** — Hook post-UpsertActivity en `internal/strava/oauth_sqlc.go`
- Acción: en la función que persiste una actividad Strava, encolar `RecalcTrainingLoadArgs{UserID}` con River tras el upsert. Ventana de dedupe día ± 3: si en los últimos 3 días ya se encoló un recalc para este usuario, omitir (G-δ, E7).
- Comando de test: `GOTOOLCHAIN=local go test ./internal/strava/... -run TestUpsertActivityEnqueuesRecalc -v`.
- Resultado esperado: actividad nueva → job encolado; segunda actividad del mismo usuario en la misma ventana → segundo job omitido por UniqueOpts.
- Work-unit commit: `feat(strava): enqueue RecalcTrainingLoad on activity upsert (window ±3d) (#16)`

- [ ] **2.9** — Tests de idempotencia
- Acción: test integration que ejecuta `RecalcTrainingLoad` dos veces con el mismo set de actividades; verifica que `COUNT(*)` de `training_load_daily` no crece y los valores CTL/ATL/TSB son idénticos.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/jobs/... -run TestRecalcIdempotency -v` (con DB de tests).
- Resultado esperado: dos ejecuciones producen mismos rows; `last_recalc_at` se actualiza en ambas pero no genera drift.
- Work-unit commit: `test(jobs): recalc idempotency (#16)`

- [ ] **2.10** — Tests del worker (mock DB + job runner)
- Acción: tests unitarios con mock del `DBTX` que verifican la secuencia: leer metadata → ComputeDailyLoad → FillMissingDays+EBalance → upsert training_load_daily → upsert dashboard_metadata. Cubre el caso "no last_recalc_at" → usa primera actividad.
- Comando de test: `GOTOOLCHAIN=local go test ./internal/jobs/... -run TestWorker -v`.
- Resultado esperado: secuencia validada; mocks verifican llamadas esperadas.
- Work-unit commit: `test(jobs): worker sequence with mock DB (#16)`

- [ ] **2.11** — `make fmt/vet/test/lint` verde
- Acción: ejecutar toolchain completo; resolver warnings.
- Comando de test: `GOTOOLCHAIN=local make fmt && GOTOOLCHAIN=local make vet && GOTOOLCHAIN=local make test && GOTOOLCHAIN=local make lint`.
- Resultado esperado: los cuatro exit 0.
- Work-unit commit: `chore(db): make fmt/vet/test/lint clean (#16)`

- [ ] **2.12** — PR merge
- Acción: `gh pr create --base main --head feat/phase-1.4-pr2-timescaledb-recalc`; esperar CI verde (Postgres+timescaledb en CI); squash & merge.
- Comando de test: `gh pr checks` (todos en verde), `gh pr view --json state` (`MERGED`).
- Resultado esperado: PR mergeado; `main` contiene migraciones 00012 y 00013, sqlc bindings, worker y hook.
- Work-unit commit: (merge commit).

> **Contingencia split PR2a/PR2b**: si el diff de PR2 > 400 LoC, dividir en
> PR2a (`feat/phase-1.4-pr2a-schema-queries`: tasks 2.0–2.5) y PR2b
> (`feat/phase-1.4-pr2b-worker-hook`: tasks 2.6–2.12), ambos stacked
> sobre `main`. Si se activa, las tasks 2.6–2.12 se renumeran 2b.1–2b.7 y
> 2.5–2.11 siguen el patrón 1.10/1.12.

---

## PR3 — `feat/phase-1.4-pr3-api-dashboard` (handlers + /healthz)

> Budget ~350 LoC. TDD con `httptest` por endpoint. Branch base: `main`
> (post-PR2). Acepta header `X-Internal-Health: 1` para `/healthz` extendido.

- [ ] **3.1** — `internal/http/handlers/dashboard.go` (esqueleto)
- Acción: crear archivo con cuatro handlers `DashboardSummary`, `DashboardLoad`, `DashboardHRZones`, `DashboardRecalc`; firma y dependencias por constructor (DI). Sin lógica de negocio aún.
- Comando de test: `GOTOOLCHAIN=local go build ./internal/http/...` (compila sin lógica).
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
