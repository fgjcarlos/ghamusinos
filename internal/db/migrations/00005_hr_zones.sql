-- +goose Up
-- +goose StatementBegin

-- Zonas de entrenamiento de frecuencia cardíaca (HR zones).
-- Calculadas a partir de streams HR de Strava usando el método Friel/Coggan
-- (5 zonas a 50/60/70/80/90% del hrMax del usuario).
-- Una fila por actividad (FK → activities).
CREATE TABLE hr_zones (
    activity_id     UUID        PRIMARY KEY REFERENCES activities(id) ON DELETE CASCADE,
    z1_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z1_seconds >= 0),
    z2_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z2_seconds >= 0),
    z3_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z3_seconds >= 0),
    z4_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z4_seconds >= 0),
    z5_seconds      INTEGER     NOT NULL DEFAULT 0 CHECK (z5_seconds >= 0),
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS hr_zones;

-- +goose StatementEnd
