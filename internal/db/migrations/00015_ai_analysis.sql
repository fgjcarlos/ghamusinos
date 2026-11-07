-- +goose Up
-- +goose StatementBegin

-- Phase 1.5 (issue #17, T4) — persistencia de los análisis de IA.
--
-- Una fila por usuario y objetivo de análisis. La restricción UNIQUE
-- (user_id, kind, scope) es lo que hace que una petición repetida
-- actualice la misma fila en vez de duplicarla:
--
--   - kind:  'activity' | 'weekly' | 'load' (los tres análisis de S4).
--   - scope: el objetivo analizado: el id de la actividad o la clave de
--            la ventana (p. ej. "2026-W23" o "2026-06-01..2026-06-07").
--
-- Privacidad: la columna `payload` guarda el payload agregado que se
-- envió realmente al proveedor. internal/ai garantiza esa forma: sólo
-- métricas numéricas agregadas (distancia, duración, desnivel, frecuencia
-- cardíaca media y carga calculada); sin identidad, sin nombre, sin GPS y
-- sin streams crudos. La columna `output` guarda la respuesta validada,
-- que es interpretativa (resumen, observaciones, cautions) y no contiene
-- métricas.
--
-- `error_code` y `error_message` son seguros por construcción: el código
-- es un token en minúsculas y el mensaje es una frase corta escrita por la
-- aplicación. El cuerpo que devuelve el proveedor y cualquier credencial
-- nunca llegan a estas columnas.
CREATE TABLE ai_analysis (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind           TEXT        NOT NULL CONSTRAINT ai_analysis_kind_valido
                               CHECK (kind IN ('activity', 'weekly', 'load')),
    scope          TEXT        NOT NULL CONSTRAINT ai_analysis_scope_valido
                               CHECK (scope <> '' AND length(scope) <= 128),
    status         TEXT        NOT NULL DEFAULT 'pending'
                               CONSTRAINT ai_analysis_status_valido
                               CHECK (status IN ('pending', 'completed', 'failed')),
    provider       TEXT        CONSTRAINT ai_analysis_provider_valido
                               CHECK (provider IS NULL OR provider ~ '^[a-z0-9_-]{1,64}$'),
    model          TEXT        CONSTRAINT ai_analysis_model_valido
                               CHECK (model IS NULL OR (model <> '' AND length(model) <= 128)),
    payload        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    output         JSONB,
    error_code     TEXT        CONSTRAINT ai_analysis_error_code_valido
                               CHECK (error_code IS NULL OR error_code ~ '^[a-z0-9_]{1,64}$'),
    error_message  TEXT        CONSTRAINT ai_analysis_error_message_valido
                               CHECK (error_message IS NULL OR length(error_message) <= 500),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, kind, scope)
);

-- Índice de lectura: los análisis de un usuario, del más reciente al más
-- antiguo. El índice único de (user_id, kind, scope) cubre la búsqueda
-- exacta del objetivo.
CREATE INDEX idx_ai_analysis_user_created ON ai_analysis (user_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- El rollback elimina la tabla completa: no hay datos que conservar (los
-- análisis son caché interpretativa, reproducible bajo demanda) y dejar
-- filas huérfanas sin tabla no es posible por el ON DELETE CASCADE.
DROP TABLE IF EXISTS ai_analysis;

-- +goose StatementEnd
