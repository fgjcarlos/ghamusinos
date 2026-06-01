# Inventario de funcionalidades — Consolidación Ghamusinos

Este documento es la **fuente de verdad del alcance** de Ghamusinos. Consolida todo lo que se construyó en las versiones previas del proyecto y lo mapea al stack objetivo (Go-first). Sirve de columna vertebral para el roadmap y para las issues del repositorio.

> Regla de lectura: una funcionalidad existe en el producto objetivo **solo si aparece en este inventario**. Si está en código legacy pero no aquí, se considera descartada o pospuesta de forma explícita.

## 1. Origen: las cuatro bases de código

Antes de consolidar hubo cuatro carpetas. Esta es su realidad técnica, no su documentación.

| Versión | Stack real | Aporte único | Estado |
|---|---|---|---|
| `ghamusinos/` (objetivo) | Go + Chi + React/Vite embebido + PostgreSQL/TimescaleDB + SQLC + Goose + River | El stack y la visión documentada | Solo documentación |
| `ghamusinos_` | NestJS 11 + TypeORM + Astro/React + Redis/Bull | Backend de ingesta y métricas completo | MVP B+, 116 tests |
| `ghamusinos__` | Igual stack, más maduro | Lo mismo + SDD/openspec + frontend modular | MVP B+, el más completo |
| `old_ghamusinos` | Astro + React + Bun, 100% cliente, sin backend | **El laboratorio GPX** (13 modos, MapLibre 3D, meteo, solar, terreno) | 112 tests, único |

**Decisión de consolidación (tomada):** se honra el stack Go-first documentado. El código TypeScript legacy pasa a ser **especificación de referencia**, no se migra tal cual. El backend se reimplementa en Go; el frontend del laboratorio (React) se reutiliza porque el objetivo ya es React/Vite.

## 2. Cómo leer el mapeo

Cada funcionalidad lleva:

- **Origen**: de qué versión legacy proviene (o `nuevo`).
- **Módulo objetivo**: dónde vive en la estructura Go (`internal/...`) o en el frontend (`web/...`).
- **Capa**: `backend Go`, `frontend React`, `client-side` (corre solo en navegador) o `job` (River).
- **Fase**: cuándo se construye (ver `docs/roadmap/roadmap.md`).
- **Esfuerzo**: `reusar` (portar UI casi tal cual), `portar` (traducir lógica TS→Go), `reimplementar` (rehacer en Go), `nuevo`.

## 3. Autenticación e identidad

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| Login/registro con Clerk | `ghamusinos_/__` | `internal/auth` | backend Go | 1.1 | reimplementar |
| Validación de JWT de Clerk en Go | nuevo (legacy lo hacía en Node) | `internal/auth` | backend Go | 1.1 | nuevo |
| Usuario interno de dominio (mapeo Clerk → `users`) | `ghamusinos_/__` | `internal/auth`, `internal/db` | backend Go | 1.1 | reimplementar |
| Auto-creación de usuario al primer login | `ghamusinos_/__` | `internal/auth` | backend Go | 1.1 | reimplementar |
| Middleware de rutas públicas/privadas | `ghamusinos_/__` | `internal/http` | backend Go | 1.1 | reimplementar |
| Actualización de perfil (`hr_max`, `lthr`, `ftp`, nivel, timezone) | `ghamusinos_/__` | `internal/auth` | backend Go | 1.1 | reimplementar |

## 4. Invitaciones

> No existía en ninguna versión legacy. Es alcance **nuevo**, documentado en el PRD y el plan de fase 1.1.

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| Modelo `invites` (email, token_hash, status, expiración) | nuevo | `internal/db`, `internal/invites` | backend Go | 1.1 | nuevo |
| Bloqueo de acceso sin invitación activa | nuevo | `internal/auth` | backend Go | 1.1 | nuevo |
| Emisión de invitación (CLI o endpoint admin temporal) | nuevo | `internal/invites` | backend Go | 1.1 | nuevo |
| Aceptación de invitación al onboarding | nuevo | `internal/invites` | backend Go | 1.1 | nuevo |

