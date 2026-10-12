-- +goose Up
ALTER TABLE users ADD COLUMN notify_evening INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE users DROP COLUMN notify_evening;
