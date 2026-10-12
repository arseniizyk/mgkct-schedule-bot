-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE schedules (
    id SERIAL PRIMARY KEY,
    week DATE UNIQUE NOT NULL,
    schedule JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TRIGGER trg_schedules_set_updated_at
BEFORE UPDATE ON schedules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

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

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL,
    username TEXT,
    group_id INT,
    teacher_name TEXT,
    state TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS users;
DROP TRIGGER IF EXISTS trg_teacher_schedules_set_updated_at ON teacher_schedules;
DROP INDEX IF EXISTS idx_teacher_schedules_name;
DROP TABLE IF EXISTS teacher_schedules;
DROP TRIGGER IF EXISTS trg_schedules_set_updated_at ON schedules;
DROP TABLE IF EXISTS schedules;
DROP FUNCTION IF EXISTS set_updated_at();
