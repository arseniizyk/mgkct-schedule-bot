-- +goose Up
CREATE TABLE schedules (
    id SERIAL PRIMARY KEY,
    week DATE UNIQUE NOT NULL,
    schedule JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_set_updated_at
BEFORE UPDATE ON schedules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_set_updated_at ON schedules;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS schedules;
