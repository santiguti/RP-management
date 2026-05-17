-- +goose Up
CREATE TABLE rp.wo_number_counters (
  year      INT     PRIMARY KEY,
  last_seq  BIGINT  NOT NULL DEFAULT 0
);

CREATE TABLE rp.work_orders (
  id                       BIGINT         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode                    UUID           NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts               TIMESTAMPTZ    NOT NULL DEFAULT now(),
  created_by_user_id       BIGINT         NULL REFERENCES rp.users(id),
  voided_ts                TIMESTAMPTZ    NULL,
  voided_by_user_id        BIGINT         NULL REFERENCES rp.users(id),

  wo_number                TEXT           NOT NULL UNIQUE,
  device_id                BIGINT         NOT NULL REFERENCES rp.devices(id) ON DELETE RESTRICT,
  client_id                BIGINT         NOT NULL REFERENCES rp.clients(id) ON DELETE RESTRICT,
  service_type             TEXT           NOT NULL CHECK (service_type IN ('in_shop', 'on_site')),
  status                   TEXT           NOT NULL DEFAULT 'received' CHECK (status IN ('received', 'diagnosing', 'quoted', 'approved', 'rejected', 'in_repair', 'waiting_parts', 'ready', 'delivered', 'cancelled')),
  reported_issue           TEXT           NOT NULL,
  diagnosis                TEXT           NULL,
  quote_amount             NUMERIC(14, 2) NULL,
  quote_currency           CHAR(3)        NOT NULL DEFAULT 'ARS',
  quote_sent_ts            TIMESTAMPTZ    NULL,
  quote_approved_ts        TIMESTAMPTZ    NULL,
  quote_rejected_ts        TIMESTAMPTZ    NULL,
  final_amount             NUMERIC(14, 2) NULL,
  labor_amount             NUMERIC(14, 2) NULL,
  parts_amount             NUMERIC(14, 2) NULL,
  intake_notes             TEXT           NULL,
  accessories              TEXT           NULL,
  device_pin_encrypted     TEXT           NULL,
  received_ts              TIMESTAMPTZ    NOT NULL DEFAULT now(),
  started_ts               TIMESTAMPTZ    NULL,
  ready_ts                 TIMESTAMPTZ    NULL,
  delivered_ts             TIMESTAMPTZ    NULL,
  cancelled_ts             TIMESTAMPTZ    NULL,
  cancel_reason            TEXT           NULL
);

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

CREATE TRIGGER work_orders_wo_number
  BEFORE INSERT ON rp.work_orders
  FOR EACH ROW EXECUTE FUNCTION rp.assign_wo_number();

CREATE INDEX work_orders_status_idx ON rp.work_orders(status) WHERE voided_ts IS NULL;
CREATE INDEX work_orders_client_recv_idx ON rp.work_orders(client_id, received_ts DESC) WHERE voided_ts IS NULL;
CREATE INDEX work_orders_device_idx ON rp.work_orders(device_id) WHERE voided_ts IS NULL;

-- +goose Down
DROP TRIGGER IF EXISTS work_orders_wo_number ON rp.work_orders;
DROP FUNCTION IF EXISTS rp.assign_wo_number();
DROP TABLE IF EXISTS rp.work_orders;
DROP TABLE IF EXISTS rp.wo_number_counters;
