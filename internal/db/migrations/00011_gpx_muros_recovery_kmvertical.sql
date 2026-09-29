-- +goose Up
-- +goose StatementBegin
CREATE TABLE gpx_muros (
    track_id UUID NOT NULL REFERENCES gpx_tracks(id) ON DELETE CASCADE,
    start_idx INTEGER NOT NULL,
    end_idx INTEGER NOT NULL,
    gain_m NUMERIC NOT NULL,
    distance_m NUMERIC NOT NULL,
    avg_slope_pct NUMERIC NOT NULL
);
CREATE INDEX idx_gpx_muros_track ON gpx_muros (track_id);

CREATE TABLE gpx_recovery_zones (
    track_id UUID NOT NULL REFERENCES gpx_tracks(id) ON DELETE CASCADE,
    start_idx INTEGER NOT NULL,
    end_idx INTEGER NOT NULL,
    distance_m NUMERIC NOT NULL
);
CREATE INDEX idx_gpx_recovery_zones_track ON gpx_recovery_zones (track_id);

CREATE TABLE gpx_km_vertical (
    track_id UUID NOT NULL UNIQUE REFERENCES gpx_tracks(id) ON DELETE CASCADE,
    start_idx INTEGER NOT NULL,
    end_idx INTEGER NOT NULL,
    gain_m NUMERIC NOT NULL,
    distance_m NUMERIC NOT NULL
);
CREATE INDEX idx_gpx_km_vertical_track ON gpx_km_vertical (track_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE gpx_km_vertical;
DROP TABLE gpx_recovery_zones;
DROP TABLE gpx_muros;
-- +goose StatementEnd
