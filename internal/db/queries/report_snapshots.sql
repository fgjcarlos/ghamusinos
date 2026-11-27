-- name: UpsertReportSnapshot :one
-- Idempotente por (user_id, kind, period_start): regenerar un periodo sustituye
-- la fila en vez de duplicarla. La version del schema vive dentro del payload.
INSERT INTO report_snapshots (user_id, kind, period_start, period_end, block_id, payload)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, kind, period_start) DO UPDATE SET
    period_end   = EXCLUDED.period_end,
    block_id     = EXCLUDED.block_id,
    payload      = EXCLUDED.payload,
    generated_at = now()
RETURNING *;

-- name: GetReportSnapshot :one
-- Devuelve un snapshot del usuario. El filtro por user_id es la frontera de
-- aislamiento: el reporte de otro usuario no existe para esta consulta.
SELECT * FROM report_snapshots WHERE user_id = $1 AND id = $2;

-- name: ListReportSnapshots :many
-- Lista paginada del usuario, del periodo mas reciente al mas antiguo. Un kind
-- vacio lista todos los tipos; from_date y to_date filtran por solape con el
-- periodo del reporte (el reporte toca la ventana si empieza antes de que
-- acabe y acaba despues de que empiece).
SELECT id, kind, period_start, period_end, block_id, payload, generated_at
FROM report_snapshots
WHERE user_id = sqlc.arg('user_id')
    AND (sqlc.arg('kind')::text = '' OR kind = sqlc.arg('kind')::text)
    AND (sqlc.arg('from_date')::date IS NULL OR period_end >= sqlc.arg('from_date')::date)
    AND (sqlc.arg('to_date')::date IS NULL OR period_start <= sqlc.arg('to_date')::date)
ORDER BY period_start DESC, id
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountReportSnapshots :one
-- Total de la misma consulta, para el has_next del sobre paginado.
SELECT count(*) FROM report_snapshots
WHERE user_id = sqlc.arg('user_id')
    AND (sqlc.arg('kind')::text = '' OR kind = sqlc.arg('kind')::text)
    AND (sqlc.arg('from_date')::date IS NULL OR period_end >= sqlc.arg('from_date')::date)
    AND (sqlc.arg('to_date')::date IS NULL OR period_start <= sqlc.arg('to_date')::date);