## 5. Integración con Strava

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| OAuth 2.0 con Strava (authorize → callback → exchange) | `ghamusinos_/__` | `internal/strava/oauth.go` | backend Go | 1.2 | reimplementar |
| Almacenamiento de tokens cifrados (AES-256-GCM) | `ghamusinos_/__` | `internal/strava`, `internal/crypto` | backend Go | 1.2 | reimplementar |
| Refresh automático de tokens | `ghamusinos_/__` | `internal/strava/client.go` | backend Go | 1.2 | reimplementar |
| Webhooks: validación `hub.challenge` + recepción de eventos | `ghamusinos_/__` | `internal/strava/webhooks.go` | backend Go | 1.2 | reimplementar |
| Inbox idempotente de eventos (`activity_events`) | `ghamusinos_/__` | `internal/db`, `internal/strava` | backend Go | 1.2 | reimplementar |
| Backfill histórico acotado (ventana 42 días por defecto) | `ghamusinos_/__` | `internal/jobs/import_strava.go` | job | 1.2 | reimplementar |
| Sesión de sincronización con progreso (`sync_sessions`) | `ghamusinos_/__` | `internal/strava`, `internal/jobs` | backend Go + job | 1.2 | reimplementar |
| Deduplicación de actividades | `ghamusinos_/__` | `internal/activities/dedup.go` | backend Go | 1.2 | reimplementar |
| Normalización a modelo canónico de actividad | `ghamusinos_/__` | `internal/activities` | backend Go | 1.2 | reimplementar |
| Ingesta de streams (HR, potencia, cadencia, altitud, latlng) | `ghamusinos_/__` | `internal/jobs`, `internal/activities` | job | 1.2 | reimplementar |
| Zonas de FC desde Strava + cálculo de distribución | `ghamusinos_/__` | `internal/metrics`, `internal/strava` | backend Go | 1.2 | portar |
| Manejo de rate limits + reintentos | `ghamusinos_/__` | `internal/strava/client.go` | backend Go | 1.2 | reimplementar |

> **Decisión abierta** (ver ADR): credenciales Strava **por usuario** (modelo legacy "bring your own") vs. **app global**. Para un producto por invitación, la app global simplifica el onboarding.

## 6. Laboratorio GPX (diferenciador del producto)

> El laboratorio vive principalmente **client-side** (React, corre en el navegador) reutilizando `old_ghamusinos`. El parsing y análisis pesado puede ofrecerse también server-side en Go para persistir resultados de tracks importados. **El parsing de fichero GPX no existía en los backends NestJS**: solo `old_ghamusinos` lo hacía.

### 6.1 Carga y parsing

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| Subida de fichero `.gpx`/`.json` (GeoJSON) | `old_ghamusinos` | `web/src/features/lab` | client-side | 1.3 | reusar |
| Parsing GPX → geometría/elevación/timestamps | `old_ghamusinos` | `web/.../lab` + `internal/gpx/parser.go` | client-side + backend Go | 1.3 | reusar + portar |
| Validación de fichero (tamaño, esquema, Zod) | `old_ghamusinos` | `web/.../lab` | client-side | 1.3 | reusar |
| Hash de fichero para deduplicación | nuevo (doc en stack) | `internal/gpx` | backend Go | 1.3 | nuevo |
| Persistencia del análisis de track | nuevo | `internal/gpx`, `internal/db` | backend Go | 1.3 | nuevo |

### 6.2 Métricas de ruta (lógica pura, portar a Go + reusar en cliente)

Todas provienen de `old_ghamusinos/src/lib/utils.ts` (~25 funciones puras, 112 tests).

| Métrica / cálculo | Módulo objetivo | Capa | Fase |
|---|---|---|---|
| Distancia (Haversine) y distancia de ruta | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| D+ / D− con umbral de ruido | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Pendiente y segmentos de gradiente (heatmap) | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Effort Index (km-esfuerzo) | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| ITRA points | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Leg-Breaker Index (variabilidad de pendiente) | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| VAM estimada | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Velocidad ajustada por pendiente + tiempo estimado | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Score de dificultad (beginner→pro) | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Runnability Index (% corrible) | `internal/gpx/analysis.go` | backend Go + client | 1.3 |
| Perfil de elevación | `web/.../charts` | client-side | 1.3 |

### 6.3 Detección de features del track

| Funcionalidad | Origen | Capa | Fase |
|---|---|---|---|
| Detección de todas las subidas (tolerancia a jitter GPS) | `old_ghamusinos` | backend Go + client | 1.3 |
| Subida más vertical (Kadane) | `old_ghamusinos` | backend Go + client | 1.3 |
| King Climb (subida dominante: VAM, muros, táctica) | `old_ghamusinos` | backend Go + client | 1.3 |
| Muros (>20% durante >50 m) | `old_ghamusinos` | backend Go + client | 1.3 |
| Recovery zones (llanos tras subidas) | `old_ghamusinos` | backend Go + client | 1.3 |
| Zonas de riesgo (steep, técnica, exposición) | `old_ghamusinos` | backend Go + client | 1.3 |
| Tipo de track (circular vs punto-a-punto) | `old_ghamusinos` | backend Go + client | 1.3 |

### 6.4 Modos avanzados del laboratorio (client-side)

