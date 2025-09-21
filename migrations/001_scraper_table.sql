CREATE TABLE schedules (
    id SERIAL PRIMARY KEY,
    week DATE UNIQUE NOT NULL,
    schedule JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_set_updated_at
BEFORE UPDATE ON schedules
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
