# Fase 1.4 — Dashboard de rendimiento y salud/fatiga

> **Phase: explore.** Investigation of the gap between issue epic #16
> ("Fase 1.4 — Dashboard de rendimiento y salud/fatiga"), the
> architecture inventory (`docs/architecture/feature-inventory.md`
> §7–§8, §11), and the code as it ships today. Evidence base for
> the next proposal / spec / design / tasks chain.

## Issue y referencias

- **Issue epic**: #16 (OPEN, label `phase:1.4`).
- **Feature document**: `odd/tasks/phase-1-4-performance-dashboard.md`
  (decisiones ya tomadas por el usuario: PORT contra literatura
  canónica; chained 4-PR; budget 400; `/healthz` público; ventana
  post-upsert = día ± 3).
- **Feature inventory**: `docs/architecture/feature-inventory.md`
  - §7 — métricas (TSS, IF, GAP, EF, Cardiac Drift, CTL/ATL/TSB,
    recálculo, series temporales).
  - §8 — dashboard y visualización (lista/detalle actividades,
    mapa, **gráficas ECharts** CTL/ATL/TSB, distribución zonas FC,
    volumen/desnivel/actividades + tendencias).
  - §11 — healthcheck: `GET /healthz` simple ya en 1.1;
    health detallado (DB, Strava, Claude) en 1.4.
- **Roadmap**: `docs/roadmap/roadmap.md` §"Fase 1.4 — Dashboard de
  rendimiento y salud/fatiga" y §"Dependencias entre fases"
  ("1.4 depende de 1.2 — necesita actividades").
- **Regla de consolidación** (`feature-inventory.md:1-15`):
  el código TypeScript legacy pasa a ser **especificación de
  referencia, no se migra tal cual**. Las fórmulas se reimplementan
  en Go contra la **literatura canónica** (TrainingPeaks TSS,
  Running Balanced/PI, PMC de Coggan, Minetti GAP).

> ⚠️ **Regla A verificada**: el legacy `ghamusinos_/__`, `ghamusinos__`
> y `old_ghamusinos` **no existen en este repo** (`find ghamusinos_*`
> sobre `/home/composedof2/Dev/Codex/ghamusinos/` → 0 resultados;
> `ls` raíz solo lista `cmd, docker-compose.yml, docs, go.mod,
> go.sum, internal, Makefile, odd, openspec, README.md, scripts,
> sqlc.yaml, web`). Las fórmulas se portan **desde la
> documentación del inventario + literatura**, no desde código
> TS legacy. Esta es la corrección explícita del feature document
> (`odd/tasks/...md:18-20`).

## Estado actual del repo (verificado)

### Tablas y migraciones existentes

Migrations presentes (`internal/db/migrations/`, ordenadas):

| # | Archivo | Tablas / efecto | Notas para 1.4 |
|---|---|---|---|
| 00001 | `users_invites.sql` | `users`, `invites` | — |
| 00002 | `unique_email_indexes.sql` | Índices únicos | — |
| 00003 | `user_preferences.sql` | Añade `hr_max`, `lthr`, `ftp`, `level`, `timezone`, `ai_enabled` a `users` | **Fuente de hr_max/lthr/ftp** para TSS/IF/EF/GAP. `users_lthr_valido`/`users_ftp_valido`/`users_hr_max_valido` ya restringen rangos (`migrations/00003_user_preferences.sql:23-26`). |
| 00004 | `strava_activities.sql` | `strava_tokens`, `activities`, `activity_streams`, `activity_events`, `sync_sessions` | **Fuente primaria de actividades y streams**. Columnas relevantes para TSS: `avg_hr`, `max_hr`, `avg_power`, `elapsed_seconds`, `moving_seconds`, `distance_meters`, `elevation_gain_m`, `sport_type`, `started_at`. Ver `migrations/00004_strava_activities.sql:30-44`. `activity_streams.stream_type ∈ {heartrate, watts, cadence, altitude, latlng, …}` (`00004_strava_activities.sql:48-53`). |
| 00005 | `hr_zones.sql` | `hr_zones` | **Zonas FC por usuario**. Distribución de zonas para `/dashboard/hr-zones`. |
| 00006 | `gpx_tracks.sql` | `gpx_tracks`, `gpx_climbs`, `gpx_risk_zones` | Patrón de referencia para tablas propias (no JSONB) vs JSONB (ver `migrations/00006_gpx_tracks.sql:1-60`). |
| 00007 | `timescaledb_extension.sql` | `CREATE EXTENSION timescaledb` | **Requisito previo a hypertable** ya activo. Comentario explícito: "training_load_daily" mencionada como caso de uso futuro (`00007_timescaledb_extension.sql:7`). |
| 00008 | `activity_events_columns.sql` | Columnas adicionales en `activity_events` | — |
| 00009 | `activity_events_real_unique.sql` | UNIQUE real en `activity_events` | — |
| 00010 | `gpx_tracks_elevation_coverage.sql` | `elevation_coverage NUMERIC` en `gpx_tracks` | Patrón: añadir columna numérica con CHECK. |
| 00011 | `gpx_muros_recovery_kmvertical.sql` | `gpx_muros`, `gpx_recovery_zones`, `gpx_km_vertical` | **Patrón exacto para PR2** (3 tablas separadas, no JSONB). |

**Próxima migración libre**: `00012_training_load_daily.sql`
(PR2). Schema propuesto (ya en el feature doc):
`training_load_daily (user_id UUID, day DATE, tss NUMERIC,
activity_count INT, distance_m NUMERIC, elevation_gain_m NUMERIC,
computed_at TIMESTAMPTZ, PRIMARY KEY (user_id, day))` + hypertable
con `chunk_time_interval => INTERVAL '7 days'` + índice
`(user_id, day DESC)`.

### Paquetes Go existentes

