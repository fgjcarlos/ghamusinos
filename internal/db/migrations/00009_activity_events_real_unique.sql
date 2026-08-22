-- +goose Up
-- +goose StatementBegin

-- Strava webhook events do not contain external_id. Their stable natural key is
-- the object, change type, and event timestamp supplied by Strava.
ALTER TABLE activity_events
    DROP CONSTRAINT activity_events_external_id_key,
    DROP COLUMN external_id,
    ALTER COLUMN event_time SET NOT NULL,
    ADD CONSTRAINT activity_events_strava_event_key
        UNIQUE (object_id, aspect_type, event_time);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE activity_events
    DROP CONSTRAINT activity_events_strava_event_key,
    ADD COLUMN external_id TEXT;

-- The removed external IDs cannot be recovered. Use each row's UUID to restore
-- a deterministic unique value so the previous schema can be reinstated safely.
UPDATE activity_events
SET external_id = 'restored:' || id::text;

ALTER TABLE activity_events
    ALTER COLUMN external_id SET NOT NULL,
    ADD CONSTRAINT activity_events_external_id_key UNIQUE (external_id),
    ALTER COLUMN event_time DROP NOT NULL;

-- +goose StatementEnd
