# Ghamusinos

Plataforma personal de **análisis y planificación para trail running**. No solo guarda actividades: convierte rutas GPX, actividades de Strava y métricas de rendimiento en información accionable para decidir mejor cómo está el corredor, cuánta fatiga acumula, cómo progresa y cómo preparar próximas rutas y carreras.

> **Estado:** fase de diseño. El repositorio contiene la documentación completa del producto y la arquitectura; la implementación arranca por la fase 1.1 (ver roadmap).

## Qué hace

| Bloque | Valor |
|---|---|
| **Laboratorio GPX** | Analiza rutas de trail: dificultad, D+/D−, VAM, subidas clave, mapa 3D, race-day, meteo y exposición solar |
| **Ingesta Strava** | Importa el histórico deportivo vía OAuth, webhooks y backfill, sin duplicar el registro |
| **Rendimiento y fatiga** | Calcula carga y forma (TSS, CTL, ATL, TSB, GAP, VAM) con fórmulas conocidas y transparentes |
| **IA opcional** | Interpreta métricas y actividades con IA (OpenAI / Claude / OpenRouter) — opt-in, nunca obligatoria |

## Stack

Aplicación **Go-first de binario único** con frontend React embebido.

| Capa | Tecnología |
|---|---|
| Backend | Go + Chi |
| Frontend | React + Vite (embebido con `embed.FS`) |
| Base de datos | PostgreSQL + TimescaleDB |
| Queries / Migraciones | SQLC / Goose |
| Jobs | River (sobre PostgreSQL) |
| Auth | Clerk |
| Mapas / Gráficas | MapLibre / ECharts |
| IA | Opcional, multi-proveedor (OpenAI → Claude → OpenRouter) |

El razonamiento detrás de cada decisión está en [`docs/architecture/technology-stack.md`](docs/architecture/technology-stack.md).

## Documentación

| Documento | Contenido |
|---|---|
| [`docs/product/ghamusinos-prd.md`](docs/product/ghamusinos-prd.md) | PRD: visión, alcance V1/V2, métricas y riesgos |
| [`docs/architecture/technology-stack.md`](docs/architecture/technology-stack.md) | Stack, tradeoffs y alternativas descartadas |
| [`docs/architecture/feature-inventory.md`](docs/architecture/feature-inventory.md) | Inventario consolidado de funcionalidades → módulos |
| [`docs/roadmap/roadmap.md`](docs/roadmap/roadmap.md) | Plan por fases (V1 1.1→1.6, V2 2.1→2.5) |
| [`docs/roadmap/v1-phase-1-plan.md`](docs/roadmap/v1-phase-1-plan.md) | Plan detallado de la fase 1.1 |
| [`docs/decisions/`](docs/decisions/) | ADRs (decisiones de arquitectura) |

## Principios de producto

- **V1 analiza; V2 planifica.**
- **Strava alimenta el sistema, pero no define el producto.**
- **Las métricas se calculan con fórmulas conocidas y transparentes.**
- **La IA interpreta datos; no inventa métricas. El producto funciona sin IA.**
- **Las métricas derivadas son estimaciones orientativas, no diagnósticos.**

## Estructura prevista del repositorio

```text
ghamusinos/
├── cmd/ghamusinos/        # entrypoint del binario
├── internal/              # backend Go (auth, strava, gpx, metrics, ai, jobs, db, http)
├── web/                   # frontend React + Vite (embebido en build)
├── docs/                  # documentación del producto y la arquitectura
├── sqlc.yaml · goose.yaml · go.mod · Makefile
```

Detalle completo en [`docs/architecture/technology-stack.md`](docs/architecture/technology-stack.md#5-estructura-de-repositorio-propuesta).

## Roadmap inmediato

1. **Fase 1.1** — Base, autenticación (Clerk) e invitaciones.
2. **Fase 1.2** — Ingesta Strava (OAuth, webhooks, backfill, dedup).
3. **Fase 1.3** — Laboratorio GPX base (parsing, métricas trail, mapa 3D).

## Licencia

Pendiente de definir.
