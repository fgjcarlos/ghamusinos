# Phase 2.3 — Race goals (#303)
Delivery: ask-on-risk; local slices by behavior, likely over 400 authored lines across the epic.

## Specs

### S1 — #303

### Objetivo
Definir carreras objetivo y orientar la planificación hacia ellas.

### Entregables
- Modelo `races` en `internal/races`: nombre, fecha, distancia, desnivel y prioridad A/B/C.
- CRUD `/api/v1/races` y su pantalla.
- Asociar rutas GPX (`gpx_tracks`) a una carrera.
- Asociar sesiones planificadas a una carrera. Cierra el «asociar sesión a objetivo» de la 2.1.
- Progreso hacia el objetivo. Las métricas concretas se definen en el change de la fase.

### Criterio de cierre
El usuario crea una carrera objetivo, le asocia su track GPX y sus sesiones, y ve su progreso hacia ella.

### Tareas
- [ ] #314
- [ ] #315
- [ ] #316

### Dependencias
Fase 2.1. Puede avanzar en paralelo a la 2.2.

### Referencias
- `docs/roadmap/roadmap.md` (Fase 2.3)
- `docs/architecture/feature-inventory.md` §10
- El plan de carrera del laboratorio (fase 1.6) quedó sin persistir: no hay tabla ni CRUD de planes. Hay que valorar si se guarda aquí, ligado a la carrera.

### S2 — #314

### Problema / objetivo

No existe el concepto de carrera objetivo. Sin él, las sesiones no pueden asociarse a un objetivo, que es lo que pide la 2.1.

### Propuesta

**Migración** (up/down):

- Tabla `races` en `internal/races`:
  - `user_id`: FK con `CASCADE`
  - `name`
  - `race_date`
  - `distance_m`
  - `elevation_gain_m`
  - `priority`, con `CHECK` `A`/`B`/`C`
  - `notes`
  - timestamps
- Tabla `race_tracks (race_id, gpx_track_id)`: asocia una o varias rutas GPX a una carrera.
- `planned_sessions.race_id`: nullable, FK con `ON DELETE SET NULL`. Cierra el «asociar sesión a objetivo» de #301.

**API:**

- CRUD `/api/v1/races`, paginado.
- `PUT` y `DELETE /api/v1/races/{id}/tracks/{trackId}`.
- `POST` y `PATCH /api/v1/workouts` aceptan `race_id`.
- Todo filtrado por usuario. Un recurso de otro usuario da 404.

### Fase del roadmap

2.3

### Criterios de aceptación

- [ ] La migración es reversible y pasa el smoke de la CI.
- [ ] Tests de handler del CRUD y de las asociaciones, incluido el 404 entre usuarios (carrera, track y sesión ajenos).
- [ ] Borrar una carrera no borra sus tracks ni sus sesiones (test).
- [ ] Rechaza una prioridad fuera de A/B/C.

### Contexto / referencias

- Epic: #303. Depende de #298 y #299.
- `docs/architecture/feature-inventory.md` §10

### S3 — #315

### Problema / objetivo

El PRD pide «ver progreso hacia el objetivo», pero ningún documento define qué es progreso.

### Propuesta

**Endpoint** `GET /api/v1/races/{id}/progress`. Propuesta inicial, a cerrar en el change:

- Días y semanas que faltan para la carrera.
- Sesiones asociadas: planificadas, completadas, omitidas.
- Volumen y D+ realizados en las últimas N semanas frente a la distancia y el D+ de la carrera.
- CTL, ATL y TSB actuales y su tendencia, desde `internal/metrics`.

Las métricas son orientativas: la respuesta y la UI lo dicen, como el resto de métricas derivadas.

### Fase del roadmap

2.3

### Criterios de aceptación

- [ ] Las métricas de progreso quedan definidas en el change antes de implementar.
- [ ] Tests con una carrera futura, una pasada y una sin sesiones ni actividades.
- [ ] 404 para la carrera de otro usuario.

### Contexto / referencias

- Epic: #303. Depende de #314.

### S4 — #316

### Problema / objetivo

Las carreras objetivo necesitan su pantalla para crearlas, asociarles rutas y ver el progreso.

### Propuesta

**Ruta `/carreras`** en `web/src/features/races`, con enlace en la navegación.

- **Lista** ordenada por fecha, con insignia de prioridad A/B/C y cuenta atrás.
- **Formulario** para crear y editar.
- **Detalle de carrera:**
  - Datos de la carrera.
  - Rutas GPX asociadas, con selector de las rutas del usuario y enlace a `/rutas/:id`.
  - Sesiones asociadas.
  - Panel de progreso.
- En el formulario de sesión (#300), un selector de carrera objetivo.

### Fase del roadmap

2.3

### Criterios de aceptación

- [ ] Se puede crear, editar y borrar una carrera, y asociar y desasociar rutas.
- [ ] Desde el formulario de sesión se elige la carrera objetivo.
- [ ] El panel de progreso tiene estados vacío, cargando y error.
- [ ] Tests web. `typecheck`, `lint` y `test:run` en verde.

### Contexto / referencias

- Epic: #303. Depende de #314 y #315.

## Tasks

- [ ] T1 · S1–S2 · Race model, CRUD, GPX links and workout linkage · parent · commit pending
- [ ] T2 · S1, S3 · Define and implement race progress · parent · commit pending
- [ ] T3 · S1, S4 · Race screens and workout selector · parent · commit pending

## Log

- L1: Resuelve la issue #303
- L2: Selected “Progreso de 4 semanas (recomendado)” — “Comparar las últimas 4 semanas de actividades registradas con la distancia y el D+ objetivo, como referencia orientativa. Mostrar sesiones vinculadas sin inferir completadas u omitidas hasta que exista ese dato.” GPX Lab plan persistence is out of scope.
- L3 (T1): RED missing routes and POST workout rejected race_id; GREEN focused handlers, PostgreSQL cross-user CRUD/links/deletion, migration up/redo/status, Go race suite, and base differential for existing workout handlers. Risk: item 2 (ownership checks for race, track and session links); migration rollback removes only the new race data.
