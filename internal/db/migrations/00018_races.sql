-- +goose Up
CREATE TABLE races (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    race_date DATE NOT NULL,
    distance_m INTEGER NOT NULL CHECK (distance_m > 0),
    elevation_gain_m INTEGER NOT NULL CHECK (elevation_gain_m >= 0),
    priority TEXT NOT NULL CHECK (priority IN ('A', 'B', 'C')),
    notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_races_user_date_id ON races (user_id, race_date, id);

CREATE TABLE race_tracks (
    race_id UUID NOT NULL REFERENCES races(id) ON DELETE CASCADE,
    gpx_track_id UUID NOT NULL REFERENCES gpx_tracks(id) ON DELETE CASCADE,
    PRIMARY KEY (race_id, gpx_track_id)
);

CREATE INDEX idx_race_tracks_gpx_track_id ON race_tracks (gpx_track_id);

ALTER TABLE planned_sessions ADD COLUMN race_id UUID
    CONSTRAINT planned_sessions_race_fk REFERENCES races(id) ON DELETE SET NULL;
CREATE INDEX idx_planned_sessions_race_id ON planned_sessions (race_id);

-- +goose Down
-- Sessions and GPX tracks survive rollback; only race data and links are lost.
ALTER TABLE planned_sessions DROP COLUMN race_id;
DROP TABLE race_tracks;
DROP TABLE races;