| Modo | Origen | Integración externa | Fase |
|---|---|---|---|
| Mapa 3D interactivo (terreno, heatmap pendientes, KM, nutrición) | `old_ghamusinos` | MapLibre GL + MapTiler | 1.3 |
| Animación fly-through + Ghost Mode (comparar dos tracks) | `old_ghamusinos` | MapLibre | 1.3 |
| Comparador de rutas (hasta 3) | `old_ghamusinos` | — | 1.3 |
| Race Day (tiempo estimado, presets de ritmo, nutrición, hitos KM) | `old_ghamusinos` | — | 1.6* |
| Cutoff Calculator (barreras horarias) | `old_ghamusinos` | — | 1.6* |
| Strategic Splits (splits por km con fatigue factor) | `old_ghamusinos` | — | 1.6* |
| Terrain Info (superficie/tecnicidad) | `old_ghamusinos` | OSM / Overpass API | 1.6* |
| Weather (4 puntos estratégicos, caché 1 h) | `old_ghamusinos` | Open-Meteo | 1.6* |
| Solar Exposure (sol/sombra/noche por hora) | `old_ghamusinos` | SunCalc | 1.6* |
| Gear Checklist (según duración, altitud, meteo) | `old_ghamusinos` | — | 1.6* |
| Post-Activity (plan vs real, Fatigue Index por km) | `old_ghamusinos` | — | 1.6* |

> `1.6*` = laboratorio avanzado. Se separa de la fase 1.3 (laboratorio base) para no inflar el alcance del MVP. Ver roadmap.

## 7. Métricas de rendimiento y carga/fatiga

| Métrica | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| TSS (Training Stress Score) ciclismo y running | `ghamusinos_/__` | `internal/metrics/performance.go` | backend Go | 1.4 | portar |
| IF (Intensity Factor) | `ghamusinos_/__` | `internal/metrics/performance.go` | backend Go | 1.4 | portar |
| GAP (Grade Adjusted Pace) | `ghamusinos_/__` + `old_ghamusinos` | `internal/metrics/performance.go` | backend Go | 1.4 | portar |
| Efficiency Factor | `ghamusinos_/__` | `internal/metrics/performance.go` | backend Go | 1.4 | portar |
| Cardiac Drift | `ghamusinos_/__` | `internal/metrics/health.go` | backend Go | 1.4 | portar |
| CTL / ATL / TSB (EMA, relleno de días vacíos) | `ghamusinos_/__` | `internal/metrics/fatigue.go` | backend Go | 1.4 | portar |
| Recálculo desde la primera actividad | `ghamusinos_/__` | `internal/jobs` | job | 1.4 | reimplementar |
| Series temporales (`training_load_daily`) | `ghamusinos_/__` | `internal/db` (TimescaleDB) | backend Go | 1.4 | reimplementar |

## 8. Dashboard y visualización

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| Lista/carrusel de actividades | `ghamusinos_/__` | `web/.../features/activities` | frontend React | 1.2 | reusar/rehacer |
| Detalle de actividad (mapa + perfil + métricas) | `ghamusinos_/__` | `web/.../features/activities` | frontend React | 1.2/1.4 | reusar |
| Mapa de actividad (polyline) | `ghamusinos_/__` (Leaflet) → MapLibre | `web/.../maps` | frontend React | 1.2 | rehacer en MapLibre |
| Gráfica CTL/ATL/TSB con selector de periodo | `ghamusinos_/__` (Recharts) → ECharts | `web/.../charts` | frontend React | 1.4 | rehacer en ECharts |
| Distribución de zonas de FC | `ghamusinos_/__` | `web/.../charts` | frontend React | 1.4 | rehacer en ECharts |
| Volumen / desnivel / nº actividades + tendencias | `ghamusinos_/__` | `web/.../features/dashboard` | frontend React | 1.4 | reimplementar |
| Modal de progreso de sincronización (polling) | `ghamusinos_/__` | `web/.../features/strava` | frontend React | 1.2 | reusar |

> **Decisión de stack:** el frontend objetivo usa **MapLibre** (mapas) y **ECharts** (gráficas). Las versiones legacy usaban Leaflet + Recharts. Se rehace la capa de visualización; el lab de `old_ghamusinos` ya usa MapLibre (encaja directo).

## 9. IA opcional (Claude)

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase | Esfuerzo |
|---|---|---|---|---|---|
| Cliente IA + feature flag global y por usuario (opt-in) | `ghamusinos_/__` | `internal/ai/claude.go` | backend Go | 1.5 | reimplementar |
| Análisis por actividad (score, fortalezas, recomendaciones) | `ghamusinos_/__` | `internal/ai`, `internal/jobs` | job | 1.5 | reimplementar |
| Análisis semanal agregado | `ghamusinos_/__` | `internal/ai`, `internal/jobs` | job | 1.5 | reimplementar |
| Análisis de carga (CTL/ATL/TSB) con tono e insights | `ghamusinos_/__` | `internal/ai` | job | 1.5 | reimplementar |
| Payload builder controlado + schema de salida validado | `ghamusinos_/__` | `internal/ai` | backend Go | 1.5 | reimplementar |
| Reintentos con backoff + no bloquear flujos críticos | `ghamusinos_/__` | `internal/ai`, `internal/jobs` | job | 1.5 | reimplementar |
| Persistencia de resultados (`ai_analysis`) | `ghamusinos_/__` | `internal/db` | backend Go | 1.5 | reimplementar |

