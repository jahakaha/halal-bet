-- +goose Up
CREATE TABLE tournament_predictions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(20) NOT NULL CHECK (type IN ('winner', 'top_scorer')),
    value      TEXT        NOT NULL,
    points     INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, type)
);

-- +goose Down
DROP TABLE tournament_predictions;
