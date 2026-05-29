-- +goose Up
CREATE TABLE wc2026_matches (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    external_id BIGINT      NOT NULL UNIQUE,
    home_team   TEXT        NOT NULL,
    away_team   TEXT        NOT NULL,
    match_date  TIMESTAMPTZ NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'TIMED',
    home_score  INT,
    away_score  INT,
    stage       TEXT        NOT NULL DEFAULT '',
    matchday    INT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_wc2026_matches_date ON wc2026_matches (match_date);
CREATE INDEX idx_wc2026_matches_status ON wc2026_matches (status);

-- +goose Down
DROP TABLE wc2026_matches;
