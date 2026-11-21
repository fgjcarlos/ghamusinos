-- +goose Up
-- +goose StatementBegin

-- Fase 2.2 (issue #311) -- estado de la sesion y vinculacion con la
-- actividad real (importada de Strava).
--
-- `status` es el estado del ciclo de planificacion: planned (por defecto,
-- tambien para las filas existentes), completed, skipped o modified. El
-- vocabulario es exactamente el del CHECK: internal/workouts lo repite y
-- el test de contrato verifica que sigan coincidiendo.
--
-- `activity_id` enlaza la sesion con la actividad que la realizo. Es
-- opcional (NULL = sin vincular) y el UNIQUE garantiza que una actividad
-- no pueda justificar dos sesiones: el handler traduce la violacion a
-- 409. El ON DELETE SET NULL mantiene la sesion si la actividad se borra:
-- la planificacion es del usuario, no de la importacion.
--
-- El rollback (ver L2 del ODD) pierde el estado y los enlaces ya
-- escritos, pero no las sesiones ni las actividades: solo caen estas dos
-- columnas.
ALTER TABLE planned_sessions
    ADD COLUMN status TEXT NOT NULL DEFAULT 'planned'
        CONSTRAINT planned_sessions_status_valido
        CHECK (status IN ('planned', 'completed', 'skipped', 'modified')),
    ADD COLUMN activity_id UUID
        CONSTRAINT planned_sessions_activity_fk
        REFERENCES activities(id) ON DELETE SET NULL,
    ADD CONSTRAINT planned_sessions_activity_unica UNIQUE (activity_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revierte 00017: caen las columnas (con sus constraints, que viven en
-- ellas) y la tabla vuelve al estado de la fase 2.1. Las actividades no
-- se tocan; el estado y los enlaces ya escritos se pierden.
ALTER TABLE planned_sessions
    DROP COLUMN IF EXISTS activity_id,
    DROP COLUMN IF EXISTS status;

-- +goose StatementEnd
