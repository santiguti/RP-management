-- name: CreateAttachment :one
INSERT INTO rp.attachments (
  work_order_id, phase, original_filename, mime_type, size_bytes,
  storage_path, width, height, uploaded_by_user_id, created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING *;

-- name: ListWorkOrderAttachments :many
SELECT * FROM rp.attachments
WHERE work_order_id = $1 AND voided_ts IS NULL
ORDER BY created_ts ASC, id ASC;

-- name: GetAttachmentByUcode :one
SELECT * FROM rp.attachments
WHERE ucode = $1 AND voided_ts IS NULL;

-- name: SoftDeleteAttachment :exec
UPDATE rp.attachments SET voided_ts = now(), voided_by_user_id = $2
WHERE id = $1 AND voided_ts IS NULL;
