-- +goose Up
ALTER TABLE rp.parts
  ADD CONSTRAINT parts_current_stock_nonnegative CHECK (current_stock >= 0);

-- +goose Down
ALTER TABLE rp.parts
  DROP CONSTRAINT IF EXISTS parts_current_stock_nonnegative;
