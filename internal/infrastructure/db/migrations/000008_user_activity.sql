-- +goose Up
ALTER TABLE users ADD COLUMN last_activity_at DATETIME;
CREATE INDEX IF NOT EXISTS idx_users_last_activity_at ON users(last_activity_at);

-- +goose Down
DROP INDEX IF EXISTS idx_users_last_activity_at;
ALTER TABLE users DROP COLUMN last_activity_at;
