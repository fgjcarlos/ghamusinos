-- +goose Up
-- +goose StatementBegin

-- Fase 1.4 — Dashboard de rendimiento y salud/fatiga (issue #16).
-- Persistencia diaria de training load (TSS agregado + KPI) por usuario.
--
-- El TSS diario es la suma del TSS de cada actividad del día. CTL/ATL/TSB
-- se derivan en lectura (módulo internal/metrics PR1) aplicando la EMA con
-- TauCTL=42 / TauATL=7 sobre la serie rellenada con días vacíos=0. Aquí
-- solo persistimos la materia prima para que el cálculo sea reproducible.
--
-- TimescaleDB: convertimos en hypertable particionada por día con chunk
-- semanal para alinearse con la cadencia de recálculo. La creación de la
-- hypertable se envuelve en un bloque que tolera la ausencia de la extensión
-- (Postgres "vanilla" o CI con imagen no-timescale): la tabla plana sigue
-- siendo válida, simplemente no se gana la optimización temporal.
CREATE TABLE training_load_daily (
    user_id            UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day                DATE         NOT NULL,
    tss                DOUBLE PRECISION NOT NULL DEFAULT 0,
    activity_count     INTEGER      NOT NULL DEFAULT 0,
    distance_m         DOUBLE PRECISION NOT NULL DEFAULT 0,
    elevation_gain_m   DOUBLE PRECISION NOT NULL DEFAULT 0,
    computed_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, day)
);

CREATE INDEX training_load_daily_user_day_desc_idx
    ON training_load_daily (user_id, day DESC);

-- Tolerancia a ausencia de la extensión timescaledb. Si la función
-- create_hypertable no existe (Postgres vanilla), la tabla se queda como
-- tabla normal y se emite un NOTICE; las queries siguen funcionando.
DO $load$
BEGIN
    BEGIN
        PERFORM create_hypertable(
            'training_load_daily',
            'day',
            chunk_time_interval => INTERVAL '7 days',
            if_not_exists => TRUE
        );
    EXCEPTION
        WHEN undefined_function THEN
            RAISE NOTICE 'timescaledb absent, training_load_daily stays as a regular table';
    END;
END
$load$;

-- Metadata del último recálculo por usuario. La idea es que /healthz
-- extendido (PR3) lea `last_recalc_at` y `training_load_rows` directamente
-- sin improvisar; el job RecalcTrainingLoad mantiene esta fila.
CREATE TABLE dashboard_metadata (
    user_id              UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_recalc_at       TIMESTAMPTZ,
    last_recalc_status   TEXT,
    last_recalc_error    TEXT,
    training_load_rows   INTEGER      NOT NULL DEFAULT 0,
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS dashboard_metadata;
DROP INDEX IF EXISTS training_load_daily_user_day_desc_idx;
DROP TABLE IF EXISTS training_load_daily;

-- +goose StatementEnd