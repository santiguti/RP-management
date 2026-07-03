-- name: CreateAuditEntry :one
INSERT INTO rp.audit_log (
  actor_user_id, action, entity_type, entity_id, entity_ucode,
  before_json, after_json, ip, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListAuditEntries :many
SELECT
  sqlc.embed(a),
  u.username  AS actor_username,
  u.full_name AS actor_full_name
FROM rp.audit_log a
LEFT JOIN rp.users u ON u.id = a.actor_user_id
WHERE
      (NOT sqlc.arg(has_actor)::bool   OR a.actor_user_id = sqlc.arg(actor_user_id)::bigint)
  AND (sqlc.arg(entity_type)::text = '' OR a.entity_type   = sqlc.arg(entity_type)::text)
  AND (NOT sqlc.arg(has_entity_ucode)::bool OR a.entity_ucode = sqlc.arg(entity_ucode)::uuid)
  AND (sqlc.arg(action)::text      = '' OR a.action        = sqlc.arg(action)::text)
  AND (NOT sqlc.arg(has_from)::bool OR a.created_ts >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_to)::bool   OR a.created_ts <= sqlc.arg(date_to)::timestamptz)
ORDER BY a.created_ts DESC, a.id DESC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountAuditEntries :one
SELECT count(*)::bigint
FROM rp.audit_log a
WHERE
      (NOT sqlc.arg(has_actor)::bool   OR a.actor_user_id = sqlc.arg(actor_user_id)::bigint)
  AND (sqlc.arg(entity_type)::text = '' OR a.entity_type   = sqlc.arg(entity_type)::text)
  AND (NOT sqlc.arg(has_entity_ucode)::bool OR a.entity_ucode = sqlc.arg(entity_ucode)::uuid)
  AND (sqlc.arg(action)::text      = '' OR a.action        = sqlc.arg(action)::text)
  AND (NOT sqlc.arg(has_from)::bool OR a.created_ts >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_to)::bool   OR a.created_ts <= sqlc.arg(date_to)::timestamptz);
