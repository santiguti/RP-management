-- name: CreatePart :one
INSERT INTO rp.parts (sku, name, description, unit, reorder_level, default_cost, default_sale_price, created_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPartByUcode :one
SELECT * FROM rp.parts WHERE ucode = $1 AND voided_ts IS NULL;

-- name: GetPartByID :one
SELECT * FROM rp.parts WHERE id = $1 AND voided_ts IS NULL;

-- name: SearchParts :many
SELECT *
FROM rp.parts
WHERE voided_ts IS NULL
  AND (sqlc.arg(q)::text = ''
       OR lower(name) LIKE lower('%' || sqlc.arg(q)::text || '%')
       OR (sku IS NOT NULL AND lower(sku) LIKE lower(sqlc.arg(q)::text || '%')))
  AND (NOT sqlc.arg(low_stock)::bool
       OR (reorder_level IS NOT NULL AND current_stock < reorder_level))
ORDER BY name ASC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountParts :one
SELECT count(*)::bigint
FROM rp.parts
WHERE voided_ts IS NULL
  AND (sqlc.arg(q)::text = ''
       OR lower(name) LIKE lower('%' || sqlc.arg(q)::text || '%')
       OR (sku IS NOT NULL AND lower(sku) LIKE lower(sqlc.arg(q)::text || '%')))
  AND (NOT sqlc.arg(low_stock)::bool
       OR (reorder_level IS NOT NULL AND current_stock < reorder_level));

-- name: UpdatePart :one
UPDATE rp.parts SET
  sku = $2, name = $3, description = $4, unit = $5,
  reorder_level = $6, default_cost = $7, default_sale_price = $8
WHERE id = $1 AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeletePart :exec
UPDATE rp.parts SET voided_ts = now(), voided_by_user_id = $2
WHERE id = $1 AND voided_ts IS NULL;

-- name: CreatePartMovement :one
INSERT INTO rp.part_movements (
  part_id, movement_type, quantity, unit_cost,
  supplier_id, work_order_id, transaction_id, notes, created_by_user_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPartMovementByUcode :one
SELECT
  sqlc.embed(m),
  s.ucode AS supplier_ucode, s.name AS supplier_name,
  wo.ucode AS work_order_ucode, wo.wo_number AS work_order_number,
  t.ucode AS transaction_ucode
FROM rp.part_movements m
LEFT JOIN rp.suppliers s ON s.id = m.supplier_id
LEFT JOIN rp.work_orders wo ON wo.id = m.work_order_id
LEFT JOIN rp.transactions t ON t.id = m.transaction_id
WHERE m.ucode = $1
  AND m.voided_ts IS NULL;

-- name: ListPartMovements :many
SELECT
  sqlc.embed(m),
  s.ucode AS supplier_ucode, s.name AS supplier_name,
  wo.ucode AS work_order_ucode, wo.wo_number AS work_order_number,
  t.ucode AS transaction_ucode
FROM rp.part_movements m
LEFT JOIN rp.suppliers s ON s.id = m.supplier_id
LEFT JOIN rp.work_orders wo ON wo.id = m.work_order_id
LEFT JOIN rp.transactions t ON t.id = m.transaction_id
WHERE m.part_id = $1
  AND m.voided_ts IS NULL
ORDER BY m.created_ts DESC, m.id DESC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountPartMovements :one
SELECT count(*)::bigint
FROM rp.part_movements
WHERE part_id = $1 AND voided_ts IS NULL;

-- name: SoftDeletePartMovement :exec
UPDATE rp.part_movements SET voided_ts = now(), voided_by_user_id = $2
WHERE id = $1 AND voided_ts IS NULL;
