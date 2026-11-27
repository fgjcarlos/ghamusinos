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
