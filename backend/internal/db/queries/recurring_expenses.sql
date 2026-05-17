-- name: ListRecurringExpenses :many
SELECT sqlc.embed(re), s.ucode AS supplier_ucode, s.name AS supplier_name
FROM rp.recurring_expenses re
LEFT JOIN rp.suppliers s ON s.id = re.supplier_id
WHERE re.voided_ts IS NULL
ORDER BY re.active DESC, re.day_of_month ASC, re.name ASC;

-- name: GetRecurringExpenseByUcode :one
SELECT sqlc.embed(re), s.ucode AS supplier_ucode, s.name AS supplier_name
FROM rp.recurring_expenses re
LEFT JOIN rp.suppliers s ON s.id = re.supplier_id
WHERE re.ucode = $1
  AND re.voided_ts IS NULL;

-- name: CreateRecurringExpense :one
INSERT INTO rp.recurring_expenses (
  name,
  amount,
  currency,
  day_of_month,
  category,
  payment_method,
  supplier_id,
  description,
  active,
  created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateRecurringExpense :one
UPDATE rp.recurring_expenses
SET
  name = $2,
  amount = $3,
  currency = $4,
  day_of_month = $5,
  category = $6,
  payment_method = $7,
  supplier_id = $8,
  description = $9,
  active = $10
WHERE id = $1
  AND voided_ts IS NULL
RETURNING *;

-- name: SoftDeleteRecurringExpense :exec
UPDATE rp.recurring_expenses
SET
  voided_ts = now(),
  voided_by_user_id = $2
WHERE id = $1
  AND voided_ts IS NULL;

-- name: ListDueRecurringExpenses :many
SELECT *
FROM rp.recurring_expenses
WHERE voided_ts IS NULL
  AND active = true
  AND (last_generated_date IS NULL OR last_generated_date < $1::date)
ORDER BY day_of_month ASC, name ASC;

-- name: MarkRecurringExpenseGenerated :exec
UPDATE rp.recurring_expenses
SET last_generated_date = $2
WHERE id = $1
  AND voided_ts IS NULL;
