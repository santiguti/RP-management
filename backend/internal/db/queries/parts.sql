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
