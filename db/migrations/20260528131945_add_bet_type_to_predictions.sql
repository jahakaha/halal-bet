-- +goose Up
ALTER TABLE predictions
    ADD COLUMN bet_type TEXT NOT NULL DEFAULT 'exact';

-- +goose Down
ALTER TABLE predictions
    DROP COLUMN bet_type;
