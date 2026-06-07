-- +goose Up
CREATE TABLE rp.parts (
  id                  BIGINT         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode               UUID           NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts          TIMESTAMPTZ    NOT NULL DEFAULT now(),
  created_by_user_id  BIGINT         NULL REFERENCES rp.users(id),
  voided_ts           TIMESTAMPTZ    NULL,
  voided_by_user_id   BIGINT         NULL REFERENCES rp.users(id),

  sku                 TEXT           NULL,
  name                TEXT           NOT NULL,
  description         TEXT           NULL,
  unit                TEXT           NOT NULL DEFAULT 'unidad' CHECK (length(unit) BETWEEN 1 AND 32),
  current_stock       NUMERIC(10, 2) NOT NULL DEFAULT 0,
  reorder_level       NUMERIC(10, 2) NULL CHECK (reorder_level IS NULL OR reorder_level >= 0),
  default_cost        NUMERIC(14, 2) NULL CHECK (default_cost IS NULL OR default_cost >= 0),
  default_sale_price  NUMERIC(14, 2) NULL CHECK (default_sale_price IS NULL OR default_sale_price >= 0)
);

CREATE UNIQUE INDEX parts_sku_unique ON rp.parts(sku) WHERE sku IS NOT NULL AND voided_ts IS NULL;
CREATE INDEX parts_name_lower_idx ON rp.parts(lower(name)) WHERE voided_ts IS NULL;
CREATE INDEX parts_low_stock_idx ON rp.parts(id) WHERE voided_ts IS NULL AND reorder_level IS NOT NULL AND current_stock < reorder_level;

CREATE TABLE rp.part_movements (
  id                  BIGINT         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode               UUID           NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts          TIMESTAMPTZ    NOT NULL DEFAULT now(),
  created_by_user_id  BIGINT         NULL REFERENCES rp.users(id),
  voided_ts           TIMESTAMPTZ    NULL,
  voided_by_user_id   BIGINT         NULL REFERENCES rp.users(id),

  part_id             BIGINT         NOT NULL REFERENCES rp.parts(id) ON DELETE RESTRICT,
  movement_type       TEXT           NOT NULL CHECK (movement_type IN ('purchase','use','adjustment','return')),
  quantity            NUMERIC(10, 2) NOT NULL CHECK (quantity <> 0),
  unit_cost           NUMERIC(14, 2) NULL CHECK (unit_cost IS NULL OR unit_cost >= 0),
  supplier_id         BIGINT         NULL REFERENCES rp.suppliers(id) ON DELETE RESTRICT,
  work_order_id       BIGINT         NULL REFERENCES rp.work_orders(id) ON DELETE RESTRICT,
  transaction_id      BIGINT         NULL REFERENCES rp.transactions(id) ON DELETE SET NULL,
  notes               TEXT           NULL
);

CREATE INDEX part_movements_part_created_idx ON rp.part_movements(part_id, created_ts DESC) WHERE voided_ts IS NULL;
CREATE INDEX part_movements_wo_idx ON rp.part_movements(work_order_id) WHERE work_order_id IS NOT NULL AND voided_ts IS NULL;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rp.recompute_part_stock(p_part_id BIGINT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
BEGIN
  -- Lock the parts row to serialize concurrent movements on the same part.
  PERFORM 1 FROM rp.parts WHERE id = p_part_id FOR UPDATE;
  UPDATE rp.parts
    SET current_stock = COALESCE((
      SELECT SUM(quantity)::numeric(10,2)
      FROM rp.part_movements
      WHERE part_id = p_part_id AND voided_ts IS NULL
    ), 0)
  WHERE id = p_part_id;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION rp.part_movements_after_change()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM rp.recompute_part_stock(NEW.part_id);
  ELSIF TG_OP = 'UPDATE' THEN
    PERFORM rp.recompute_part_stock(NEW.part_id);
    IF NEW.part_id <> OLD.part_id THEN
      PERFORM rp.recompute_part_stock(OLD.part_id);
    END IF;
  ELSIF TG_OP = 'DELETE' THEN
    PERFORM rp.recompute_part_stock(OLD.part_id);
  END IF;
  RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER part_movements_stock_sync
  AFTER INSERT OR UPDATE OR DELETE ON rp.part_movements
  FOR EACH ROW EXECUTE FUNCTION rp.part_movements_after_change();

CREATE TABLE rp.work_order_parts (
  id                  BIGINT         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode               UUID           NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts          TIMESTAMPTZ    NOT NULL DEFAULT now(),
  created_by_user_id  BIGINT         NULL REFERENCES rp.users(id),
  voided_ts           TIMESTAMPTZ    NULL,
  voided_by_user_id   BIGINT         NULL REFERENCES rp.users(id),

  work_order_id       BIGINT         NOT NULL REFERENCES rp.work_orders(id) ON DELETE RESTRICT,
  part_id             BIGINT         NOT NULL REFERENCES rp.parts(id) ON DELETE RESTRICT,
  quantity            NUMERIC(10, 2) NOT NULL CHECK (quantity > 0),
  unit_price_charged  NUMERIC(14, 2) NOT NULL CHECK (unit_price_charged >= 0),
  cost_unit           NUMERIC(14, 2) NULL CHECK (cost_unit IS NULL OR cost_unit >= 0),
  part_movement_id    BIGINT         NULL REFERENCES rp.part_movements(id) ON DELETE SET NULL
);

CREATE INDEX work_order_parts_wo_idx ON rp.work_order_parts(work_order_id) WHERE voided_ts IS NULL;

CREATE TABLE rp.attachments (
  id                   BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode                UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts           TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id   BIGINT       NULL REFERENCES rp.users(id),
  voided_ts            TIMESTAMPTZ  NULL,
  voided_by_user_id    BIGINT       NULL REFERENCES rp.users(id),

  work_order_id        BIGINT       NOT NULL REFERENCES rp.work_orders(id) ON DELETE RESTRICT,
  phase                TEXT         NOT NULL CHECK (phase IN ('intake','diagnosis','during_repair','delivery')),
  original_filename    TEXT         NOT NULL,
  mime_type            TEXT         NOT NULL CHECK (mime_type IN ('image/jpeg','image/png','image/webp')),
  size_bytes           BIGINT       NOT NULL CHECK (size_bytes > 0),
  storage_path         TEXT         NOT NULL UNIQUE,
  width                INT          NULL CHECK (width IS NULL OR width > 0),
  height               INT          NULL CHECK (height IS NULL OR height > 0),
  uploaded_by_user_id  BIGINT       NULL REFERENCES rp.users(id)
);

CREATE INDEX attachments_wo_idx ON rp.attachments(work_order_id, phase) WHERE voided_ts IS NULL;

-- +goose Down
DROP TABLE IF EXISTS rp.attachments;
DROP TABLE IF EXISTS rp.work_order_parts;
DROP TRIGGER IF EXISTS part_movements_stock_sync ON rp.part_movements;
DROP FUNCTION IF EXISTS rp.part_movements_after_change();
DROP TABLE IF EXISTS rp.part_movements;
DROP FUNCTION IF EXISTS rp.recompute_part_stock(BIGINT);
DROP TABLE IF EXISTS rp.parts;
