-- name: ReportBalance :one
SELECT
  COALESCE(SUM(CASE WHEN transaction_type = 'income' THEN amount * fx_rate_to_ars ELSE 0 END), 0)::numeric(18, 2) AS income_ars,
  COALESCE(SUM(CASE WHEN transaction_type = 'expense' THEN amount * fx_rate_to_ars ELSE 0 END), 0)::numeric(18, 2) AS expense_ars,
  COUNT(*)::bigint AS transaction_count
FROM rp.transactions
WHERE voided_ts IS NULL
  AND transaction_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date;

-- name: ReportPnLByCategory :many
SELECT
  transaction_type,
  category,
  COALESCE(SUM(amount * fx_rate_to_ars), 0)::numeric(18, 2) AS total_ars,
  COUNT(*)::bigint AS transaction_count
FROM rp.transactions
WHERE voided_ts IS NULL
  AND transaction_date BETWEEN sqlc.arg(date_from)::date AND sqlc.arg(date_to)::date
GROUP BY transaction_type, category
ORDER BY transaction_type, total_ars DESC;

-- name: ReportDashboardCounters :one
SELECT
  COALESCE(SUM(CASE WHEN t.transaction_date = (now() AT TIME ZONE 'UTC')::date AND t.transaction_type = 'income' THEN t.amount * t.fx_rate_to_ars ELSE 0 END), 0)::numeric(18, 2) AS income_today_ars,
  COALESCE(SUM(CASE WHEN t.transaction_date = (now() AT TIME ZONE 'UTC')::date AND t.transaction_type = 'expense' THEN t.amount * t.fx_rate_to_ars ELSE 0 END), 0)::numeric(18, 2) AS expense_today_ars,
  COALESCE(SUM(CASE WHEN date_trunc('month', t.transaction_date)::date = date_trunc('month', now() AT TIME ZONE 'UTC')::date AND t.transaction_type = 'income' THEN t.amount * t.fx_rate_to_ars ELSE 0 END), 0)::numeric(18, 2) AS income_month_ars,
  COALESCE(SUM(CASE WHEN date_trunc('month', t.transaction_date)::date = date_trunc('month', now() AT TIME ZONE 'UTC')::date AND t.transaction_type = 'expense' THEN t.amount * t.fx_rate_to_ars ELSE 0 END), 0)::numeric(18, 2) AS expense_month_ars
FROM rp.transactions t
WHERE t.voided_ts IS NULL;

-- name: ReportWorkOrderCountsByStatus :many
SELECT status, COUNT(*)::bigint AS count
FROM rp.work_orders
WHERE voided_ts IS NULL
GROUP BY status;

-- name: ReportTopClientsByRevenue :many
SELECT c.ucode, c.name, COALESCE(SUM(t.amount * t.fx_rate_to_ars), 0)::numeric(18, 2) AS total_ars
FROM rp.transactions t
JOIN rp.clients c ON c.id = t.client_id
WHERE t.voided_ts IS NULL
  AND t.transaction_type = 'income'
  AND t.transaction_date >= ((now() AT TIME ZONE 'UTC')::date - INTERVAL '90 days')
GROUP BY c.ucode, c.name
ORDER BY total_ars DESC
LIMIT 5;

-- name: ReportAgingReadyWorkOrders :many
SELECT wo.ucode, wo.wo_number, wo.ready_ts, c.name AS client_name
FROM rp.work_orders wo
JOIN rp.clients c ON c.id = wo.client_id
WHERE wo.voided_ts IS NULL
  AND wo.status = 'ready'
  AND wo.ready_ts < now() - INTERVAL '7 days'
ORDER BY wo.ready_ts ASC;
