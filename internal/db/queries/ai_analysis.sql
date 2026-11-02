-- name: CreateAIAnalysis :one
-- Crea (o reutiliza) el análisis de un usuario para un objetivo concreto.
--
-- La restricción UNIQUE (user_id, kind, scope) hace que una petición
-- repetida actualice la misma fila en vez de duplicarla: el ON CONFLICT
-- devuelve la fila existente, reiniciada a 'pending' y sin resultado
-- anterior. Es el punto de entrada del job (T5) y de la API (T6).
INSERT INTO ai_analysis (user_id, kind, scope, status)
VALUES ($1, $2, $3, 'pending')
ON CONFLICT (user_id, kind, scope) DO UPDATE SET
    status         = 'pending',
    provider       = NULL,
    model          = NULL,
    payload        = '{}'::jsonb,
    output         = NULL,
    error_code     = NULL,
    error_message  = NULL,
    updated_at     = now()
RETURNING *;

-- name: GetAIAnalysis :one
-- Devuelve el análisis de un usuario para un objetivo concreto.
-- El filtro por user_id es la frontera de aislamiento: el análisis de
-- otro usuario no existe para esta consulta (pgx.ErrNoRows).
SELECT *
FROM ai_analysis
WHERE user_id = $1
  AND kind = $2
  AND scope = $3;

-- name: ListAIAnalyses :many
-- Lista los análisis de un usuario, del más reciente al más antiguo.
-- El límite ($2) acota la respuesta; el llamante decide el tamaño.
SELECT *
FROM ai_analysis
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CompleteAIAnalysis :one
-- Marca el análisis como completado y guarda el payload enviado y la
-- respuesta validada. Sólo el dueño de la fila (user_id) puede
-- actualizarla.
UPDATE ai_analysis
SET status         = 'completed',
    provider       = $4,
    model          = $5,
    payload        = $6,
    output         = $7,
    error_code     = NULL,
    error_message  = NULL,
    updated_at     = now()
WHERE user_id = $1
  AND kind = $2
  AND scope = $3
RETURNING *;

-- name: FailAIAnalysis :one
-- Marca el análisis como fallido y guarda un error seguro: error_code es
-- un token en minúsculas y error_message una frase corta escrita por la
-- aplicación. El cuerpo del proveedor nunca llega aquí. Sólo el dueño de
-- la fila (user_id) puede actualizarla.
UPDATE ai_analysis
SET status         = 'failed',
    provider       = $4,
    model          = $5,
    payload        = $6,
    output         = NULL,
    error_code     = $7,
    error_message  = $8,
    updated_at     = now()
WHERE user_id = $1
  AND kind = $2
  AND scope = $3
RETURNING *;
