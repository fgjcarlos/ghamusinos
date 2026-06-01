# Backlog inicial de issues

Conjunto de issues a crear al abrir el repositorio. Se crean **épicas por fase** (seguimiento) y las **issues granulares de la fase 1.1** (la primera a implementar). Las fases posteriores se detallarán al acercarse.

> Convención de títulos: conventional commits (`feat`, `chore`, `fix`, `docs`). Etiqueta de fase `phase:1.x`.

## Épicas (una por fase)

| # | Título | Label |
|---|---|---|
| E1 | `epic: Fase 1.1 — Base, arquitectura y autenticación` | `epic`, `phase:1.1` |
| E2 | `epic: Fase 1.2 — Ingesta Strava` | `epic`, `phase:1.2` |
| E3 | `epic: Fase 1.3 — Laboratorio GPX (base)` | `epic`, `phase:1.3` |
| E4 | `epic: Fase 1.4 — Dashboard de rendimiento y salud/fatiga` | `epic`, `phase:1.4` |
| E5 | `epic: Fase 1.5 — IA opcional con Claude` | `epic`, `phase:1.5` |
| E6 | `epic: Fase 1.6 — Laboratorio GPX avanzado` | `epic`, `phase:1.6` |

## Issues de la Fase 1.1

Orden de implementación según `docs/roadmap/v1-phase-1-plan.md`.

| # | Título | Criterio de aceptación clave |
|---|---|---|
| 1 | `chore: scaffold del módulo Go y estructura base del repo` | `go.mod`, `cmd/ghamusinos`, `internal/*`, `Makefile`; el proyecto compila |
| 2 | `feat: servidor HTTP con Chi, healthz y graceful shutdown` | `GET /healthz` responde; middleware base; cierre ordenado |
| 3 | `feat: app React + Vite mínima con build a web/dist` | `make web-build` genera `web/dist` |
| 4 | `feat: embeber frontend con embed.FS y servir SPA con fallback` | el binario sirve la SPA; rutas cliente con fallback |
| 5 | `feat: configuración central por entorno con validación` | falla al arrancar si faltan vars obligatorias; `.env.example` |
| 6 | `feat: conexión a PostgreSQL y Goose configurado` | conexión viva; `make migrate` aplica migraciones |
| 7 | `feat: migración inicial de users e invites` | tablas `users` e `invites` creadas |
| 8 | `feat: SQLC configurado con queries mínimas de usuario e invitación` | `make generate` produce código tipado |
| 9 | `feat: validación de JWT de Clerk y middleware de rutas privadas` | ruta pública accesible; ruta privada exige sesión |
| 10 | `feat: resolución y auto-creación de usuario interno desde Clerk` | usuario Clerk se mapea a fila en `users` |
| 11 | `feat: invitaciones — emisión y bloqueo de acceso sin invitación activa` | sin invitación no entra; con invitación activa entra |
| 12 | `chore: CI (lint + test + build) y README de desarrollo local` | workflow verde; comandos de arranque documentados |

## Labels a crear

| Label | Color | Uso |
|---|---|---|
| `epic` | `#6f42c1` | Issue de seguimiento de una fase |
| `phase:1.1` … `phase:1.6` | `#0e8a16` | Fase del roadmap |
| `phase:2.x` | `#0e8a16` | Fases de V2 |
| `enhancement` | `#a2eeef` | Feature / tarea |
| `bug` | `#d73a4a` | Defecto |
| `backend` | `#1d76db` | Backend Go |
| `frontend` | `#fbca04` | Frontend React |
| `infra` | `#5319e7` | CI, build, plataforma |
| `docs` | `#0075ca` | Documentación |