> **Decisión abierta** (ver ADR): **Claude API directa** (principio del PRD) vs. **OpenRouter** (camino legacy probado, multi-modelo, menor coste). El PRD pide IA opcional con Claude; OpenRouter sigue siendo válido como capa de abstracción.

## 10. Planificación (V2)

| Funcionalidad | Origen | Módulo objetivo | Fase |
|---|---|---|---|
| CRUD de entrenamientos planificados | `ghamusinos_/__` | `internal/workouts` | 2.1 |
| Generación de entreno con IA | `ghamusinos_/__` | `internal/workouts`, `internal/ai` | 2.5 |
| Vinculación entreno → actividad real | `ghamusinos_/__` | `internal/workouts` | 2.2 |
| Calendario (semana/mes, drag & drop) | `ghamusinos_/__` | `web/.../features/calendar` | 2.2 |
| Carreras objetivo (prioridad A/B/C, distancia, desnivel) | `ghamusinos_/__` | `internal/races` | 2.3 |
| Reportes semanales/mensuales + snapshots | `ghamusinos_/__` | `internal/reports`, `internal/jobs` | 2.4 |
| Cron de reporte semanal | `ghamusinos_/__` | `internal/jobs` | 2.4 |

## 11. Operación, seguridad y plataforma

| Funcionalidad | Origen | Módulo objetivo | Capa | Fase |
|---|---|---|---|---|
| Healthcheck `GET /healthz` (+ DB) | `ghamusinos_/__` | `internal/http` | backend Go | 1.1 |
| Health detallado (DB, Strava, Claude) | `ghamusinos_/__` | `internal/http` | backend Go | 1.4 |
| Config por variables de entorno + validación al arrancar | `ghamusinos_/__` | `internal/config` | backend Go | 1.1 |
| Correlation-ID, logging estructurado | `ghamusinos_/__` | `internal/http` | backend Go | 1.1 |
| Rate limiting + security headers | `ghamusinos_/__` | `internal/http` | backend Go | 1.1 |
| Cifrado de secretos AES-256-GCM | `ghamusinos_/__` | `internal/crypto` | backend Go | 1.2 |
| Migraciones Goose | nuevo (legacy usaba TypeORM) | `internal/db/migrations` | backend Go | 1.1 |
| Queries tipadas SQLC | nuevo | `internal/db/queries` | backend Go | 1.1 |
| Jobs con River (colas, reintentos, idempotencia, retención) | nuevo (legacy usaba Bull/Redis) | `internal/jobs` | backend Go | 1.2 |
| CI (lint + test + build) | `ghamusinos_` | `.github/workflows` | infra | 1.1 |
| Build de binario único (frontend embebido) | nuevo | `Makefile`, `internal/frontend` | infra | 1.1 |

## 12. Funcionalidades legacy descartadas o degradadas

Decisiones explícitas para evitar arrastrar deuda:

| Elemento legacy | Decisión |
|---|---|
| `DebugController` sin auth (SQL raw) | **Descartado.** Operaciones de debug solo tras auth y solo en entorno dev. |
| Redis + Bull para colas | **Sustituido** por River sobre PostgreSQL (decisión de stack). |
| Leaflet (mapas) | **Sustituido** por MapLibre. |
| Recharts (gráficas) | **Sustituido** por ECharts. |
| Doble snapshot (`runner_profile_snapshots` + `training_analysis_snapshots`) | **Unificar** en un único modelo de snapshot al portar reportes (V2). |
| Componentes frontend duplicados (plano + modular) | **No se arrastran.** Frontend nuevo organizado por features. |
| Desincronización schema SQL ↔ entidad ORM de streams | **No aplica** (SQLC parte de SQL real, sin ORM). |

## 13. Resumen de esfuerzo por fase

| Fase | Foco | Volumen principal |
|---|---|---|
| 1.1 | Base, auth, invitaciones, plataforma | reimplementar + nuevo |
| 1.2 | Ingesta Strava completa + actividades | reimplementar (alto) |
| 1.3 | Laboratorio GPX base | reusar (frontend) + portar (cálculos) |
| 1.4 | Métricas de carga/fatiga + dashboard | portar + rehacer visualización |
| 1.5 | IA opcional con Claude | reimplementar |
| 1.6 | Laboratorio GPX avanzado (meteo, solar, terreno, race-day) | reusar (client-side) |
| 2.x | Planificación: entrenos, calendario, carreras, reportes | reimplementar |
