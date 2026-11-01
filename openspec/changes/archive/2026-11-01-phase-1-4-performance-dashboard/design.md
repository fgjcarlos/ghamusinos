# Design: phase-1-4-performance-dashboard

## Decisions

### D1 — `training_load_daily` como hypertable
- **Context:** se requiere una serie diaria de carga por usuario, consultada por usuario y rango temporal.
- **Decision:** tabla relacional `training_load_daily`, convertida a hypertable con chunks de 7 días e índice `(user_id, day DESC)` en `00012`.
- **Rationale:** conserva una fila por usuario/día y permite consultas temporales con TimescaleDB; sigue el patrón de tablas relacionales separado de JSONB descrito en la proposal.
- **Alternatives:** tabla BTREE con particionado por trigger; JSONB en `activities`; materialized view refrescada por cron.
- **Tradeoffs:** el modo hypertable requiere TimescaleDB. `00012` conserva una tabla normal cuando no existe la extensión y la CI con `timescaledb/timescaledb-ha:pg16` prueba la conversión real (E9).

### D2 — Umbral de running en migración 00013 (G1)
- **Context:** TSS running requiere un umbral de ritmo; `users.ftp` representa FTP de ciclismo y no lo sustituye.
- **Decision:** añadir `users.running_threshold_sec_per_km (120..1800) SMALLINT NULL` en PR2, con CHECK de rango y Down simétrico.
- **Rationale:** valor opt-in, configurable/derivable de prueba o resultado de carrera; no inventar un proxy basado en HR.
- **Alternatives:** diferirlo a follow-up (rechazado: TSS running no sería utilizable al lanzar la serie).
- **Tradeoffs:** SQLC/servicio deben distinguir NULL de cero. El CHECK y el contrato documental fijan el rango aceptado de 120..1800 segundos/km.

### D3 — `dashboard_metadata` como tabla por usuario (G2)
- **Context:** la API interna de health necesita resultado/timestamp del recálculo y número de filas.
- **Decision:** crear en `00012` una fila por usuario con `user_id` PK, `last_recalc_at`, `last_recalc_status` y `training_load_rows`.
- **Rationale:** estado explícito escrito por el worker; evita consultas improvisadas a `MAX(computed_at)` y evita añadir estado operativo a `users`.
- **Alternatives:** derivar `MAX(computed_at)` desde training load; columnas en `users`.
- **Tradeoffs:** write/upsert adicional al finalizar o fallar un job; puede no existir fila antes del primer recálculo, por lo que lecturas deben modelar valores ausentes.

### D4 — `/healthz` extendido con header (G3)
- **Context:** detalles de DB, Strava, actividad de recálculo y configuración AI son información operativa.
- **Decision:** `X-Internal-Health: 1` solicita secciones ampliadas; el GET sano sin header mantiene `{ "status": "ok" }`.
- **Rationale:** conserva el contrato público para probes y limita la exposición ordinaria.
- **Alternatives:** endpoint separado `/healthz/full`; bearer token (mayor coste operativo); respuesta completa pública.
- **Tradeoffs:** el header es un selector de representación, no una credencial. La red/deploy debe limitar el acceso interno si los datos necesitan protección fuerte. Cuando falla el ping DB, ambas representaciones devuelven HTTP 503 con `{ "status": "degraded", "db": { "ok": false } }`; las secciones operativas restantes se omiten.

### D5 — `POST /api/v1/dashboard/recalc` en PR3 (G4)
- **Context:** historiales anteriores al recálculo incremental requieren un disparador explícito.
- **Decision:** endpoint autenticado en PR3 encola `RecalcTrainingLoadArgs{UserID, From:nil, To:nil}` y retorna `202` sin esperar al worker.
- **Rationale:** recálculo de historial completo asíncrono, alineado con la deduplicación del job.
- **Alternatives:** diferir al follow-up (rechazado por G4).
- **Tradeoffs:** `202` significa encolado, no completado. El response incluye `job_id`, `queued_at`, `will_recompute_from`; la fecha debe consultarse para la primera actividad o ser null si no hay actividades (DA-004).

### D6 — Cardiac Drift UI condicionado a streams HR (G5)
- **Context:** `avg_hr` no conserva la evolución intrasesión requerida para drift.
- **Decision:** no calcular/representar drift si no hay streams HR; la UI explica el motivo con «necesita streams HR para calcular».
- **Rationale:** honestidad de métrica antes que mostrar un proxy ruidoso.
- **Alternatives:** mostrar siempre con advertencia (rechazado).
- **Tradeoffs:** hace falta un indicador de disponibilidad de streams para el rango. El response de HR zones (`degraded`) indica uso de fallback de zonas, no es por sí mismo el contrato de disponibilidad de streams de Cardiac Drift. DW-005 también debe precisar cómo puede aparecer el texto si el panel se oculta del DOM.

