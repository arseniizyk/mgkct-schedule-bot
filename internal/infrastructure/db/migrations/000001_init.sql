-- +goose Up
CREATE TABLE IF NOT EXISTS schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    week TEXT UNIQUE NOT NULL,
    schedule TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_schedules_set_updated_at AFTER UPDATE ON schedules
BEGIN
    UPDATE schedules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS teacher_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    week TEXT NOT NULL,
    schedule TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, week)
);

CREATE INDEX IF NOT EXISTS idx_teacher_schedules_name ON teacher_schedules(name);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_teacher_schedules_set_updated_at AFTER UPDATE ON teacher_schedules
BEGIN
    UPDATE teacher_schedules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER UNIQUE NOT NULL,
    username TEXT,
    group_id INTEGER,
    teacher_name TEXT,
    state TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE IF EXISTS users;
DROP TRIGGER IF EXISTS trg_teacher_schedules_set_updated_at;
DROP INDEX IF EXISTS idx_teacher_schedules_name;
DROP TABLE IF EXISTS teacher_schedules;
DROP TRIGGER IF EXISTS trg_schedules_set_updated_at;
DROP TABLE IF EXISTS schedules;
