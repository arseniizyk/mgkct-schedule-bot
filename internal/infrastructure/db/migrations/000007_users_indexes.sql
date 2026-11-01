-- +goose Up
CREATE INDEX IF NOT EXISTS idx_users_group_id ON users(group_id);
CREATE INDEX IF NOT EXISTS idx_users_teacher_name ON users(teacher_name);
CREATE INDEX IF NOT EXISTS idx_users_notify_evening_group_id ON users(notify_evening, group_id);

-- +goose Down
DROP INDEX IF EXISTS idx_users_notify_evening_group_id;
DROP INDEX IF EXISTS idx_users_teacher_name;
DROP INDEX IF EXISTS idx_users_group_id;