### D7 — Deduplicación River por ventana, TTL 30s (E7)
- **Context:** sincronizaciones con muchas actividades pueden crear muchos recálculos.
- **Decision:** `UniqueOpts` para `recalc_training_load`, con clave conceptual `recalc_training_load:{user_id}:{window_hash}` y período de 30 s; el hook usa ventana `started_at ± 3 días`.
- **Rationale:** coalescer enqueues idénticos sin añadir un batch global con cron.
- **Alternatives:** job consolidado global por usuario/cron.
- **Tradeoffs:** ventanas diferentes permanecen distintas; la propuesta no demuestra que ventanas solapadas se coaleszcan. Confirmar semántica de `UniqueOpts` y composición concreta de opciones con la versión River del proyecto durante PR2.

### D8 — GAP con curva Minetti normalizada (E6)
- **Context:** curva académica y convención de GAP compatible con origen Strava pueden divergir.
- **Decision:** `GradeAdjustedPace` usa `pace × C(grade)/C(0)` con curva Minetti normalizada y grade limitado a `[-0.35, +0.35]`.
- **Rationale:** mantiene compatibilidad de unidades/convención con entradas de Strava y hace explícita la curva.
- **Alternatives:** Minetti crudo.
- **Tradeoffs:** declarar explícitamente si grade es fracción (0.10 = 10%) y verificar los golden tests de ±10% frente a la fórmula matemática antes de aceptar la especificación.

## Architecture overview

```text
PR1: metrics puras (TSS / IF / GAP / EF / Drift / PMC)
                         │
Strava activities ──> UpsertActivity ──(éxito)──> encolar RecalcTrainingLoad
                                                  args: user + (día ± 3)
                                                  UniqueOpts, 30 s
                                                        │
                                                        v
                                      ComputeDailyLoad + FillMissingDays
                                                        │
                                      ┌─────────────────┴──────────────────┐
                                      v                                    v
                           training_load_daily                      dashboard_metadata
                           (hypertable diaria)                 (estado/filas recalc)
                                      │                                    │
                   ┌──────────────────┴──────────────┐                     └──> /healthz interno
                   v                                 v
 GET /dashboard/{summary,load,hr-zones}     POST /dashboard/recalc
                   │                                 │
                   └──────────────> JSON API <───────┘
                                      │ tipos/fetchers espejo
                                      v
                       web /dashboard + ECharts
```

Scope implementable: Go root `.` para PR1–PR3 y `web/` para PR4. Cuatro PRs encadenados y cada uno apunta a quedar mergeable; cada proyecto se verifica por separado. Los fallos de cálculo/lectura se devuelven según errores RFC 9457 existentes, salvo health que usa estado de disponibilidad.

## Data flow

### Actividad nueva → recálculo incremental
1. La ingestión confirma `UpsertActivity`; solo tras éxito se solicita un job con `user_id` y límites `started_at - 3d` / `started_at + 3d`.
2. El enqueuer aplica identidad única derivada de usuario y ventana; duplicados idénticos dentro de 30 s se deduplican según comportamiento confirmado de River.
3. El worker calcula TSS por actividad en el rango; antes de agrupar convierte `started_at` a la zona horaria del usuario y escribe una fila por día mediante upsert.
4. Al terminar, actualiza `dashboard_metadata` con timestamp, estado y número de filas. Debe registrar `failed` y timestamp de intento también ante error (TL-007).

### Primer login / recálculo completo
No se especifica un hook automático de primer login en proposal ni en specs: el camino garantizado para historial completo es `POST /api/v1/dashboard/recalc`, autenticado y asíncrono. Este encola límites nulos; el worker encuentra la primera actividad y calcula hasta hoy. No inferir un auto-backfill durante login.

### Visualización dashboard y relleno
1. El cliente selecciona rango (default 30 días) y llama summary, load y HR-zones; la autorización es por usuario.
2. Load obtiene TSS diario, crea días ausentes como TSS cero en rango inclusivo y corre EMA CTL (42 días) / ATL (7 días); TSB = CTL − ATL. La API devuelve un punto por fecha y `Cache-Control: private, max-age=60`.
3. HR-zones prefiere samples de streams; solo si no existen usa estimación desde `avg_hr × elapsed_seconds`, distribuyendo según contrato de zonas, y marca `degraded: true`.
4. La página muestra KPIs y gráficas cuando existen actividades; el selector 7d/30d/90d/1y actualiza los hooks. ECharts debe incorporarse como primer cambio de PR4 antes de cualquier import.

