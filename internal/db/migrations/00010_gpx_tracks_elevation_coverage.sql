-- +goose Up
-- +goose StatementBegin

-- Issue #171, A8: el desnivel positivo/negativo pasa a admitir NULL
-- porque un track con cobertura de elevación insuficiente no debe
-- publicar un número (un cero sería una mentira sobre los datos del
-- usuario). Se añade también elevation_coverage NUMERIC para que la
-- API pueda comunicar la fracción del track con elevación utilizable
-- (issue #160/#158/#126 — el frontend sustituye su heurística por
-- este campo).
ALTER TABLE gpx_tracks ALTER COLUMN d_plus_m DROP NOT NULL;
ALTER TABLE gpx_tracks ALTER COLUMN d_minus_m DROP NOT NULL;
ALTER TABLE gpx_tracks ADD COLUMN elevation_coverage NUMERIC;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- El Down tiene que backfill a 0 los NULL antes de volver a NOT NULL
-- porque ALTER ... SET NOT NULL falla si hay NULLs. Mantener este
-- paso en orden: primero datos, luego la restricción.
UPDATE gpx_tracks SET d_plus_m = 0  WHERE d_plus_m  IS NULL;
UPDATE gpx_tracks SET d_minus_m = 0 WHERE d_minus_m IS NULL;
ALTER TABLE gpx_tracks DROP COLUMN elevation_coverage;
ALTER TABLE gpx_tracks ALTER COLUMN d_plus_m SET NOT NULL;
ALTER TABLE gpx_tracks ALTER COLUMN d_minus_m SET NOT NULL;

-- +goose StatementEnd
