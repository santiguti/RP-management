-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rp.assign_wo_number()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  y         INT := EXTRACT(YEAR FROM now())::INT;
  next_seq  BIGINT;
BEGIN
  INSERT INTO rp.wo_number_counters (year, last_seq)
    VALUES (y, 1)
    ON CONFLICT (year) DO UPDATE SET last_seq = rp.wo_number_counters.last_seq + 1
    RETURNING last_seq INTO next_seq;

  NEW.wo_number := y::text || '-' || lpad(next_seq::text, 4, '0');
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rp.assign_wo_number()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  y         INT := EXTRACT(YEAR FROM now())::INT;
  next_seq  BIGINT;
BEGIN
  IF NEW.wo_number IS NOT NULL AND NEW.wo_number <> '' THEN
    RETURN NEW;
  END IF;

  INSERT INTO rp.wo_number_counters (year, last_seq)
    VALUES (y, 1)
    ON CONFLICT (year) DO UPDATE SET last_seq = rp.wo_number_counters.last_seq + 1
    RETURNING last_seq INTO next_seq;

  NEW.wo_number := y::text || '-' || lpad(next_seq::text, 4, '0');
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
