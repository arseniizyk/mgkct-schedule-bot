-- +goose Up
CREATE TABLE teacher_schedules (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    week DATE NOT NULL,
    schedule JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(name, week)
);

CREATE INDEX idx_teacher_schedules_name ON teacher_schedules(name);

CREATE TRIGGER trg_teacher_schedules_set_updated_at
BEFORE UPDATE ON teacher_schedules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_teacher_schedules_set_updated_at ON teacher_schedules;
DROP TABLE IF EXISTS teacher_schedules;
