-- name: GetUserByUsername :one
SELECT *
FROM rp.users
WHERE username = $1
  AND voided_ts IS NULL;

-- name: GetUserByID :one
SELECT *
FROM rp.users
WHERE id = $1
  AND voided_ts IS NULL;

-- name: CreateUser :one
INSERT INTO rp.users (username, password_hash, full_name, role, created_by_user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateUserLastLogin :exec
UPDATE rp.users
SET last_login_ts = now()
WHERE id = $1;