```
internal/
├── app/            (server bootstrap, rutas, integration tests)
├── auth/           (JWT + middleware; patrón a reusar en handlers dashboard)
├── config/         (env vars + validación)
├── crypto/         (AES-GCM para secretos)
├── db/
│   ├── db.go       (pool)
│   ├── migrations.go (goose runner)
│   ├── migrations/  (ver tabla arriba)
│   ├── pool.go
│   ├── queries/    (13 archivos .sql; sqlc)
│   ├── schema.sql  (snapshot para tests)
│   ├── sqlc/       (código generado)
│   └── status/     (helpers)
├── frontend/       (embed.FS para SPA)
├── gpx/            (laboratorio 1.3 — entrega cerrada)
├── http/
│   ├── handlers/   (activities, gpx_*, health, readyz, me, strava, errors)
│   ├── router.go   (chi; Server struct con WithXxx() fluent chaining)
│   ├── middleware.go
│   └── security_middleware.go
├── jobs/
│   ├── river.go    (River client + adapters)
│   ├── workers.go  (AUD-04: deps inyectadas por constructor)
│   ├── backfill.go, streams.go, tokens.go
│   └── integration_test.go, webhook_integration_test.go
├── logging/        (slog)
└── strava/         (OAuth, webhook, ratelimit, types)
```