## File-by-file per PR

### PR1 — Métricas puras (`.`)
- `internal/metrics/performance.go`: TSS ciclismo/running, IF, GAP y EF.
- `internal/metrics/health.go`: Cardiac Drift puntual y por serie de muestras.
- `internal/metrics/fatigue.go`: CTL/ATL/TSB, constantes tau 42/7 y `FillMissingDays`.
- `internal/metrics/SPEC.md`: fórmula, unidades, fuentes y casos golden por métrica.
- `internal/metrics/{performance,health,fatigue}_test.go` y ejemplos ejecutables previstos por proposal.
- Ninguna dependencia de SQL, HTTP, DB o frontend.

### PR2 — Persistencia, SQLC y job (`.`)
- `internal/db/migrations/00012_training_load_daily.sql`: `training_load_daily`, índice, hypertable condicional y `dashboard_metadata`; Down reversible.
- `internal/db/migrations/00013_users_running_threshold.sql`: nullable running threshold y CHECK; Down reversible.
- `internal/db/queries/training_load.sql`, `dashboard_metadata.sql` y extensión de `users.sql`: upsert/listas/metadata/umbral; regenerar `internal/db/sqlc/` con SQLC.
- `internal/metrics/training_load.go`: `ComputeDailyLoad(ctx, db, userID, from, to)`, agrupación por día local e idempotencia.
- `internal/jobs/recalc_training_load.go` y registro de worker/dependencias: args, ejecución, full-history por límites nil, metadata y dedupe.
- Hook post-`UpsertActivity` en el punto efectivo de escritura: encolar ventana ±3 días. El proposal deja pendiente descubrir la ubicación exacta; escogerla durante apply, no fijarla aquí.
- Tests de agrupación TZ, idempotencia, job/dedupe y migration forward/back con smoke TimescaleDB.

### PR3 — API y health (`.`)
- `internal/http/handlers/dashboard.go`: summary, load y HR zones; auth de usuario, parámetros ISO, errores RFC 9457.
- `internal/http/handlers/dashboard_recalc.go`: POST autenticado con 202 y metadatos del job.
- `internal/http/handlers/health.go`: representación pública mínima e interna ampliada; rama 503.
- Wiring de dashboard en router de API y middleware JWT existente; consulta de metadata/Strava/DB para health.
- Tests `httptest` + chi y dobles sqlc/enqueuer para escenarios de permisos, agregación, fechas, fallback y health.

### PR4 — Dashboard web (`web/`)
- `web/package.json` y lockfile: ECharts añadido antes de imports; priorizar imports tree-shaken de `echarts/core`.
- `web/src/lib/api/dashboard.ts`: interfaces y fetchers espejo del JSON Go.
- `web/src/features/dashboard/`: hooks/containers y componentes summary, training load, HR zones, cardiac drift y selector, con tests y CSS modules.
- `web/src/routes/dashboard.tsx` y wiring de rutas; `AppShell` añade enlace.
- `web/src/test/setup.ts` solo si ECharts requiere un mock compartido; colores mediante `var(--gh-*)`, empty state reutiliza UI existente.

## Sequence diagrams

### 1. Actividad nueva → dashboard
```text
Strava/ingest      UpsertActivity       Enqueuer/River       Recalc worker       DB
     |                   |                    |                    |              |
     |                   |-- upsert OK ------>|                    |              |
     |                   |                    |-- user, day±3 ---->|              |
     |                   |                    |   UniqueOpts       |              |
     |                   |                    |                    |-- read acts ->|
     |                   |                    |                    |-- upsert load>
     |                   |                    |                    |-- metadata -->|
     |                   |                    |                    |              |
Dashboard web       GET summary/load/zones    API handlers         SQLC           DB
     |-------------------------- request -------------------------->|              |
     |<------------------------ JSON shapes ------------------------|<-------------|
```

### 2. `/dashboard/load` con días ausentes
```text
Browser       GET /dashboard/load       Handler        SQLC / metrics          DB
  |                    |                   |                  |                  |
  |-- from,to + JWT -->|------------------>|-- auth user ---->|                  |
  |                    |                   |-- query daily ->|----------------->|
  |                    |                   |<-- sparse TSS ---|<-----------------|
  |                    |                   |-- FillMissingDays (UTC API dates)   |
  |                    |                   |-- EMA CTL/ATL; TSB = CTL - ATL      |
  |<-- 200 + dates + private max-age=60 ---|                  |                  |
```

