-- +goose Up
ALTER TABLE users ADD COLUMN teacher_name TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN teacher_name;
