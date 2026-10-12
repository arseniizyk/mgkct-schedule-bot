-- +goose Up
CREATE TABLE IF NOT EXISTS faculty (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT NOT NULL,
    short_name TEXT NOT NULL,
    position TEXT NOT NULL DEFAULT '',
    qualification TEXT NOT NULL DEFAULT '',
    education TEXT NOT NULL DEFAULT '',
    experience TEXT NOT NULL DEFAULT '',
    department TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(full_name, department)
);

CREATE INDEX IF NOT EXISTS idx_faculty_short_name ON faculty(short_name);
CREATE INDEX IF NOT EXISTS idx_faculty_full_name ON faculty(full_name);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_faculty_set_updated_at AFTER UPDATE ON faculty
BEGIN
    UPDATE faculty SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_faculty_set_updated_at;
DROP INDEX IF EXISTS idx_faculty_full_name;
DROP INDEX IF EXISTS idx_faculty_short_name;
DROP TABLE IF EXISTS faculty;
