-- name: CreateClient :one
INSERT INTO rp.clients (
  name,
  phone,
  email,
  dni_cuit,
  address,
  notes,
  client_type,
  created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetClientByUcode :one
SELECT *
FROM rp.clients
WHERE ucode = $1
  AND voided_ts IS NULL;

-- name: GetClientByPhone :one
SELECT *
FROM rp.clients
WHERE phone = $1
  AND voided_ts IS NULL;

-- name: SearchClients :many
SELECT *
FROM rp.clients
WHERE voided_ts IS NULL
  AND (
    sqlc.arg(q)::text = ''
    OR search @@ plainto_tsquery('spanish', sqlc.arg(q)::text)
    OR lower(name) LIKE lower('%' || sqlc.arg(q)::text || '%')
    OR phone = sqlc.arg(q)::text
  )
ORDER BY name ASC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountClients :one
SELECT count(*)::bigint
FROM rp.clients
WHERE voided_ts IS NULL
  AND (
    sqlc.arg(q)::text = ''
    OR search @@ plainto_tsquery('spanish', sqlc.arg(q)::text)
    OR lower(name) LIKE lower('%' || sqlc.arg(q)::text || '%')
    OR phone = sqlc.arg(q)::text
  );

-- name: UpdateClient :one
UPDATE rp.clients
SET
  name = $2,
  phone = $3,
  email = $4,
  dni_cuit = $5,
  address = $6,
  notes = $7,
  client_type = $8
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteClient :exec
UPDATE rp.clients
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE id = $1
  AND voided_ts IS NULL;

-- name: ListClientDevices :many
SELECT *
FROM rp.devices
WHERE client_id = $1
  AND voided_ts IS NULL
ORDER BY created_ts DESC;
