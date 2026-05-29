-- +goose Up
CREATE TABLE predictions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id),
    match_id    BIGINT      NOT NULL REFERENCES wc2026_matches(id),
    home_score  INT         NOT NULL,
    away_score  INT         NOT NULL,
    double_down  BOOLEAN     NOT NULL DEFAULT false,
    bet_penalty  BOOLEAN     NOT NULL DEFAULT false,
    bet_red_card BOOLEAN     NOT NULL DEFAULT false,
    bet_own_goal BOOLEAN     NOT NULL DEFAULT false,
    points      INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, match_id)
);

CREATE INDEX idx_predictions_user_id  ON predictions (user_id);
CREATE INDEX idx_predictions_match_id ON predictions (match_id);

-- +goose Down
DROP TABLE predictions;
