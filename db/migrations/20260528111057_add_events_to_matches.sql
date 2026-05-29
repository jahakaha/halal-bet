-- +goose Up
ALTER TABLE wc2026_matches
    ADD COLUMN group_name   TEXT,
    ADD COLUMN had_red_card BOOLEAN,
    ADD COLUMN had_penalty  BOOLEAN,
    ADD COLUMN had_own_goal BOOLEAN;

-- +goose Down
ALTER TABLE wc2026_matches
    DROP COLUMN group_name,
    DROP COLUMN had_red_card,
    DROP COLUMN had_penalty,
    DROP COLUMN had_own_goal;
