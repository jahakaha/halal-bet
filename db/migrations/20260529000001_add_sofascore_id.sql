-- +goose Up
ALTER TABLE wc2026_matches ADD COLUMN sofascore_id BIGINT;

-- +goose Down
ALTER TABLE wc2026_matches DROP COLUMN sofascore_id;
