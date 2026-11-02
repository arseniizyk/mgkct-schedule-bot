-- +goose Up
CREATE INDEX IF NOT EXISTS idx_teacher_schedules_week ON teacher_schedules(week);

-- +goose Down
DROP INDEX IF EXISTS idx_teacher_schedules_week;
