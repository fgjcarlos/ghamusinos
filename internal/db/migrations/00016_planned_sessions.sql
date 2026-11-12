-- +goose Up
-- +goose StatementBegin

-- Fase 2.1 (issue #298) -- sesiones de entrenamiento planificadas.
--
-- Una fila por sesion planificada del usuario. `scheduled_on` es un DATE
-- (dia civil), no un instante: el dia se resuelve en la zona horaria del
-- usuario (users.timezone) antes de escribir, y la columna no lleva hora
-- ni offset. Verificado: el mismo registro se lee igual bajo cualquier
-- zona horaria de sesion, asi que el dia no depende de quien lo lee.
--
-- Los objetivos son todos opcionales (NULL = no planificado) y la base
-- rechaza valores negativos: un objetivo negativo es un dato invalido, no
-- un caso raro. La validacion de dominio en internal/workouts da el
-- mensaje util; estos CHECK son la ultima linea de defensa.
--
-- El aislamiento entre usuarios vive en las queries, no en Go: las cinco
-- filtran por user_id, asi que la sesion de otro usuario no existe para
-- esa llamada. El ON DELETE CASCADE hace que al borrar el usuario caigan
-- sus sesiones.
CREATE TABLE planned_sessions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scheduled_on            DATE        NOT NULL,
    title                   TEXT        NOT NULL CONSTRAINT planned_sessions_title_valido
                                      CHECK (title <> '' AND length(title) <= 200),
    notes                   TEXT        CONSTRAINT planned_sessions_notes_valido
                                      CHECK (notes IS NULL OR length(notes) <= 2000),
    workout_type            TEXT        NOT NULL CONSTRAINT planned_sessions_workout_type_valido
                                      CHECK (workout_type IN ('rodaje', 'tirada_larga', 'series', 'cuestas',
                                                             'tempo', 'fartlek', 'fuerza', 'recuperacion', 'otro')),
    target_duration_sec     INTEGER     CONSTRAINT planned_sessions_target_duration_no_negativo
                                      CHECK (target_duration_sec IS NULL OR target_duration_sec >= 0),
    target_distance_m       INTEGER     CONSTRAINT planned_sessions_target_distance_no_negativo
                                      CHECK (target_distance_m IS NULL OR target_distance_m >= 0),
    target_elevation_gain_m INTEGER     CONSTRAINT planned_sessions_target_elevation_no_negativo
                                      CHECK (target_elevation_gain_m IS NULL OR target_elevation_gain_m >= 0),
    target_intensity        TEXT        CONSTRAINT planned_sessions_target_intensity_valido
                                      CHECK (target_intensity IS NULL OR target_intensity IN ('z1', 'z2', 'z3', 'z4', 'z5')),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Lectura principal: las sesiones del usuario en una ventana de fechas.
CREATE INDEX idx_planned_sessions_user_scheduled ON planned_sessions (user_id, scheduled_on);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- El rollback elimina la tabla completa: las sesiones planificadas son
-- datos de planificacion que el usuario puede volver a crear, y no hay
-- ningun sistema externo que reconciliar.
DROP TABLE IF EXISTS planned_sessions;

-- +goose StatementEnd
