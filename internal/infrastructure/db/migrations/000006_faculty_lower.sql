-- +goose Up
ALTER TABLE faculty ADD COLUMN full_name_lower TEXT NOT NULL DEFAULT '';
ALTER TABLE faculty ADD COLUMN short_name_lower TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_faculty_full_name_lower ON faculty(full_name_lower);
CREATE INDEX IF NOT EXISTS idx_faculty_short_name_lower ON faculty(short_name_lower);

-- Recreate trigger with loop-protection for existing databases
DROP TRIGGER IF EXISTS trg_faculty_set_updated_at;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_faculty_set_updated_at AFTER UPDATE ON faculty
WHEN OLD.updated_at = NEW.updated_at
BEGIN
    UPDATE faculty SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_faculty_set_updated_at;
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_faculty_set_updated_at AFTER UPDATE ON faculty
BEGIN
    UPDATE faculty SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd
DROP INDEX IF EXISTS idx_faculty_short_name_lower;
DROP INDEX IF EXISTS idx_faculty_full_name_lower;
ALTER TABLE faculty DROP COLUMN short_name_lower;
ALTER TABLE faculty DROP COLUMN full_name_lower;
