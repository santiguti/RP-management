-- name: ListBrands :many
SELECT *
FROM rp.brands
WHERE voided_ts IS NULL
ORDER BY name;

-- name: GetBrandByUcode :one
SELECT *
FROM rp.brands
WHERE ucode = $1
  AND voided_ts IS NULL;

-- name: CreateBrand :one
INSERT INTO rp.brands (name, created_by_user_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateBrand :one
UPDATE rp.brands
SET name = $2
WHERE ucode = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteBrand :exec
UPDATE rp.brands
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE ucode = $1
  AND voided_ts IS NULL;

-- name: ListDeviceModelsByBrand :many
SELECT sqlc.embed(dm), b.ucode AS brand_ucode
FROM rp.device_models dm
JOIN rp.brands b ON b.id = dm.brand_id
WHERE dm.brand_id = $1
  AND dm.voided_ts IS NULL
ORDER BY dm.name;

-- name: GetDeviceModelByUcode :one
SELECT sqlc.embed(dm), b.ucode AS brand_ucode
FROM rp.device_models dm
JOIN rp.brands b ON b.id = dm.brand_id
WHERE dm.ucode = $1
  AND dm.voided_ts IS NULL;

-- name: CreateDeviceModel :one
INSERT INTO rp.device_models (brand_id, name, created_by_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateDeviceModel :one
UPDATE rp.device_models
SET name = $2
WHERE ucode = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteDeviceModel :exec
UPDATE rp.device_models
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE ucode = $1
  AND voided_ts IS NULL;

-- name: ListArticleTypes :many
SELECT *
FROM rp.article_types
WHERE voided_ts IS NULL
ORDER BY name;

-- name: GetArticleTypeByUcode :one
SELECT *
FROM rp.article_types
WHERE ucode = $1
  AND voided_ts IS NULL;

-- name: CreateArticleType :one
INSERT INTO rp.article_types (name, created_by_user_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateArticleType :one
UPDATE rp.article_types
SET name = $2
WHERE ucode = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteArticleType :exec
UPDATE rp.article_types
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE ucode = $1
  AND voided_ts IS NULL;
