-- +goose Up
CREATE TABLE training_blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    race_id UUID REFERENCES races(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT training_blocks_date_range CHECK (end_date >= start_date)
);

CREATE INDEX idx_training_blocks_user_start ON training_blocks (user_id, start_date, end_date);

-- +goose Down
-- Only blocks are lost; races and sessions survive rollback.
DROP TABLE training_blocks;
