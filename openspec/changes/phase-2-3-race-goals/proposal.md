# Fase 2.3: carreras objetivo (#303)

Una carrera reúne sus rutas GPX, sesiones planificadas y una referencia de preparación en `/carreras`. Los planes generados por el laboratorio GPX siguen sin persistirse.

## Contrato de carreras (#314)

- `GET /api/v1/races`: listado por `race_date` ascendente y luego `id`, con `limit` (1–100, por defecto 20), `offset` (por defecto 0), `data`, `total` y `has_next`.
- `POST /api/v1/races` crea; `GET /api/v1/races/{id}` devuelve el detalle con `tracks` y `sessions`; `PUT /api/v1/races/{id}` reemplaza los campos editables; `DELETE /api/v1/races/{id}` borra la carrera. Campos editables: `name`, `race_date` (`YYYY-MM-DD`), `distance_m` (entero positivo), `elevation_gain_m` (entero no negativo), `priority` (`A`, `B` o `C`) y `notes` (opcional).
- `PUT` y `DELETE /api/v1/races/{id}/tracks/{trackId}` vinculan o desvinculan una ruta existente, de forma idempotente. `POST` y `PATCH /api/v1/workouts` aceptan `race_id`; omitirlo en un PATCH conserva la asociación y `null` la quita.
- Un ID inexistente o ajeno devuelve 404 sin distinguir ambos casos. Borrar la carrera quita los vínculos, no las rutas ni las sesiones. Revertir la migración elimina las carreras y sus vínculos, pero conserva rutas y sesiones.

## Progreso (#315)

`GET /api/v1/races/{id}/progress` devuelve un resumen **orientativo**, no una predicción de rendimiento:

- Cuenta atrás desde la fecha civil de hoy (zona del perfil) a `race_date`. Días y semanas completas son cero para una carrera pasada; `past` permite distinguirla del día de la carrera.
- Total de sesiones vinculadas y recuentos por los cuatro estados persistidos por la fase 2.2 (`planned`, `completed`, `skipped`, `modified`). No se deduce un estado a partir de una actividad; las modificadas se cuentan por separado.
- Distancia y D+ totales de las actividades registradas del usuario en los últimos 28 días naturales, incluyendo hoy. Se devuelven junto a las metas de la carrera; la comparación describe el volumen registrado, no un porcentaje de preparación. Una carrera pasada mantiene la misma ventana reciente.
- CTL, ATL y TSB de la serie de `internal/metrics`, al final de la ventana y como tendencia diaria dentro de ella. Si no hay carga calculada, no se inventa un valor. Sin registros, los recuentos y totales son cero y la serie está vacía. Solo el dueño puede consultar el progreso.

## Interfaz (#316)

`/carreras` ofrece listado por fecha, prioridad y cuenta atrás, formulario de alta y edición, detalle con rutas y sesiones, y panel orientativo con estados vacío, cargando y error. El formulario de entreno permite elegir o quitar la carrera objetivo.