Al persistir, los días se agrupan en TZ de usuario; el contrato de `FillMissingDays` y las fechas de API son fechas de calendario. El efecto de ejecutar EMA desde el comienzo del historial frente al rango solicitado debe mantenerse consistente con datos previos; no inicializar de nuevo a cero en cada consulta si hay estado anterior disponible.

### 3. `/healthz` header-gated
```text
Probe/ops         GET /healthz          Health handler       DB/status + metadata
   |                   |                      |                       |
   |-- no header ----->|--------------------->|-- ping ------------->|
   |<-- {status:ok} ---|<---------------------|<-- healthy ----------|
   |                   |                      |                       |
   |-- X-Internal-Health:1 ------------------>|-- ping/metadata ---->|
   |<-- status + db/strava/recalc/rows/ai ----|<-- healthy data -----|
   |                   |                      |                       |
   |-- internal + DB down ------------------>|<-- error -------------|
   |<-- 503 degraded; extended db failure body per resolved contract |
```

## Wiring contract Go ↔ TypeScript

Los nombres JSON snake_case son el contrato de handlers; TypeScript los conserva sin transformación. La proposal enumera shapes en `proposal.md` Wire-shape delta y PR4. Requisitos completos de endpoints están en `specs/dashboard-api/spec.md` (DA-001…DA-006), UI en `specs/dashboard-web/spec.md` (DW-001…DW-007).

| Endpoint / Go response JSON | TypeScript mirror (`web/src/lib/api/dashboard.ts`) | HTTP / notes |
|---|---|---|
| `GET /api/v1/dashboard/summary`: `weekly_volume_m`, `weekly_elevation_m`, `weekly_activities_count`, `trend_7d_pct`, `trend_30d_pct` | `DashboardSummary`; trends son `number \| null` | `200`; tendencias null si ventana previa sin filas; requiere JWT |
| `GET /api/v1/dashboard/load?from=&to=`: `{from,to,series:[{day,ctl,atl,tsb}]}` | `TrainingLoadSeries`, `TrainingLoadPoint` | `200`, ISO date sin TZ, inclusivo y una entrada/día; `Cache-Control: private, max-age=60` |
| `GET /api/v1/dashboard/hr-zones?from=&to=`: `{from,to,degraded,zones:[{zone,minutes}]}` | `HRZonesResponse`, `HRZoneBucket` (`z1`…`z5`) | `200`; degraded señala fallback avg_hr; requiere JWT |
| `POST /api/v1/dashboard/recalc`: `{job_id,queued_at,will_recompute_from}` | `RecalcResponse` | `202`; primer día de actividad o null; límites de job nil; requiere JWT |
| `GET /healthz` público | `HealthPublic` si se modela | `200 {status:ok}`; forma de fallo público a resolver |
| `GET /healthz` con `X-Internal-Health: 1`: status/db/strava/last_recalc_at/training_load_rows/ai | tipo interno separado opcional; nullable timestamps según spec | `200` sano; `503` DB caída; sólo secciones internas solicitadas por header |

Errores de API autenticada/error de parámetro usan el envelope RFC 9457 (`application/problem+json`) conforme a handlers existentes; spec no fija códigos concretos para parámetros inválidos, rango invertido, errores de DB ni fallos de enqueue. Definirlos de manera uniforme en PR3 y no presentar esos códigos como contrato ya acordado.

### Notas de fidelidad matemática y serialización
- Los specs exponen outputs de pace/TSS como `float64`; DB persiste TSS `NUMERIC(8,2)`. Documentar el redondeo únicamente en frontera de persistencia, no en funciones puras.
- CTL/ATL para una respuesta de rango han de tener warm-up histórico suficiente; calcular EMA solo sobre el rango visible cambia el resultado respecto al PMC desde inicio de actividad. La propuesta contempla inicio desde primera actividad y fill de rango, pero hay que fijar el query/warm-up al implementar.
- HR-zones API devuelve exactamente buckets z1..z5 en el ejemplo/spec. El detalle de distribución uniforme del fallback y la interpretación de `hr_zones` deben conservar misma semántica Go/Web.

## Test strategy

