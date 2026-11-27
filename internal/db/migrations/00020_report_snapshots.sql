-- +goose Up
CREATE TABLE report_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('week', 'month', 'block')),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    block_id UUID REFERENCES training_blocks(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT report_snapshots_period_range CHECK (period_end >= period_start),
    CONSTRAINT report_snapshots_block_id_matches_kind CHECK ((kind = 'block') = (block_id IS NOT NULL)),
    CONSTRAINT report_snapshots_user_kind_start UNIQUE (user_id, kind, period_start)
);

-- Listing por usuario y tipo, del periodo más reciente al más antiguo.
CREATE INDEX idx_report_snapshots_user_kind_start ON report_snapshots (user_id, kind, period_start DESC);

-- +goose Down
-- Only snapshots are lost; their blocks, races and activities survive rollback.
DROP TABLE report_snapshots;
