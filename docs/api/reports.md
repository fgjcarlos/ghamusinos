# API de reportes (fase 2.4 · issue #320)

Todas las rutas viven bajo `/api/v1`, requieren autenticación y solo ven los reportes del
usuario autenticado: el reporte de otro usuario no existe para estas rutas. Las fechas son
fechas civiles (`YYYY-MM-DD`, parseadas de forma estricta) en la zona del usuario
(`users.timezone`, UTC si no tiene preferencias). Los errores usan RFC 9457
`application/problem+json`.

- `GET /reports?kind=&from=&to=&limit=&offset=`: lista paginada de los snapshots del usuario, del
  periodo más reciente al más antiguo (`period_start` descendente y, a igualdad, `id`). `limit`
  va de 1 a 100 (20 por defecto) y `offset` debe ser no negativo (0 por defecto), como en el
  listado de bloques. `kind` es opcional y, si viene, debe ser `week`, `month` o `block`. `from` y
  `to` son opcionales y filtran por solape con `[period_start, period_end]`: entra el reporte que
  comparte al menos un día con la ventana, aunque la contenga o solo la toque. Devuelve
  `{data,total,limit,offset,has_next}`; cada elemento de `data` lleva `id`, `kind`,
  `period_start`, `period_end`, `block_id` (`null` salvo en `block`), `generated_at` y un objeto
  `summary` con `tss`, `activities`, `distance_m`, `elevation_gain_m`, `avg_ef`,
  `avg_gap_sec_per_km` y `adherence`, extraído del payload guardado, así que la lista no necesita
  el payload completo. Los promedios y `adherence` son `null` cuando el periodo no tiene muestras
  o sesiones planificadas. Un `kind` desconocido, una fecha inválida o un rango invertido
  (`from` posterior a `to`), y un `limit` u `offset` fuera de rango, devuelven `400`.
- `GET /reports/{id}`: un reporte con su `payload` completo (versión 1, documentada en
  `docs/architecture/report-snapshot.md`) más `id` y `generated_at`. Un id inexistente, ajeno o
  mal formado devuelve `404` sin distinguir el caso.
- `POST /reports/generate`: genera a demanda el periodo o bloque con `{kind, date, block_id}`.
  `kind` es obligatorio; `date` (`YYYY-MM-DD`, cualquier día dentro del periodo) lo es para `week`
  y `month`; `block_id` lo es para `block` y debe ser un bloque del usuario. Responde `202` con
  `{status:"queued", kind, period_start, period_end}`, calculando el periodo igual que el
  generador (semana de lunes a domingo, mes calendario, o las fechas del bloque). La petición no
  bloquea: encola el job `generate_report` del usuario con ese periodo explícito, y el worker
  escribe exactamente ese snapshot, que aparece en el listado y en el detalle en cuanto termina. La
  escritura es idempotente por `(user_id, kind, period_start)`, así que generar dos veces el mismo
  periodo sustituye la fila en vez de duplicarla. El job periódico, que no lleva periodo explícito,
  sigue generando las semanas cerradas y, el día 1 local, el mes cerrado. Un `kind` desconocido o un campo ausente o inválido devuelve
  `422`; un bloque ajeno o inexistente, `404`. Si la cola de jobs no está disponible o el encolado
  falla, devuelve `503` en lugar de generar en línea.
