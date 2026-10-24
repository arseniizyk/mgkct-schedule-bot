-- +goose Up
ALTER TABLE users ADD COLUMN subgroup INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN subgroup;
