-- Transaction financial identity is immutable after creation: transaction_type,
-- amount, currency, counterparty_type, and relation FKs are not patched. If one
-- of those is wrong, soft-delete the row and create a replacement.

-- name: CreateTransaction :one
INSERT INTO rp.transactions (
  transaction_type,
  amount,
  currency,
  fx_rate_to_ars,
  transaction_date,
  payment_method,
  category,
  counterparty_type,
  client_id,
  supplier_id,
  work_order_id,
  description,
  recurring_expense_id,
  created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetTransactionByUcode :one
SELECT
  sqlc.embed(t),
  c.ucode  AS client_ucode, c.name AS client_name,
  s.ucode  AS supplier_ucode, s.name AS supplier_name,
  wo.ucode AS work_order_ucode, wo.wo_number AS work_order_number,
  re.ucode AS recurring_expense_ucode, re.name AS recurring_expense_name
FROM rp.transactions t
LEFT JOIN rp.clients c ON c.id = t.client_id
LEFT JOIN rp.suppliers s ON s.id = t.supplier_id
LEFT JOIN rp.work_orders wo ON wo.id = t.work_order_id
LEFT JOIN rp.recurring_expenses re ON re.id = t.recurring_expense_id
WHERE t.ucode = $1
  AND t.voided_ts IS NULL;

-- name: ListTransactions :many
SELECT
  sqlc.embed(t),
  c.ucode AS client_ucode, c.name AS client_name,
  s.ucode AS supplier_ucode, s.name AS supplier_name,
  wo.ucode AS work_order_ucode, wo.wo_number AS work_order_number,
  re.ucode AS recurring_expense_ucode, re.name AS recurring_expense_name
FROM rp.transactions t
LEFT JOIN rp.clients c ON c.id = t.client_id
LEFT JOIN rp.suppliers s ON s.id = t.supplier_id
LEFT JOIN rp.work_orders wo ON wo.id = t.work_order_id
LEFT JOIN rp.recurring_expenses re ON re.id = t.recurring_expense_id
WHERE t.voided_ts IS NULL
  AND (NOT sqlc.arg(has_from)::bool OR t.transaction_date >= sqlc.arg(date_from)::date)
  AND (NOT sqlc.arg(has_to)::bool OR t.transaction_date <= sqlc.arg(date_to)::date)
  AND (sqlc.arg(transaction_type)::text = '' OR t.transaction_type = sqlc.arg(transaction_type)::text)
  AND (sqlc.arg(category)::text = '' OR t.category = sqlc.arg(category)::text)
  AND (NOT sqlc.arg(has_work_order)::bool OR t.work_order_id = sqlc.arg(work_order_id)::bigint)
ORDER BY t.transaction_date DESC, t.id DESC
LIMIT sqlc.arg(page_size)::int OFFSET sqlc.arg(page_offset)::int;

-- name: CountTransactions :one
SELECT count(*)::bigint
FROM rp.transactions t
WHERE t.voided_ts IS NULL
  AND (NOT sqlc.arg(has_from)::bool OR t.transaction_date >= sqlc.arg(date_from)::date)
  AND (NOT sqlc.arg(has_to)::bool OR t.transaction_date <= sqlc.arg(date_to)::date)
  AND (sqlc.arg(transaction_type)::text = '' OR t.transaction_type = sqlc.arg(transaction_type)::text)
  AND (sqlc.arg(category)::text = '' OR t.category = sqlc.arg(category)::text)
  AND (NOT sqlc.arg(has_work_order)::bool OR t.work_order_id = sqlc.arg(work_order_id)::bigint);

-- name: UpdateTransaction :one
UPDATE rp.transactions
SET
  transaction_date = $2,
  payment_method = $3,
  category = $4,
  description = $5
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteTransaction :exec
UPDATE rp.transactions
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE id = $1
  AND voided_ts IS NULL;

-- name: ListWorkOrderTransactions :many
SELECT
  sqlc.embed(t),
  c.ucode AS client_ucode, c.name AS client_name,
  s.ucode AS supplier_ucode, s.name AS supplier_name,
  wo.ucode AS work_order_ucode, wo.wo_number AS work_order_number,
  re.ucode AS recurring_expense_ucode, re.name AS recurring_expense_name
FROM rp.transactions t
LEFT JOIN rp.clients c ON c.id = t.client_id
LEFT JOIN rp.suppliers s ON s.id = t.supplier_id
LEFT JOIN rp.work_orders wo ON wo.id = t.work_order_id
LEFT JOIN rp.recurring_expenses re ON re.id = t.recurring_expense_id
WHERE t.work_order_id = $1
  AND t.voided_ts IS NULL
ORDER BY t.transaction_date DESC, t.id DESC;
