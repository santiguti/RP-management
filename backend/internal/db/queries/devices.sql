-- name: CreateDevice :one
INSERT INTO rp.devices (
  client_id,
  brand_id,
  model_id,
  article_type_id,
  serial_number,
  color,
  description,
  created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetDeviceByUcode :one
SELECT
  sqlc.embed(d),
  c.ucode AS client_ucode,
  b.ucode AS brand_ucode,
  dm.ucode AS model_ucode,
  at.ucode AS article_type_ucode
FROM rp.devices d
JOIN rp.clients c ON c.id = d.client_id
JOIN rp.brands b ON b.id = d.brand_id
LEFT JOIN rp.device_models dm ON dm.id = d.model_id
JOIN rp.article_types at ON at.id = d.article_type_id
WHERE d.ucode = $1
  AND d.voided_ts IS NULL;

-- name: UpdateDevice :one
UPDATE rp.devices
SET
  client_id = $2,
  brand_id = $3,
  model_id = $4,
  article_type_id = $5,
  serial_number = $6,
  color = $7,
  description = $8
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteDevice :exec
UPDATE rp.devices
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE id = $1
  AND voided_ts IS NULL;

-- name: SearchDevices :many
SELECT
  sqlc.embed(d),
  c.ucode AS client_ucode,
  b.ucode AS brand_ucode,
  dm.ucode AS model_ucode,
  at.ucode AS article_type_ucode
FROM rp.devices d
JOIN rp.clients c ON c.id = d.client_id
JOIN rp.brands b ON b.id = d.brand_id
LEFT JOIN rp.device_models dm ON dm.id = d.model_id
JOIN rp.article_types at ON at.id = d.article_type_id
WHERE d.voided_ts IS NULL
  AND (NOT sqlc.arg(has_client)::bool OR d.client_id = sqlc.arg(client_id)::bigint)
  AND (sqlc.arg(serial)::text = '' OR d.serial_number ILIKE sqlc.arg(serial)::text || '%')
ORDER BY d.created_ts DESC
LIMIT 50;