### Por PR
- **PR1:** tests table-driven de cada métrica con golden de literatura, valores degenerados y límites; casos de relleno inclusive/UTC, series vacías y EMA. Gate: `make test`; calidad: `make lint`, `make vet`, `make fmt`; coverage ≥90% es objetivo de proposal, no umbral global configurado.
- **PR2:** migration Up/Down y Timescale smoke en `timescaledb/timescaledb-ha:pg16`; test de grupos por fecha local, `UpsertDailyLoad` idempotente, job nil/full-history, metadata ok/failure y dedupe con TTL. Gate: `make test` y quality gates Go.
- **PR3:** `httptest` + chi, auth real del middleware o fixture apropiada, sqlc mocks y enqueuer mock; verificar shapes, orden/continuidad diaria, private cache, nulos de tendencia, streams preferidos/fallback, 202, health público/interno y 503. Gate: `make test` y quality gates Go.
- **PR4:** Vitest + Testing Library; mockear ECharts en test setup si DOM canvas lo requiere; probar loading/error/populado, hooks/rango, labels degraded, empty state y gating drift. Gate: `pnpm -C web test:run`, `pnpm -C web typecheck`, `pnpm -C web lint`, `pnpm -C web format:check`.

### Fronteras de mocks e integración
SQLC Querier/repository y River enqueuer se sustituyen por interfaces/dobles en unit/handler tests; no exigir PostgreSQL/River vivo para `go test ./...`. La migración hypertable requiere smoke con TimescaleDB real. Para web, verificar que la opción de ECharts se entrega al adaptador probado, no afirmar que canvas se valida visualmente en jsdom.

## Rollback

Seguir el plan por PR de `proposal.md`, sección `Rollback` (aprox. líneas 850–951): revertir en orden inverso de dependencias: PR4 → PR3 → PR2 → PR1. PR2 Down elimina metadata e hypertable/table y la migración 00013 elimina el campo de umbral; PR3 revierte endpoints/health sin cambiar contrato web existente; PR4 quita dashboard, rutas y ECharts. No revertir PR1 mientras PR2 dependa de las métricas. Confirmar el Down de `00012` en ambas bases (con y sin extensión) como parte de smoke.

## Open design questions

1. **Umbral running:** rango contractual unificado en 120..1800 segundos/km para el CHECK de la migración y los consumidores.
2. **Bloqueante de fórmula — `TSSRunning`:** M-002 dice `IF = actualSecPerKm / thresholdSecPerKm`; para pace menor (más rápido) eso produce IF menor, contradiciendo el texto y la formulación por velocidad del exploration/proposal. Corregir spec a ratio de velocidades (`thresholdSecPerKm / actualSecPerKm`) y recalcular golden de ritmo 4:30, o decidir otra fórmula antes de implementación.
3. **Bloqueante de fórmula — Cardiac Drift:** exploration propone comparar media de primer y tercer cuartil; proposal describe partir streams en mitades / promediar pares; M-006 especifica medias de primer y tercer cuartil. Mantener el contrato del spec o editar la proposal para que no haya dos algoritmos.
4. **Health degradado:** proposal presenta `503 {status:degraded, db:{ok:false}}` incluso sin header; DA-006 especifica body público `{status:degraded}` y `db.ok=false` solo para header interno. Escoger una representación.
5. **Cardiac Drift empty state:** G5 permite panel oculto o mensaje; DW-005 exige panel ausente y label presente. Especificar si el label se muestra fuera del panel para evitar una condición imposible.
6. **TSSRunning sin umbral y sesión sin actividades:** definir conducta del servicio/API cuando threshold es NULL y comportamiento `will_recompute_from` si no hay actividad (spec establece null para este último).
7. **Serie/EMA:** especificar el warm-up desde primera actividad para rangos visibles posteriores y cómo se inicializa una cuenta sin carga previa, de modo que resultados no dependan arbitrariamente de `from`.
8. **Correlación E7:** TTL/unique key deduplica mismo user + misma ventana, no necesariamente ventanas ±3 solapadas; confirmar si basta la garantía literal de TL-006 o si se exige coalescing de ventanas solapadas.

## Referencias de artefactos

- `proposal.md`: decisiones G1–G5, riesgos E1–E9, shapes, tests y rollback.
- `exploration.md`: evidencia técnica y detalles de fórmulas, patrones y riesgos.
- `specs/metrics-go/spec.md`: M-001…M-010.
- `specs/training-load/spec.md`: TL-001…TL-007.
- `specs/dashboard-api/spec.md`: DA-001…DA-006.
- `specs/dashboard-web/spec.md`: DW-001…DW-007.
- `openspec/config.yaml`: reglas design (decisiones/rationale/alternativas, secuencias, wiring Go↔TS), testing por proyecto.

---

**Skill resolution:** `fallback-path` (no se inyectaron rutas de skill; se cargó `gentle-ai` como fallback).