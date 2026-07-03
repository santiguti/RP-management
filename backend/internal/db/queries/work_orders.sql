-- name: CreateWorkOrder :one
INSERT INTO rp.work_orders (
  client_id,
  device_id,
  service_type,
  reported_issue,
  intake_notes,
  accessories,
  device_pin_encrypted,
  created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetWorkOrderByUcode :one
SELECT
  sqlc.embed(wo),
  c.ucode  AS client_ucode,
  c.name   AS client_name,
  c.phone  AS client_phone,
  d.ucode  AS device_ucode,
  b.ucode  AS brand_ucode,
  b.name   AS brand_name,
  dm.ucode AS model_ucode,
  dm.name  AS model_name,
  at.ucode AS article_type_ucode,
  at.name  AS article_type_name,
  d.serial_number AS device_serial
FROM rp.work_orders wo
JOIN rp.clients c       ON c.id = wo.client_id
JOIN rp.devices d       ON d.id = wo.device_id
JOIN rp.brands b        ON b.id = d.brand_id
LEFT JOIN rp.device_models dm ON dm.id = d.model_id
JOIN rp.article_types at      ON at.id = d.article_type_id
WHERE wo.ucode = $1
  AND wo.voided_ts IS NULL;

-- name: GetWorkOrderByNumber :one
SELECT *
FROM rp.work_orders
WHERE wo_number = $1
  AND voided_ts IS NULL;

-- name: ListWorkOrders :many
SELECT
  sqlc.embed(wo),
  c.ucode AS client_ucode,
  c.name AS client_name,
  c.phone AS client_phone,
  d.ucode AS device_ucode,
  b.name AS brand_name,
  dm.name AS model_name,
  at.name AS article_type_name
FROM rp.work_orders wo
JOIN rp.clients c ON c.id = wo.client_id
JOIN rp.devices d ON d.id = wo.device_id
JOIN rp.brands b ON b.id = d.brand_id
LEFT JOIN rp.device_models dm ON dm.id = d.model_id
JOIN rp.article_types at ON at.id = d.article_type_id
WHERE wo.voided_ts IS NULL
  AND (sqlc.arg(status)::text = '' OR wo.status = sqlc.arg(status)::text)
  AND (NOT sqlc.arg(has_client)::bool OR wo.client_id = sqlc.arg(client_id)::bigint)
  AND (
    sqlc.arg(q)::text = ''
    OR wo.wo_number ILIKE '%' || sqlc.arg(q)::text || '%'
    OR c.name ILIKE '%' || sqlc.arg(q)::text || '%'
    OR c.phone = sqlc.arg(q)::text
    OR d.serial_number ILIKE sqlc.arg(q)::text || '%'
  )
ORDER BY wo.received_ts DESC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountWorkOrders :one
SELECT count(*)::bigint
FROM rp.work_orders wo
JOIN rp.clients c ON c.id = wo.client_id
JOIN rp.devices d ON d.id = wo.device_id
JOIN rp.brands b ON b.id = d.brand_id
LEFT JOIN rp.device_models dm ON dm.id = d.model_id
JOIN rp.article_types at ON at.id = d.article_type_id
WHERE wo.voided_ts IS NULL
  AND (sqlc.arg(status)::text = '' OR wo.status = sqlc.arg(status)::text)
  AND (NOT sqlc.arg(has_client)::bool OR wo.client_id = sqlc.arg(client_id)::bigint)
  AND (
    sqlc.arg(q)::text = ''
    OR wo.wo_number ILIKE '%' || sqlc.arg(q)::text || '%'
    OR c.name ILIKE '%' || sqlc.arg(q)::text || '%'
    OR c.phone = sqlc.arg(q)::text
    OR d.serial_number ILIKE sqlc.arg(q)::text || '%'
  );

-- name: UpdateWorkOrderFields :one
UPDATE rp.work_orders
SET
  service_type = $2,
  reported_issue = $3,
  diagnosis = $4,
  intake_notes = $5,
  accessories = $6,
  device_pin_encrypted = $7
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: UpdateWorkOrderStatus :one
UPDATE rp.work_orders
SET
  status = $2,
  started_ts = COALESCE(started_ts, CASE WHEN $2 = 'in_repair' THEN now() END),
  ready_ts = COALESCE(ready_ts, CASE WHEN $2 = 'ready' THEN now() END),
  delivered_ts = COALESCE(delivered_ts, CASE WHEN $2 = 'delivered' THEN now() END),
  cancelled_ts = COALESCE(cancelled_ts, CASE WHEN $2 = 'cancelled' THEN now() END),
  cancel_reason = COALESCE($3, cancel_reason)
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SetWorkOrderQuote :one
UPDATE rp.work_orders
SET
  status = 'quoted',
  diagnosis = COALESCE($2, diagnosis),
  quote_amount = $3,
  quote_currency = COALESCE(sqlc.narg(quote_currency)::text, quote_currency)::char(3),
  quote_sent_ts = now()
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SetWorkOrderQuoteOutcome :one
UPDATE rp.work_orders
SET
  status = $2,
  quote_approved_ts = CASE WHEN $2 = 'approved' THEN now() ELSE quote_approved_ts END,
  quote_rejected_ts = CASE WHEN $2 = 'rejected' THEN now() ELSE quote_rejected_ts END
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SetWorkOrderFinals :one
UPDATE rp.work_orders
SET
  status = 'ready',
  diagnosis = COALESCE($2, diagnosis),
  labor_amount = COALESCE($3, labor_amount),
  parts_amount = COALESCE($4, parts_amount),
  final_amount = COALESCE($5, final_amount),
  ready_ts = COALESCE(ready_ts, now())
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteWorkOrder :exec
UPDATE rp.work_orders
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE id = $1
  AND voided_ts IS NULL;

-- name: ListWorkOrderParts :many
SELECT
  sqlc.embed(wop),
  p.ucode AS part_ucode,
  p.name AS part_name,
  p.unit AS part_unit
FROM rp.work_order_parts wop
JOIN rp.parts p ON p.id = wop.part_id
WHERE wop.work_order_id = $1
  AND wop.voided_ts IS NULL
ORDER BY wop.created_ts ASC, wop.id ASC;

-- name: GetWorkOrderPartByID :one
SELECT * FROM rp.work_order_parts
WHERE id = $1 AND voided_ts IS NULL;

-- name: CreateWorkOrderPart :one
INSERT INTO rp.work_order_parts (
  work_order_id, part_id, quantity, unit_price_charged,
  cost_unit, part_movement_id, created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: SoftDeleteWorkOrderPart :exec
UPDATE rp.work_order_parts SET voided_ts = now(), voided_by_user_id = $2
WHERE id = $1 AND voided_ts IS NULL;

-- name: RecomputeWorkOrderPartsAmount :exec
UPDATE rp.work_orders SET parts_amount = COALESCE((
  SELECT SUM(quantity * unit_price_charged)::numeric(14,2)
  FROM rp.work_order_parts
  WHERE work_order_id = $1 AND voided_ts IS NULL
), 0)
WHERE id = $1;