**`internal/metrics/` NO existe todavía** — debe crearse en PR1
con tres archivos: `performance.go`, `health.go`, `fatigue.go`
(según feature doc PR1 #2-#4). Paquete **puro, sin I/O**.

**Patrón River confirmado** (`internal/jobs/workers.go:31-125`):

```go
type Deps struct {
    Pool      *pgxpool.Pool
    Config    *config.Config
    Strava    *strava.Client     // opcional
    CipherKey []byte              // opcional
}
func NewRiverWorkers(d Deps, registerStravaWorkers bool) (*river.Workers, error)
// cada worker: river.WorkerDefaults[ArgsType] + NewXxxWorker(id importDeps)
```

Para PR2, `RecalcTrainingLoadArgs{UserID, From, To}` se registra
vía `river.AddWorker(workers, NewRecalcTrainingLoadWorker(id))`,
extendiendo `Deps` si necesita config adicional (p. ej.
`RecalcWindowDays`). Adaptador `RiverEnqueuerAdapter` ya existe
como patrón en `river.go:127-141`.

**Patrón de handlers confirmado** (`internal/http/handlers/activities.go:21-30`):

```go
func ListActivities(q sqlc.Querier) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := auth.AuthUser(r.Context())
        if user == nil { /* 401 */ }
        // ...
        WriteProblem(w, problem)
    })
}
```

Los handlers dashboard en PR3 deben seguir este patrón
(`func GetDashboardSummary(q sqlc.Querier) http.Handler`), usando
`auth.AuthUser` para gating y `WriteProblem` para errores.

**Patrón health actual** (`internal/http/handlers/health.go:10-17`):

```go
func Health(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

⚠️ **Acción PR3**: extender con `db`, `last_recalc_at`,
`training_load_rows`, **sin romper** la clave `status: "ok"` del
contrato público. Status code debe ser 200 si DB OK, 503 si DB
cae. Hay `readyz.go` separado (`/readyz`) con patrón de ping a
DB; PR3 podría compartir el helper de `internal/db/status/`.

### Frontend y stack de gráficas

**`web/package.json` dependencias actuales** (verificado):
- Runtime: `maplibre-gl`, `react`, `react-dom`, `react-router-dom`.
- Dev: vitest, @testing-library/react, eslint, prettier, tsc.
- ⚠️ **NO hay `echarts` ni `echarts-for-react`** — debe añadirse en
  PR4 (`pnpm add echarts` + tipo `@types/echarts` si procede).
  Es el gap más visible: el feature doc promete ECharts pero el
  stack aún no lo tiene. Mitigación: tree-shaking por módulo
  (`import * as echarts from 'echarts/core'` + registrar
  `LineChart`, `BarChart` y componentes necesarios).

**Rutas existentes** (`web/src/routes/`):
`activities.tsx`, `lab.tsx`, `lab-compare.tsx`, `profile.tsx`.
⚠️ **No existe `web/src/routes/dashboard.tsx`** — debe crearse en
PR4. App shell (`web/src/ui/AppShell/AppShell.tsx`) ya provee
`NavLink`; PR4 debe añadir un enlace `<NavLink to="/dashboard">`.

**Patrón features** (verificado en `web/src/features/`):
- `activities/` — `ActivityList/`, `ActivityRow/`,
  `HRZoneBars/` (container + presentacional).
- `gpx/` — `RouteClimbs/`, `RouteDetail/`, `RouteHeader/`,
  `RouteKmVertical/`, `RouteMuros/`, `RouteRecovery/`,
  `RouteRisks/`, `normalize.ts`.
- `lab/`, `profile/`, `strava/`.
- ⚠️ **No existe `web/src/features/dashboard/`** — debe crearse
  en PR4 siguiendo el patrón container + presentacional
  (`DashboardContainer.tsx` + `DashboardSummaryCard/`,
  `TrainingLoadChart/`, `HRZonesChart/`).

**Componentes UI reusables identificados** (`web/src/ui/`):
`AppShell` (con `NavLink`, `TopBar`, `SyncStatusChip`),
`Button`, `Chip`, `DifficultyBadge`, `EmptyState`, `Field`,
`MetricTile` (útil para los 4 KPIs del summary), `SeverityPill`.
Tokens `var(--gh-*)` ya usados — PR4 debe respetarlos (sin
colores hardcoded).

**Reuso probable**: `web/src/features/activities/HRZoneBars/` ya
existe y renderiza zonas FC por actividad individual. Para
`/dashboard/hr-zones` (agregado por periodo) PR4 necesitará un
nuevo componente que sume minutos por zona sobre el rango.

## Métricas: fuentes canónicas y casos golden

> **Decisión explícita** (`odd/tasks/...md:18-20`): "el legacy
> `ghamusinos_/__` no está presente en este repo; la 'portación'
> se hace desde las especificaciones documentadas en
> `feature-inventory.md` y desde las definiciones canónicas de
> la literatura". PR1 debe documentar cada fórmula en
> `internal/metrics/SPEC.md` con la fuente primaria y al menos
> un caso golden. Esta sección es el **esqueleto de esa SPEC.md**.

### TSS ciclismo

- **Fórmula canónica** (TrainingPeaks / Coggan 2003):
  `TSS = (sec × NP × IF) / (FTP × 3600) × 100`
  donde `IF = NP / FTP`.
- **Fuente primaria**: Hunter Allen & Andrew Coggan, *Training
  and Racing with a Power Meter* (2nd ed., 2012), cap. 7;
  Coggan, *The Science of Cycling*, 2003. Forma equivalente:
  `TSS = (work_kJ / (FTP_W × 3600)) × IF × 100` (sin NP).
- **Casos golden**:
  - 1 h a FTP (NP = FTP, IF = 1.0) → **TSS = 100**.
  - 2 h a 75 % NP (IF = 0.75) → TSS = (7200 × NP × 0.75) / (FTP × 3600) × 100 ≈ **150**.
  - 30 min a 50 % NP (IF = 0.50) → TSS = (1800 × 0.5 × 0.5) / 1 × 100 = **25**.
- **Edge**: duración 0 → 0; FTP ≤ 0 → sentinel defensivo (no
  panic, devolver 0 con `errors.New` o `pgtype.Numeric` zero).

### TSS running

- **Fórmula canónica** (TrainingPeaks Running TSS, balanced):
  `rTSS = (sec × NGP × IF) / (vFTP × 3600) × 100`
  donde:
  - `vFTP` = "velocity at FTP" en m/s (FTP running en m/s, no W).
  - `NGP` = "Normalized Graded Pace" (ajustada por pendiente,
    análogo a NP en ciclismo). Strava ya calcula `grade_adjusted
    _distance` / `grade_adjusted_moving_time` — fuente ideal.
  - `IF = NGP / vFTP`.
- **Fuente primaria**: Daniels & Gilbert, *Daniels' Running
  Formula* (cap. 4-5); TrainingPeaks, *"How to Use Running TSS"*
  (Coach Joe Friel). Forma equivalente: `rTSS = (sec × IF²) ×
  (vFTP / vActual_equiv)` que requiere vActual equivalente.
- **Casos golden**:
  - 1 h a vFTP (NGP = vFTP, IF = 1.0) → **rTSS = 100**.
  - Umbral pace típico 4:00 min/km → vFTP = 1000/240 = 4.167 m/s.
  - Carrera de 50 min a ritmo 4:30 min/km (NGP ≈ 3.704 m/s) →
    IF ≈ 0.889 → rTSS ≈ (3000 × 0.889 × 0.889) / (4.167 × 3600)
    × 100 ≈ **79**.
- **Edge**: pace ≤ 0 (inválido), `thresholdPaceSecPerKm <= 0` →
  sentinel 0 con error.

### IF

- **Fórmula canónica**:
  `IF = NP / FTP` (ciclismo) o `IF = NGP / vFTP` (running).
- **Casos golden**: NP = FTP → 1.0; NP = 0.75 × FTP → 0.75;
  NP = 1.10 × FTP → 1.10. Rango típico [0.5, 1.2].
- **Edge**: FTP = 0 → sentinel 0.

### GAP

- **Fórmula canónica**: **Minetti et al. (2002)** *"Energy cost
  of walking and running at extreme uphill and downhill slopes"*
  (J. Appl. Physiol. 93: 1039-1046) provee la curva
  C(grade) = 155.4·grade⁵ − 30.4·grade⁴ − 43.3·grade³ +
  46.3·grade² + 19.5·grade + 3.6 (J · kg⁻¹ · m⁻¹). Strava usa
  una versión normalizada: GAP_min/km = pace · (C(grade) /
  C(0)). Strava expone `grade_adjusted_moving_time` (segundos)
  y `grade_adjusted_distance` (metros).
- **Decisión PR1**: implementar `GradeAdjustedPace(grade float64,
  paceSecPerKm float64) float64` con la **curva de Minetti
  normalizada** (la misma que Strava usa desde 2018). Si la
  issue pide reconciliar con `old_ghamusinos`, ese legacy no
  está en el repo; usar la referencia como spec.
- **Casos golden**:
  - grade = 0 → multiplicador 1.0 → GAP = pace (sin cambio).
  - grade = +10 % → multiplicador ≈ 2.0 → GAP = 2 × pace
    (más lento en subida). Con pace 6:00 min/km → GAP ≈
    12:00 min/km.
  - grade = −10 % → multiplicador ≈ 0.45 → GAP = 0.45 × pace
    (más rápido en bajada). Con pace 6:00 min/km → GAP ≈
    2:42 min/km.
- **Edge**: pace ≤ 0 → sentinel 0; grade fuera de [-35%, +35%]
  (rango fisiológico razonable) → clampear o devolver error.

### Efficiency Factor

- **Fórmula canónica**: `EF = NP / avgHR` (ciclismo).
- **Fuente**: Coggan, *Training and Racing with a Power Meter*
  (2nd ed., 2012). EF sube con el entrenamiento aeróbico
  (más potencia con menos FC). Rango típico [0.8, 2.0].
- **Casos golden**: NP = 200 W, avgHR = 150 → EF = 1.33.
- **Edge**: avgHR ≤ 0 → sentinel 0 (avg_hr puede ser NULL en
  `activities`, ver `migrations/00004_strava_activities.sql:38-39`).

### Cardiac Drift

- **Fórmula canónica**:
  `Drift = ((HR_end − HR_start) / HR_start) × 100`.
  En sesión de steady state: se calcula en los últimos 20-30 min
  respecto al primer tramo a misma intensidad. Strava expone
  `average_heartrate` y `max_heartrate` por actividad, pero **no
  expone HR por tercio**. La opción práctica con streams es
  partir el array `heartrate` en dos mitades y comparar
  medias. **Sin streams**, fallback con `avg_hr` y duración
  (aproximación pobre — marcar como tal).
- **Decisión PR1**: implementar dos funciones:
  `CardiacDrift(hrStart, hrEnd float64) float64` (puntual) +
  `CardiacDriftSeries([]int) float64` que toma primer y tercer
  cuartil (más conservador que 1ª vs 2ª mitad, evita sesgo
  por warm-up incompleto).
- **Casos golden**:
  - Sin drift: 140 → 145 → 3.57 %.
  - Drift alto (deshidratación): 140 → 165 → 17.86 %.
- **Edge**: hrStart ≤ 0 → sentinel 0; series vacías → 0.

### CTL / ATL / TSB (PMC)

- **Fórmula canónica** (Coggan PMC, implementado por TrainingPeaks):
  - **CTL** (chronic training load, "fitness") — EMA exponencial
    de TSS con **τ = 42 días** (constante de tiempo).
    `CTL_today = CTL_yesterday + (TSS_today − CTL_yesterday) ×
    (1 − exp(−1/42))`.
  - **ATL** (acute training load, "fatigue") — EMA con **τ = 7 días**.
    `ATL_today = ATL_yesterday + (TSS_today − ATL_yesterday) ×
    (1 − exp(−1/7))`.
  - **TSB** (training stress balance, "form") — `TSB = CTL − ATL`.
  - **Relleno de días vacíos**: TSS = 0 en días sin actividad
    (no NaN; la EMA sigue evolucionando con entrada 0).
  - **Inicio**: CTL₀ = ATL₀ = 0 (estado de "no atleta"); o
    alternativa `CTL₀ = TSS_primer_día` (estado de "primer día
    es baseline"). TrainingPeaks usa la primera.
- **Fuente primaria**: Coggan, *"Power-Based Training, Part 1: The
  Performance Manager"* (2003, foros Slowtwitch);
  TrainingPeaks, *"Understanding PMC"* (Coach Joe Friel).
- **Casos golden**:
  - TSS constante 100/día, suficiente para estabilizar:
    CTL → ~100; ATL → ~100; TSB → 0.
  - TSS = 0 durante 7 días tras CTL=100: ATL baja rápido
    (`1 − exp(−1/7) ≈ 0.133`), CTL baja lento (`1 − exp(−1/42)
    ≈ 0.0236`). A los 7 días: ATL ≈ 41.7, CTL ≈ 84.4, TSB ≈
    +42.7.
  - Día con TSS = 0 debe **mantener** la serie (no saltarse);
    `FillMissingDays(from, to, raw)` debe devolver exactamente
    `(to − from)/24h + 1` puntos.
- **Edge**:
  - Series vacías → CTL/ATL = 0.
  - Un único punto → CTL = ATL = TSS_único, TSB = 0.
  - tz del usuario: trabajar en día local (campo `timezone` en
    `users`, `migrations/00003_user_preferences.sql:10`) o UTC;
    **decisión PR1**: documentar y elegir uno. Recomendado:
    UTC en almacenamiento; conversión a día local al agrupar
    en PR2 (recalc job).

## Plan de 4 PRs (validado)

### PR1 — Métricas puras (TDD)

**Branch**: `feat/phase-1.4-pr1-metrics-pure-go`. **Budget**: ≤ 400 LoC.

- Crear `internal/metrics/{performance,health,fatigue}.go` con
  funciones puras documentadas contra `internal/metrics/SPEC.md`.
- `TSSCycling`, `TSSRunning`, `IntensityFactor`,
  `GradeAdjustedPace`, `EfficiencyFactor`,
  `CardiacDrift`(+`CardiacDriftSeries`),
  `CTL`, `ATL`, `TSB`, `FillMissingDays`, `DailyLoad` struct.
- Tests tabla-driven por métrica: (a) golden canónico,
  (b) borde (input 0 / negativo → sentinel), (c) caso
  documentado.
- Cobertura ≥ 90 % líneas en `internal/metrics/`.
- **Cero I/O, cero SQL, cero HTTP**. Paquete importable
  trivialmente por PR2 sin ciclos.
- **Subdivisión condicional** (feature doc PR1 nota): si el
  bloque CTL/ATL/TSB + `FillMissingDays` pasa de 250 LoC,
  dividir en **PR1a** (TSS/IF/GAP/EF/cardiac drift) y
  **PR1b** (CTL/ATL/TSB). Sin llegar a 4 PRs por esto, el
  chained sigue siendo de 5 efectivos — mantener 4 por ahora.
- **Tareas secundarias**: `internal/metrics/SPEC.md` con cada
  fórmula y fuente; alinear constantes `TauCTL = 42`,
  `TauATL = 7` como `const` exportadas para que PR2 las reuse.

### PR2 — Persistencia TimescaleDB + recálculo

**Branch**: `feat/phase-1.4-pr2-timescaledb-recalc`. **Budget**: ≤ 400 LoC.

- Migración `internal/db/migrations/00012_training_load_daily.sql`:
  tabla + `create_hypertable` + índice `(user_id, day DESC)`
  + `goose Down` simétrico.
- SQLC queries (`internal/db/queries/training_load.sql`):
  `UpsertDailyLoad`, `ListDailyLoadByUserRange`,
  `ListDailyLoadFromFirstActivity`, `ListUserIdsWithActivities`.
- `internal/metrics/training_load.go` (capa de servicio, **no
  pura** — importa SQLC + contexto): `ComputeDailyLoad(ctx,
  pool, userID, from, to)`. Itera actividades en rango, agrupa
  por día (zona horaria del usuario), calcula TSS por actividad
  usando las funciones de PR1, devuelve filas para upsert.
  **Idempotente**.
- `internal/jobs/recalc_training_load.go`:
  `RecalcTrainingLoadArgs{UserID, From, To}` + worker River.
  Default: `From = ListDailyLoadFromFirstActivity(userID)` o
  primera `activities.started_at`; `To = hoy`. Extender
  `Deps` en `workers.go` solo si se necesita config nueva.
- **Hook post-`UpsertActivity`**: en `internal/jobs/workers.go`
  o en el `ActivityInserter` adapter, encolar
  `RecalcTrainingLoadArgs{UserID, From: started_at − 3 días,
  To: started_at + 3 días}` después de upsert exitoso.
- Tests:
  - Migración forward/backward con timescaledb.
  - `TestComputeDailyLoadGroupsByDay` (sintético).
  - `TestUpsertDailyLoadIdempotent` (segunda ejecución no
    duplica filas).
  - Mock del job runner (no necesita River corriendo).
- ⚠️ **Cuestión abierta para sdd-spec**: dónde se persiste
  `last_recalc_at` (necesario para `/healthz`). Opciones:
  (a) columna `last_recalc_at` en una tabla de metadatos
  nueva `dashboard_metadata (user_id PK, last_recalc_at,
  last_recalc_status)`; (b) `MAX(computed_at) FROM
  training_load_daily WHERE user_id = $1` por consulta;
  (c) columna global en `users` (mezcla concerns — no
  recomendado). **Recomendación**: (a), fila única por usuario,
  upsert desde el worker.

### PR3 — API dashboard + health detallado

**Branch**: `feat/phase-1.4-pr3-api-dashboard`. **Budget**: ≤ 400 LoC.

- `internal/http/handlers/dashboard.go` con 3 handlers factory:
  - `GetDashboardSummary(q sqlc.Querier, pool *pgxpool.Pool)
    http.Handler`: respuesta
    `{ weekly_volume_m, weekly_elevation_m,
    weekly_activities_count, trend_7d_pct, trend_30d_pct }`.
    Cálculo desde `training_load_daily` (7 días) +
    `activities` (volumen agregado).
  - `GetTrainingLoad(q sqlc.Querier) http.Handler`: query
    params `from`, `to`; devuelve `[]DailyLoad{day, ctl, atl,
    tsb}`. Si `from` anterior a primera actividad, rellenar
    con 0 desde la primera actividad hasta `from` (UX).
  - `GetHRZones(q sqlc.Querier) http.Handler`: query params
    `from`, `to`; agrega minutos por zona desde
    `activity_streams WHERE stream_type = 'heartrate'` cuando
    exista, **fallback** a `activities.avg_hr ×
    elapsed_seconds` repartido uniforme entre zonas (peor
    estimación — marcar en godoc).
- `internal/http/handlers/health.go`: extender con secciones:
  ```
  {
    "status": "ok",
    "db": { "ok": true, "latency_ms": 12 },
    "last_recalc_at": "2026-09-30T08:00:00Z",
    "training_load_rows": 1342
  }
  ```
  Mantener `status: "ok"` siempre presente (compatibilidad).
  Status HTTP 200 si DB OK, 503 si DB caída (helper existente
  en `internal/db/status/`).
- Opcional `POST /api/v1/dashboard/recalc` para forzar recálculo
  completo (encolar job por usuario con `From = nil` →
  "completo"). El feature doc lo menciona en riesgos
  (`odd/tasks/...md:115`); si no entra en 400 LoC, **deferirlo
  a PR3.5 o parte de PR4**.
- Montaje en `internal/http/router.go`:
  `r.Route("/api/v1/dashboard", func(r chi.Router) { ... })`
  con auth (mismo patrón que `/api/v1/gpx/*`,
  `internal/http/router.go:194-201`).
- Tests: tabla-driven por endpoint con `httptest`, store
  mockeado (extender la interfaz mínima), auth middleware
  aplicado.
- OpenAPI / godoc actualizado (comentarios en handlers).

### PR4 — Web dashboard (ECharts)

**Branch**: `feat/phase-1.4-pr4-web-dashboard`. **Budget**: ≤ 400 LoC.

- ⚠️ **Acción previa**: `pnpm add echarts` y registrar tipo si
  aplica (decisión tree-shaking: importar `echarts/core` +
  `LineChart`, `BarChart`, `GridComponent`, `TooltipComponent`,
  `LegendComponent`, `TitleComponent`,
  `DataZoomComponent`).
- `web/src/lib/api/dashboard.ts`: tipos espejo de handlers
  PR3 + funciones `getSummary`, `getLoad`, `getHRZones`
  (mismo patrón que `web/src/lib/api/gpx.ts:95-115`).
- Hooks: `useDashboardSummary`, `useTrainingLoad(from, to)`,
  `useHRZones(from, to)` (mismo patrón que
  `RouteDetailContainer.tsx:1-50`).
- Componentes (patrón container + presentacional como
  `web/src/features/activities/`):
  - `DashboardSummaryCard/` (4 KPI tiles con sparkline,
    reutilizando `MetricTile` de `web/src/ui/MetricTile/`).
  - `TrainingLoadChart/` (ECharts línea CTL/ATL/TSB,
    selector de periodo 7d/30d/90d/1y).
  - `HRZonesChart/` (ECharts barras apiladas por actividad o
    acumulado por periodo; **diferenciar** de
    `web/src/features/activities/HRZoneBars/` que es por
    actividad individual).
- Página `web/src/routes/dashboard.tsx`: layout responsive,
  navegación desde `AppShell` (añadir `<NavLink to="/dashboard">`).
- i18n: tokens `var(--gh-*)`, sin colores hardcoded.
- Empty state: usuario sin actividades → mensaje amigable
  (reutilizar `web/src/ui/EmptyState/`).
- Tests: render contract por componente + integración de
  hooks con msw o mock fetcher existente (vitest ya en
  `package.json:14`).

## Riesgos identificados

### Riesgos NUEVOS (no listados en el feature doc)

- **E1. ECharts no instalado en `web/package.json`.** El
  feature doc asume ECharts como elección pero el stack aún
  no lo tiene. **PR4 debe ejecutar `pnpm add echarts` antes
  de cualquier import**. Mitigación: documentar este paso
  en PR4 tasks; bloquear merge de PR4 sin dependencia
  declarada. Probable impacto en bundle size (ECharts core
  ~400 KB) — mitigación con tree-shaking por módulo
  (importar `echarts/core` + registrar `LineChart`,
  `BarChart` y componentes necesarios).
- **E2. `last_recalc_at` no tiene lugar natural en el
  schema.** El feature doc lo lista como sección del
  `/healthz` pero no hay tabla ni columna donde persistirlo.
  **PR2 debe decidir dónde** (recomendado: tabla nueva
  `dashboard_metadata (user_id PK, last_recalc_at,
  last_recalc_status, training_load_rows)`). Si se difiere
  a PR3, `/healthz` no puede reportarlo.
- **E3. Cardiac Drift con streams vs sin streams.** La
  fórmula canónica (1ª mitad vs 2ª mitad de HR en sesión
  steady-state) requiere `activity_streams`. La tabla
  `activities` solo guarda `avg_hr`. **Decisión PR1**:
  `CardiacDriftSeries([]int)` solo funciona si el caller
  le pasa los HR samples (viene de streams); fallback con
  `avg_hr` no es "drift" real sino ruido. Documentar
  limitación en godoc y posponer render del drift a un
  follow-up cuando todos los actividades tengan streams.
- **E4. TSS running depende de vFTP (m/s) o threshold pace
  (seg/km) que no están en `users`.** El campo
  `users.ftp SMALLINT` (migrations/00003:8) es **watts
  ciclismo**, no running threshold. El rango válido de
  `users.running_threshold_sec_per_km` es 120..1800 s/km.
  **PR1 debe aceptar
  threshold pace como parámetro explícito** (vía
  `user_preferences` extendida o `users.running_threshold
  _sec_per_km` en migración nueva). Esto **rompe PR1 ↔ PR2**
  ligeramente: PR2 necesitará leer este campo. Propuesta:
  añadir columna `users.running_threshold_sec_per_km
  SMALLINT NULL` en una nueva migración **00013** (puede
  ser parte de PR2 si encaja en budget; si no, separar).
- **E5. TZ del usuario para `FillMissingDays`.** El campo
  `users.timezone` existe (`migrations/00003_user_preferences.sql:10`)
  pero PR1 `FillMissingDays(from, to, raw)` trabaja con
  `time.Time` UTC. **Decisión**: PR1 trabaja en UTC; PR2
  convierte `started_at` (TIMESTAMPTZ) a día local del
  usuario antes de agrupar. Documentar en godoc de
  `FillMissingDays` que la granularidad es día UTC y que la
  traslación a TZ local ocurre aguas arriba en PR2.
- **E6. GAP: curva de Minetti vs referencia Strava.**
  Minetti (2002) es la curva académica; Strava usa una
  variante normalizada. El feature doc dice "GAP
  (referencia combinada con `old_ghamusinos`)" pero ese
  legacy no está en el repo. **PR1 debe elegir una curva y
  documentarla en `SPEC.md`**. Recomendado: Minetti
  normalizada (la de Strava) por compatibilidad con datos
  de origen.
- **E7. Hook post-`UpsertActivity` con ± 3 días.** El job
  River se inserta tras upsert; con actividad en `started_at`
  hace 90 días, el job recalcula 90±3. **Si llegan 50
  actividades en 1 min** (sync masivo), 50 jobs recalculan
  rangos solapados → contención en `training_load_daily`.
  Mitigación: deduplicar jobs en River con `UniqueOpts`
  (river v0.10+) o coalescer con un job "consolidado" por
  usuario con TTL de 30 s. **Decisión PR2**.
- **E8. Estado público de `/healthz` con secciones
  nuevas.** El handler actual (`internal/http/handlers/health.go`)
  es público y minimal. La forma extendida con `db`,
  `last_recalc_at`, `training_load_rows` añade **información
  operativa que puede no ser deseable exponer
  públicamente** (DoS intel: número de filas en training_load
  ≈ tamaño del historial del sistema). **Decisión PR3**:
  mantener el endpoint público **pero** limitar la versión
  detallada a una sección **opcional** detrás de un header
  (p. ej. `X-Internal-Health: 1` o token bearer
  compartido). El feature doc no menciona esto — **gap**.
- **E9. `00012_training_load_daily.sql` necesita timescaledb
  activo.** Si la suite de tests corre sin timescaledb
  (p. ej. CI en entorno mínimo), la migración falla. Hay
  precedente: `00007_timescaledb_extension.sql:7` ya advierte
  del problema. **PR2 debe** marcar esta migración con un
  guard o moverla a un subdirectorio `migrations_timescale/`
  (ver patrón usado por la comunidad Goose para esto).
  Verificar en `internal/db/migrations.go` y CI actual si
  hay guard antes de empezar PR2.

### Riesgos del feature doc confirmados

- **GAP**: ya resuelto por regla A (legacy no presente → fórmulas
  desde literatura).
- **Series + backfill**: ventana post-upsert = 3 días (PR2
  mitigación principal); recálculo completo solo bajo
  `POST /dashboard/recalc` (si entra en PR3).
- **Frontend ECharts bundle size**: ver E1.
- **`/healthz` público**: ver E8 (gap nuevo en granularidad).

## Gaps issue ↔ plan

> Cotejo entre la issue #16 (resumida en feature document ODD) y
> el plan de 4 PRs propuesto. ✅ = cubierto, ⚠️ = parcial / a
> confirmar, ❌ = faltante.

| Entregable ODD | PR | Cobertura | Notas |
|---|---|---|---|
| TSS ciclismo | PR1 | ✅ | Función pura, tests golden (1h@FTP → 100). |
| TSS running | PR1 | ✅ (con caveat E4) | Necesita `running_threshold_sec_per_km`; columna nueva pospuesta a 00013. |
| IF | PR1 | ✅ | — |
| GAP | PR1 | ✅ (con caveat E6) | Decisión de curva documentada en SPEC.md. |
| Efficiency Factor | PR1 | ✅ | — |
| Cardiac Drift | PR1 | ✅ (con caveat E3) | Solo útil con streams HR; sin streams es aproximación. |
| CTL/ATL/TSB + relleno | PR1 | ✅ | τ = 42/7 exportados como `const`. |
| Tabla `training_load_daily` | PR2 | ✅ | Hypertable + chunk 7 días + índice `(user_id, day DESC)`. |
| Job River de recálculo | PR2 | ✅ | `RecalcTrainingLoadArgs{UserID, From, To}`. |
| Hook post-`UpsertActivity` ± 3 días | PR2 | ✅ (con caveat E7) | Deduplicación de jobs por decidir. |
| `/api/v1/dashboard/summary` | PR3 | ✅ | Volumen, desnivel, count, trends 7d/30d. |
| `/api/v1/dashboard/load` | PR3 | ✅ | Serie CTL/ATL/TSB con relleno. |
| `/api/v1/dashboard/hr-zones` | PR3 | ✅ | Streams HR con fallback a `avg_hr × elapsed`. |
| `POST /api/v1/dashboard/recalc` | PR3 | ⚠️ | Mencionado en feature doc riesgos pero no en tasks PR3 explícitos; verificar en `sdd-tasks` si entra en 400 LoC. Si no, deferir. |
| `/healthz` extendido (db, last_recalc_at, training_load_rows) | PR3 | ✅ (con caveat E8) | Decisión de exposición pública/privada a tomar. |
| ECharts (línea CTL/ATL/TSB) | PR4 | ✅ (con caveat E1) | `pnpm add echarts` antes de cualquier import. |
| Barras apiladas zonas FC | PR4 | ✅ | Componente nuevo en `web/src/features/dashboard/HRZonesChart/`. |
| Sparkline semanal | PR4 | ✅ | En `DashboardSummaryCard/`, reusando `MetricTile`. |
| Página `/dashboard` | PR4 | ✅ | Ruta + enlace en `AppShell`. |
| Patrón container + presentacional | PR4 | ✅ | Como `web/src/features/activities/`. |
| Empty state sin actividades | PR4 | ✅ | Reusar `web/src/ui/EmptyState/`. |
| Cobertura `internal/metrics` ≥ 90 % | PR1 | ✅ | Acceptance criterion explícito. |
| `make fmt vet test` verde | PR1-PR3 | ✅ | CI ya lo exige (ver `Makefile`). |
| `pnpm typecheck/lint/test` verde | PR4 | ✅ | Scripts ya en `package.json:8-15`. |

**Gaps abiertos** (no resueltos por el plan; requieren
confirmación del usuario en `sdd-proposal`):

- **G1. `users.running_threshold_sec_per_km`**: ¿se añade en
  00013 dentro de PR2 o se difiere? Sin este campo, TSS
  running queda inutilizable en PR3 incluso con actividades
  reales.
- **G2. `dashboard_metadata` table para `last_recalc_at`**:
  PR2 lo crea o PR3 lo improvisa. Recomendación: PR2.
- **G3. `/healthz` extendido — ¿público o detrás de token?**
  El feature doc dice "mantener público, no añadir auth", pero
  exponer `training_load_rows` cuenta usuarios. Decisión de
  producto.
- **G4. `POST /api/v1/dashboard/recalc`**: scope de PR3 o
  follow-up. Decisión en `sdd-tasks`.
- **G5. Cardiac Drift UI**: si solo funciona con streams, ¿el
  dashboard muestra el drift o lo difiere a 1.4.1? Decisión
  PR4.

## Referencias (archivo:línea)

> Formato: `ruta/relativa/al/repo:linea` o
> `ruta:linea-linea`. Todos los archivos son del repo
> `/home/composedof2/Dev/Codex/ghamusinos`.

### Estado actual del repo

- `docs/architecture/feature-inventory.md:1-15` — §1 origen de
  las 4 bases legacy + regla de "spec, no migración literal".
- `docs/architecture/feature-inventory.md:142-145` — §7
  métricas de rendimiento y carga/fatiga (tabla 1.4).
- `docs/architecture/feature-inventory.md:147-156` — §8
  dashboard y visualización.
- `docs/architecture/feature-inventory.md:158-160` — §8
  decisión de stack: **MapLibre + ECharts** (no Leaflet, no
  Recharts).
- `docs/architecture/feature-inventory.md:172-174` — §11
  healthcheck: `/healthz` simple en 1.1; detallado en 1.4.
- `docs/roadmap/roadmap.md:51-54` — Fase 1.4 entregables +
  criterio de cierre.
- `docs/roadmap/roadmap.md:81` — dependencias: "1.4 depende
  de 1.2 — necesita actividades".
- `internal/db/migrations/00003_user_preferences.sql:8-15` —
  `users.hr_max`, `users.lthr`, `users.ftp`, `users.level`,
  `users.timezone`, `users.ai_enabled` + checks.
- `internal/db/migrations/00004_strava_activities.sql:30-44` —
  tabla `activities` con todas las columnas necesarias.
- `internal/db/migrations/00004_strava_activities.sql:48-53` —
  tabla `activity_streams` con `stream_type` JSONB.
- `internal/db/migrations/00005_hr_zones.sql` — tabla
  `hr_zones` (no leída en este turno; existencia confirmada).
- `internal/db/migrations/00007_timescaledb_extension.sql:7` —
  comentario explícito: "training_load_daily" como caso de
  uso futuro.
- `internal/db/migrations/00011_gpx_muros_recovery_kmvertical.sql`
  — patrón a replicar en PR2 (3 tablas separadas).
- `internal/jobs/workers.go:31-56` — `Deps` struct (Pool,
  Config, Strava, CipherKey).
- `internal/jobs/workers.go:99-125` — `NewRiverWorkers`
  registro de workers Strava (patrón PR2).
- `internal/jobs/workers.go:131-181` — `ImportStravaWorker`
  completo con sync session, sync `defer-to-failed`,
  paginación.
- `internal/jobs/river.go:127-141` — `RiverEnqueuerAdapter`
  patrón para encolar jobs desde handlers.
- `internal/http/handlers/health.go:10-17` — `/healthz`
  handler actual (público, minimal).
- `internal/http/handlers/activities.go:21-30` — patrón
  handler factory con `sqlc.Querier` + `auth.AuthUser` +
  `WriteProblem`.
- `internal/http/router.go:18-58` — `Server` struct con
  `WithXxx()` fluent chaining.
- `web/package.json:30-34` — dependencias runtime: SOLO
  maplibre-gl, react, react-dom, react-router-dom. **NO
  echarts**.
- `web/package.json:8-15` — scripts: typecheck, lint, format,
  test, build.
- `web/src/features/activities/ActivityList/ActivityList.tsx`
  — patrón container.
- `web/src/features/activities/HRZoneBars/HRZoneBars.tsx` —
  existente para zonas FC por actividad individual.
- `web/src/ui/MetricTile/` — reutilizable para los 4 KPIs del
  dashboard summary.

### Feature document ODD

- `odd/tasks/phase-1-4-performance-dashboard.md:18-20` —
  corrección legacy no presente, fórmulas desde literatura.
- `odd/tasks/phase-1-4-performance-dashboard.md:60-82` — PR1
  tareas (métricas puras, TDD).
- `odd/tasks/phase-1-4-performance-dashboard.md:84-103` — PR2
  tareas (TimescaleDB + recálculo).
- `odd/tasks/phase-1-4-performance-dashboard.md:105-120` — PR3
  tareas (API + health).
- `odd/tasks/phase-1-4-performance-dashboard.md:122-137` — PR4
  tareas (web dashboard).
- `odd/tasks/phase-1-4-performance-dashboard.md:139-148` —
  riesgos identificados (GAP, series+backfill, ECharts,
  health público).
- `odd/tasks/phase-1-4-performance-dashboard.md:150-159` —
  rollback por PR.

## Resumen para el orquestador

**(a) Confirmación de coherencia**: el plan de 4 PRs (PR1
métricas puras, PR2 TimescaleDB + River recalc, PR3 API +
`/healthz`, PR4 ECharts dashboard) **encaja en la arquitectura
existente** del repo: extensión natural de `internal/jobs/`
(Workers + Deps ya preparados), del patrón de handlers
(`sqlc.Querier` + `auth.AuthUser`), del router chi con
`WithXxx()` fluent, y del patrón features en `web/src/`. La
migración 00012 sigue el precedente de `00011_gpx_muros_…`.
Las funciones de PR1 son **realmente puras** y reutilizables
por PR2 sin ciclos.

**(b) Riesgos nuevos no listados en ODD**:

1. **E1** ECharts **no está en `web/package.json`** — PR4 debe
   añadir la dependencia antes de cualquier import. Riesgo
   bloqueante si no se aborda explícitamente.
2. **E2** `last_recalc_at` no tiene schema natural — PR2
   necesita decidir dónde persiste (recomendado: tabla
   `dashboard_metadata`).
3. **E3** Cardiac Drift con solo `avg_hr` no es "drift" real —
   sin streams HR hay que documentar la limitación.
4. **E4** `users.ftp` es watts ciclismo; TSS running necesita
   `running_threshold_sec_per_km` que no existe — migración
   adicional 00013 dentro de PR2 o 00014 si se difiere.
5. **E8** `/healthz` extendido expone `training_load_rows`
   públicamente — revisar si gating interno procede.

**(c) Gaps issue ↔ plan**:

- **G1**: `users.running_threshold_sec_per_km` (sin este
  campo, TSS running queda inutilizable en PR3).
- **G2**: tabla `dashboard_metadata` para `last_recalc_at`
  (PR2 lo crea, recomendado).
- **G3**: ¿`/healthz` extendido público o detrás de token?
  (decisión de producto).
- **G4**: `POST /dashboard/recalc` (¿PR3 o follow-up?).
- **G5**: Cardiac Drift UI sin streams (¿PR4 o 1.4.1?).

**Listo para proposal**: sí — `nextRecommended: propose`. El
siguiente paso es `/sdd-new phase-1-4-performance-dashboard`
(verificar que el directorio ya existe — creado en este turno),
luego `sdd-proposal`, `sdd-spec`, `sdd-design`, `sdd-tasks` (con
chain-strategy decisión entre stacked-to-main y auto-chain en
cada PR), `sdd-apply` gated por `make test` (Go) y
`pnpm -C web test` (web).
