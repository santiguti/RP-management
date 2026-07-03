-- name: ListSuppliers :many
SELECT *
FROM rp.suppliers
WHERE voided_ts IS NULL
ORDER BY name ASC;

-- name: GetSupplierByUcode :one
SELECT *
FROM rp.suppliers
WHERE ucode = $1
  AND voided_ts IS NULL;

-- name: GetSupplierByName :one
SELECT *
FROM rp.suppliers
WHERE lower(name) = lower($1)
  AND voided_ts IS NULL;

-- name: CreateSupplier :one
INSERT INTO rp.suppliers (
  name,
  phone,
  email,
  notes,
  created_by_user_id
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateSupplier :one
UPDATE rp.suppliers
SET
  name = $2,
  phone = $3,
  email = $4,
  notes = $5
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteSupplier :exec
UPDATE rp.suppliers
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE id = $1
  AND voided_ts IS NULL;
