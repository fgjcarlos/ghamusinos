# Roadmap — Ghamusinos

Plan de construcción por fases del producto consolidado. Cada fase es entregable de forma independiente y deja una base ejecutable para la siguiente. El alcance de cada fase deriva de `docs/architecture/feature-inventory.md`.

## Orientación rápida

| Versión | Objetivo | Fases |
|---|---|---|
| **V1** | Analizar: rutas GPX, actividades Strava y estado de forma | 1.1 → 1.6 |
| **V2** | Planificar: entrenos, calendario, carreras y reportes | 2.1 → 2.5 |

Principio rector: **V1 analiza, V2 planifica**. La IA es opcional en todo el producto.

## V1 — Análisis

### Fase 1.1 — Base, arquitectura y autenticación

**Objetivo:** una app fullstack ejecutable con acceso controlado.

Entregables: estructura Go + Chi, frontend React/Vite embebido (`embed.FS`), config por entorno, PostgreSQL + Goose + SQLC, auth Clerk con usuario interno, modelo de invitaciones y bloqueo de acceso, healthcheck, CI y Makefile.

Criterio de cierre: un usuario invitado entra por Clerk, se mapea a usuario interno y accede a un área privada servida desde el binario.

> Plan detallado: `docs/roadmap/v1-phase-1-plan.md`.

### Fase 1.2 — Ingesta Strava

**Objetivo:** importar y normalizar actividades reales.

Entregables: OAuth Strava, tokens cifrados (AES-256-GCM), refresh automático, webhooks con inbox idempotente (`activity_events`), backfill acotado y deduplicación, normalización canónica, ingesta de streams, zonas de FC, jobs River con reintentos, lista/carrusel de actividades y modal de progreso.

Criterio de cierre: el histórico reciente del usuario aparece deduplicado y los nuevos eventos llegan por webhook sin duplicar.

### Fase 1.3 — Laboratorio GPX (base)

**Objetivo:** convertir tracks GPX en análisis trail útil. Diferenciador central.

Entregables: parsing y análisis GPX en el **backend Go** (`internal/gpx`; el cliente sube el fichero y solo renderiza), métricas de ruta (distancia, D+/D−, pendiente, Effort Index, VAM, ITRA, Leg-Breaker, Runnability, dificultad), detección de subidas, **Km Vertical (tramo de subida sostenida)**, King Climb/muros/recovery/risk zones, mapa 3D MapLibre con heatmap de pendientes y perfil de elevación, comparador de rutas, persistencia y hash de fichero.

Criterio de cierre: subir un GPX produce un análisis completo, visualizado en mapa 3D y persistido.

### Fase 1.4 — Dashboard de rendimiento y salud/fatiga

**Objetivo:** mostrar estado y evolución del corredor.

Entregables: métricas portadas a Go (TSS, IF, GAP, EF, cardiac drift, CTL/ATL/TSB con EMA y relleno de días vacíos), series temporales en TimescaleDB, recálculo desde la primera actividad, dashboard con volumen/desnivel/nº actividades y tendencias, gráficas ECharts (carga/fatiga, zonas FC), health detallado.

Criterio de cierre: con actividades importadas, el dashboard muestra tendencias y carga/fatiga coherentes.

### Fase 1.5 — IA opcional (multi-proveedor)

**Objetivo:** enriquecer la interpretación sin hacerla obligatoria.

Entregables: interfaz IA multi-proveedor (OpenAI → Claude → OpenRouter) con feature flag global y opt-in por usuario, análisis por actividad / semanal / de carga, payload builder controlado, schema de salida validado e idéntico entre proveedores, reintentos con backoff, ejecución en jobs (no bloquea el core) y persistencia (`ai_analysis`).

Criterio de cierre: con IA activada, el usuario obtiene resúmenes; con IA desactivada o caída, el producto funciona igual.

### Fase 1.6 — Laboratorio GPX avanzado

**Objetivo:** planificación de carrera y contexto ambiental (client-side, ya prototipado en `old_ghamusinos`).

Entregables: Race Day (tiempo, ritmo, nutrición, hitos KM), Cutoff Calculator, Strategic Splits, Terrain Info (OSM/Overpass), Weather (Open-Meteo), Solar Exposure (SunCalc), Gear Checklist y Post-Activity (plan vs. real).

Criterio de cierre: el laboratorio ofrece preparación de carrera completa sobre un track.

## V2 — Planificación

### Fase 2.1 — Creación de entrenos
CRUD de sesiones, tipo de entrenamiento, objetivos de duración/distancia/desnivel/intensidad, asociación a objetivo.

### Fase 2.2 — Calendario
Vista semana/mes, mover sesiones, marcar completada/omitida/modificada, vincular actividad real a sesión planificada (drag & drop).

### Fase 2.3 — Objetivos de carrera
Carrera objetivo con fecha/distancia/desnivel/prioridad (A/B/C), asociación de rutas GPX y progreso hacia el objetivo.

### Fase 2.4 — Reportes avanzados
Reportes semanales y mensuales, evolución de carga/fatiga/desnivel/rendimiento, resúmenes por bloque, cron de reporte y snapshots unificados.

### Fase 2.5 — IA aplicada a planificación
Lectura del bloque de entrenamiento, explicación de riesgos de fatiga, sugerencias orientativas (opt-in) y resúmenes antes/después de una carrera objetivo.

## Dependencias entre fases

```text
1.1 ─┬─> 1.2 ─┬─> 1.4 ──> 1.5 ──> 2.x
     │        └─> (dashboard usa métricas + actividades)
     └─> 1.3 ──> 1.6
```

- 1.2 y 1.3 pueden avanzar en paralelo tras 1.1 (ingesta vs. laboratorio).
- 1.4 depende de 1.2 (necesita actividades) y de las métricas.
- 1.5 depende de 1.4 (consume métricas calculadas).
- V2 depende de toda la V1.

## Estado de partida (reutilización)

| Fase | Qué se reutiliza del código previo |
|---|---|
| 1.1 | Patrones de auth/config/seguridad de `ghamusinos_/__` (como spec) |
| 1.2 | Flujo Strava completo de `ghamusinos_/__` (como spec a portar) |
| 1.3 | Frontend del laboratorio de `old_ghamusinos` (reuso directo) + `utils.ts` (portar a Go) |
| 1.4 | Fórmulas de métricas de `ghamusinos_/__` (portar a Go) |
| 1.5 | Servicio de IA de `ghamusinos_/__` (reimplementar en Go) |
| 1.6 | Modos avanzados de `old_ghamusinos` (reuso client-side) |
| 2.x | Módulos workouts/races/reports de `ghamusinos_/__` (portar a Go) |
