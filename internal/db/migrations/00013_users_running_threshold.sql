-- +goose Up
-- +goose StatementBegin

-- Fase 1.4 — Dashboard de rendimiento y salud/fatiga (issue #16, G1).
-- Umbral de ritmo de carrera en segundos por kilómetro (running pace
-- threshold). Necesario para que TSSRunning tenga un dominio válido
-- (PR1 acepta el parámetro; PR3 lo leerá desde aquí).
--
-- Rango 120..1800 (2 min/km..30 min/km). NULL significa "sin umbral
-- definido": PR2 omite la actividad running del TSS diario; PR3 puede
-- mostrar un mensaje "configura running_threshold_sec_per_km" en el UI.
ALTER TABLE users
    ADD COLUMN running_threshold_sec_per_km SMALLINT NULL
        CONSTRAINT users_running_threshold_chk
        CHECK (running_threshold_sec_per_km IS NULL
               OR (running_threshold_sec_per_km BETWEEN 120 AND 1800));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_running_threshold_chk;
ALTER TABLE users DROP COLUMN IF EXISTS running_threshold_sec_per_km;

-- +goose StatementEnd