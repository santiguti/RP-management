-- +goose Up
CREATE TABLE rp.suppliers (
  id                 BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode              UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts         TIMESTAMPTZ  NOT NULL DEFAULT now(),
  created_by_user_id BIGINT       NULL REFERENCES rp.users(id),
  voided_ts          TIMESTAMPTZ  NULL,
  voided_by_user_id  BIGINT       NULL REFERENCES rp.users(id),

  name               TEXT         NOT NULL,
  phone              TEXT         NULL,
  email              TEXT         NULL,
  notes              TEXT         NULL
);

CREATE UNIQUE INDEX suppliers_name_unique ON rp.suppliers(lower(name)) WHERE voided_ts IS NULL;

CREATE TABLE rp.recurring_expenses (
  id                  BIGINT         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode               UUID           NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts          TIMESTAMPTZ    NOT NULL DEFAULT now(),
  created_by_user_id  BIGINT         NULL REFERENCES rp.users(id),
  voided_ts           TIMESTAMPTZ    NULL,
  voided_by_user_id   BIGINT         NULL REFERENCES rp.users(id),

  name                TEXT           NOT NULL,
  amount              NUMERIC(14, 2) NOT NULL CHECK (amount > 0),
  currency            CHAR(3)        NOT NULL DEFAULT 'ARS',
  day_of_month        INT            NOT NULL CHECK (day_of_month BETWEEN 1 AND 28),
  category            TEXT           NOT NULL CHECK (category IN ('rent','utilities','salary','taxes','supplies','other_expense')),
  payment_method      TEXT           NOT NULL DEFAULT 'transfer' CHECK (payment_method IN ('cash','transfer','card','mercadopago','other')),
  supplier_id         BIGINT         NULL REFERENCES rp.suppliers(id) ON DELETE SET NULL,
  description         TEXT           NULL,
  active              BOOL           NOT NULL DEFAULT true,
  last_generated_date DATE           NULL
);

CREATE INDEX recurring_expenses_active_idx ON rp.recurring_expenses(active) WHERE voided_ts IS NULL;

CREATE TABLE rp.transactions (
  id                   BIGINT         GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  ucode                UUID           NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  created_ts           TIMESTAMPTZ    NOT NULL DEFAULT now(),
  created_by_user_id   BIGINT         NULL REFERENCES rp.users(id),
  voided_ts            TIMESTAMPTZ    NULL,
  voided_by_user_id    BIGINT         NULL REFERENCES rp.users(id),

  transaction_type     TEXT           NOT NULL CHECK (transaction_type IN ('income', 'expense')),
  amount               NUMERIC(14, 2) NOT NULL CHECK (amount > 0),
  currency             CHAR(3)        NOT NULL DEFAULT 'ARS',
  fx_rate_to_ars       NUMERIC(14, 4) NOT NULL DEFAULT 1,
  transaction_date     DATE           NOT NULL DEFAULT (now() AT TIME ZONE 'UTC')::date,
  payment_method       TEXT           NOT NULL CHECK (payment_method IN ('cash','transfer','card','mercadopago','other')),
  category             TEXT           NOT NULL CHECK (category IN ('wo_payment','wo_deposit','part_purchase','supplies','rent','utilities','salary','taxes','food','transport','other_income','other_expense')),
  counterparty_type    TEXT           NOT NULL CHECK (counterparty_type IN ('client','supplier','none')),
  client_id            BIGINT         NULL REFERENCES rp.clients(id) ON DELETE RESTRICT,
  supplier_id          BIGINT         NULL REFERENCES rp.suppliers(id) ON DELETE RESTRICT,
  work_order_id        BIGINT         NULL REFERENCES rp.work_orders(id) ON DELETE RESTRICT,
  description          TEXT           NULL,
  recurring_expense_id BIGINT         NULL REFERENCES rp.recurring_expenses(id) ON DELETE SET NULL,

  CONSTRAINT transactions_counterparty_consistency CHECK (
    (counterparty_type = 'client' AND client_id IS NOT NULL AND supplier_id IS NULL)
    OR (counterparty_type = 'supplier' AND supplier_id IS NOT NULL AND client_id IS NULL)
    OR (counterparty_type = 'none' AND client_id IS NULL AND supplier_id IS NULL)
  )
);

CREATE INDEX transactions_date_idx ON rp.transactions(transaction_date DESC) WHERE voided_ts IS NULL;
CREATE INDEX transactions_wo_idx ON rp.transactions(work_order_id) WHERE work_order_id IS NOT NULL AND voided_ts IS NULL;
CREATE INDEX transactions_client_idx ON rp.transactions(client_id) WHERE client_id IS NOT NULL AND voided_ts IS NULL;
CREATE INDEX transactions_supplier_idx ON rp.transactions(supplier_id) WHERE supplier_id IS NOT NULL AND voided_ts IS NULL;
CREATE INDEX transactions_type_category_idx ON rp.transactions(transaction_type, category) WHERE voided_ts IS NULL;

-- +goose Down
DROP TABLE IF EXISTS rp.transactions;
DROP TABLE IF EXISTS rp.recurring_expenses;
DROP TABLE IF EXISTS rp.suppliers;
