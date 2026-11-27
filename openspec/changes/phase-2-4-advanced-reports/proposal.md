# Fase 2.4: reportes avanzados (#304)

Reportes semanales, mensuales y por bloque de entrenamiento, generados solos cada semana y consultables en `/reportes`. Un único modelo de snapshot sustituye a las dos tablas del legado, que nunca se portaron a este repositorio.

## Bloques de entrenamiento (#317)

- `training_blocks` guarda un rango de fechas con nombre, notas y una carrera objetivo opcional (`race_id`, `ON DELETE SET NULL`).
- `GET /api/v1/blocks`: listado por `start_date` con `limit` (1–100, por defecto 20), `offset` (por defecto 0), `data`, `total` y `has_next`. `POST` crea; `GET /api/v1/blocks/{id}` devuelve uno; `PUT /api/v1/blocks/{id}` reemplaza los campos editables; `DELETE /api/v1/blocks/{id}` borra. Campos editables: `name`, `start_date` y `end_date` (`YYYY-MM-DD`, con `end_date` igual o posterior a `start_date`), `race_id` (opcional) y `notes` (opcional). Un `PUT` sin `race_id` quita la carrera.
- **Decisión: no se permiten bloques solapados.** Dos bloques se solapan cuando comparten al menos un día; tocarse sin compartir día es válido. Responde 409. La comprobación corre dentro de una transacción con un lock consultivo por usuario, así que dos pedidos simultáneos no se cuelan.
- `race_id` debe ser una carrera del usuario: si no existe o es ajena, 404 sin distinguir ambos casos. Un bloque inexistente o ajeno también devuelve 404; un campo o rango inválido devuelve 422. Una escritura rechazada no cambia lo guardado.
- La interfaz mínima para crear y editar bloques vive en `/reportes`, junto al selector por bloque (#320).
- Revertir la migración elimina los bloques; carreras y sesiones siguen.

## Snapshots y generador (#318)

- `report_snapshots` guarda un reporte por usuario, tipo y periodo: `kind` (`week`, `month`, `block`), `period_start` y `period_end` (fechas de calendario en la zona horaria del usuario), `block_id` (solo en `block`, `ON DELETE CASCADE`: borrar el bloque borra su reporte), `payload jsonb` y `generated_at`. `UNIQUE (user_id, kind, period_start)` hace que regenerar sustituya la fila en vez de duplicarla. La versión del schema vive dentro del `payload` (`schema_version`).
- **Tipos de periodo.** Semana: de lunes a domingo. Mes: del primer al último día del mes. Bloque: del `start_date` al `end_date` del bloque. Los límites se calculan en la zona horaria del usuario (`users.timezone`).
- **Periodo anterior**, para la comparación: el mes calendario anterior en `month`; en `week` y `block`, la misma cantidad de días justo antes del periodo.
- **Generador.** `internal/reports` separa una función pura que arma el reporte a partir de datos ya cargados, de una capa de persistencia idempotente que carga los datos del usuario, arma el reporte y lo guarda con un upsert. Bloque y carrera se cargan filtrados por usuario.
- **Carga (CTL, ATL, TSB)** desde `training_load_daily` e `internal/metrics`: TSS del periodo con la serie diaria completa a ceros, y CTL, ATL y TSB al cerrar el día previo al periodo y al cerrar el último día. Los días de `training_load_daily` son UTC y el reporte los usa tal cual, como el dashboard.
- **Volumen**: número de actividades, distancia, desnivel positivo y tiempo en movimiento de las actividades que empezaron dentro del periodo (en la zona del usuario).
- **Rendimiento, orientativo**, porque no se guarda por actividad: EF medio (ciclismo: vatios medios entre pulso medio, con `metrics.EfficiencyFactor`; carrera: velocidad en m/min entre pulso medio) y GAP medio (carreras: `metrics.GradeAdjustedPace` con la pendiente media del desnivel y el ritmo medio en movimiento). Van con su cantidad de muestras y quedan en `null` sin muestras.
- **Cumplimiento del plan** sobre las sesiones planificadas del periodo: cantidad por estado y `adherence` = (completadas + modificadas) / total, en `null` sin sesiones.
- El payload versión 1 se documenta campo por campo en `docs/architecture/report-snapshot.md`.
