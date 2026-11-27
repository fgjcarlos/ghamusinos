# Snapshots de reporte (fase 2.4 · issue #318)

`internal/reports` genera un único modelo de reporte para semanas, meses y
bloques de entrenamiento. Sustituye a las dos tablas de snapshot del legado
(`runner_profile_snapshots` y `training_analysis_snapshots`, que nunca se
portaron a este repositorio): una sola tabla `report_snapshots`, un solo payload
versionado.

El paquete separa dos piezas:

- `reports.Build` es **pura**: recibe datos ya cargados y devuelve el snapshot.
  Ni reloj ni base de datos; `Input.Now` decide los días que faltan para la
  carrera y `Input.Location` las fechas civiles.
- `reports.Store` es la capa de persistencia: carga los datos del usuario, arma
  el reporte y lo guarda con un upsert **idempotente**.

## Tabla `report_snapshots`

| Columna | Tipo | Reglas |
|---|---|---|
| `id` | `UUID` | PK, `gen_random_uuid()` |
| `user_id` | `UUID` | FK a `users`, `ON DELETE CASCADE` |
| `kind` | `TEXT` | `week`, `month` o `block` |
| `period_start` | `DATE` | primer día del periodo, en la zona del usuario |
| `period_end` | `DATE` | último día (inclusive), `>= period_start` |
| `block_id` | `UUID` | solo en `block`; FK a `training_blocks`, `ON DELETE CASCADE` |
| `payload` | `JSONB` | el snapshot completo, con `schema_version` dentro |
| `generated_at` | `TIMESTAMPTZ` | `now()` por defecto; el upsert la refresca |

`UNIQUE (user_id, kind, period_start)` es la clave del upsert: regenerar el
mismo periodo **sustituye** la fila (`period_end`, `block_id`, `payload` y
`generated_at`) en vez de duplicarla. El índice
`idx_report_snapshots_user_kind_start` sirve el listado por usuario y tipo, del
periodo más reciente al más antiguo.

Borrar el bloque borra su reporte (`ON DELETE CASCADE`); semanas y meses del
usuario siguen. Revertir la migración elimina solo los snapshots.

## Tipos de periodo

| `kind` | Periodo | Periodo anterior |
|---|---|---|
| `week` | lunes a domingo (`WeekPeriod`) | los 7 días justo antes |
| `month` | primer al último día del mes (`MonthPeriod`) | el mes calendario anterior |
| `block` | `start_date` a `end_date` del bloque | tantos días como el bloque, justo antes |

Los límites se calculan en la zona horaria del usuario (`users.timezone`, con
`workouts.UserLocation`): `WeekPeriod` y `MonthPeriod` devuelven fechas civiles
a medianoche local. En una zona que se salta la medianoche (Azores,
2026-03-29) el día empieza en la primera hora que existe, para que la fecha
nunca retroceda. Los días se cuentan sobre fechas civiles, nunca sobre horas
transcurridas: una semana con cambio de hora dura 167 o 169 horas.

Los días de `training_load_daily` son UTC y el reporte los usa tal cual, como el
dashboard. Las actividades, en cambio, se filtran por la fecha civil del
usuario, así que una actividad cerca del límite nunca cae en dos reportes.

## Payload versión 1

`Snapshot`, con `schema_version: 1`. Todos los campos están siempre presentes;
lo que no se puede calcular es `null`, nunca se omite.

| Campo | Tipo | Significado |
|---|---|---|
| `schema_version` | número | versión del payload (hoy `1`) |
| `kind` | texto | `week`, `month` o `block` |
| `period_start` | `YYYY-MM-DD` | primer día del periodo |
| `period_end` | `YYYY-MM-DD` | último día del periodo, inclusive |
| `days` | número | días civiles del periodo |
| `block` | objeto o `null` | `{id, name}` del bloque; `null` en semana y mes |
| `race` | objeto o `null` | `{id, name, race_date, priority, days_to_race}` de la carrera objetivo del bloque; `null` sin carrera |
| `load` | objeto | carga, fatiga y forma del periodo (ver abajo) |
| `volume` | objeto | `{activities, distance_m, elevation_gain_m, moving_seconds}` |
| `performance` | objeto | EF y GAP medios, orientativos |
| `plan` | objeto | cumplimiento de las sesiones planificadas |
| `previous` | objeto | comparación con el periodo anterior |

### `load`

| Campo | Significado |
|---|---|
| `tss` | suma del TSS diario del periodo |
| `daily` | un `{date, tss, activity_count}` por cada día del periodo, relleno a ceros |
| `ctl_start`, `atl_start`, `tsb_start` | CTL, ATL y TSB al cerrar el día anterior al periodo |
| `ctl_end`, `atl_end`, `tsb_end` | CTL, ATL y TSB al cerrar el último día del periodo |

Sale de `training_load_daily` con las fórmulas de `internal/metrics`: media
exponencial con τ=42 (CTL) y τ=7 (ATL), `TSB = CTL − ATL`. La EMA arranca en
cero en el primer día con datos del usuario, igual que el dashboard; los valores
se redondean a dos decimales.

### `volume`

Actividades que **empezaron** dentro del periodo (fecha civil del usuario):
número de actividades, distancia, desnivel positivo y tiempo en movimiento.

### `performance`

Orientativo, porque no se guarda el rendimiento por actividad.

| Campo | Significado |
|---|---|
| `avg_ef`, `ef_samples` | EF medio y su número de muestras |
| `avg_gap_sec_per_km`, `gap_samples` | GAP medio (s/km) y su número de muestras |

- **EF ciclismo**: `metrics.EfficiencyFactor` (vatios medios entre pulso medio),
  con potencia y pulso medios presentes.
- **EF carrera**: velocidad en m/min entre pulso medio, con distancia, tiempo en
  movimiento y pulso medios.
- **GAP carrera**: `metrics.GradeAdjustedPace` con la pendiente media del
  desnivel (`elevation_gain_m / distance_m`) y el ritmo medio en movimiento.

Ambas medias son `null` sin muestras, y un valor que no se puede calcular no
cuenta como muestra.

### `plan`

Cantidad de sesiones planificadas del periodo por estado (`planned`, `completed`,
`skipped`, `modified`) y `adherence = (completed + modified) / total`, en `null`
sin sesiones del periodo. Se redondea a dos decimales.

### `previous`

| Campo | Significado |
|---|---|
| `period_start`, `period_end` | límites del periodo de comparación |
| `activities`, `distance_m`, `elevation_gain_m` | volumen del periodo anterior |
| `tss` | TSS del periodo anterior |
| `avg_ef`, `avg_gap_sec_per_km` | rendimiento medio del periodo anterior, `null` sin muestras |

## Uso

```go
store := reports.NewStore(q)
week, err := store.GenerateWeek(ctx, userID, time.Now())   // semana cerrada que contiene la fecha
month, err := store.GenerateMonth(ctx, userID, time.Now())
block, err := store.GenerateBlock(ctx, userID, blockID, time.Now())

record, err := store.Get(ctx, userID, snapshotID)          // fila + payload guardado
```

Generar el mismo periodo dos veces deja **una** fila con el segundo payload. Un
bloque inexistente o de otro usuario responde `reports.ErrNotFound`, igual que
su carrera objetivo; un `Input` inválido (tipo desconocido o rango invertido)
responde `reports.ErrInvalid`.

## Cambiar el payload

Cualquier cambio de campos o de significado sube `SchemaVersion` y se documenta
aquí. Los snapshots ya guardados conservan su versión dentro del `payload`, así
que un lector puede distinguirlos sin consultar la tabla.
