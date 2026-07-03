package handlers

import (
	"context"
	"testing"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func newTxQueries(t *testing.T) (*sqlc.Queries, func()) {
	t.Helper()

	resetTestDB(t)
	return sqlc.New(testPool), func() { resetTestDB(t) }
}

func resetTestDB(t *testing.T) {
	t.Helper()

	_, err := testPool.Exec(context.Background(), `
TRUNCATE
  rp.audit_log,
  rp.sessions,
  rp.attachments,
  rp.work_order_parts,
  rp.part_movements,
  rp.parts,
  rp.transactions,
  rp.recurring_expenses,
  rp.suppliers,
  rp.work_orders,
  rp.wo_number_counters,
  rp.devices,
  rp.clients,
  rp.device_models,
  rp.brands,
  rp.article_types,
  rp.users
RESTART IDENTITY CASCADE;

INSERT INTO rp.brands (name) VALUES
  ('Samsung'), ('Apple'), ('LG'), ('Philips'), ('Whirlpool'),
  ('BGH'), ('Drean'), ('Liliana'), ('Atma'), ('Yelmo'),
  ('Noblex'), ('RCA'), ('Hisense'), ('Xiaomi'), ('Motorola'),
  ('Otros')
ON CONFLICT (name) DO NOTHING;

INSERT INTO rp.article_types (name) VALUES
  ('celular'), ('notebook'), ('tablet'),
  ('heladera'), ('lavarropas'), ('microondas'), ('aire_acondicionado'),
  ('tv'), ('audio'), ('otro')
ON CONFLICT (name) DO NOTHING;
`)
	if err != nil {
		t.Fatal(err)
	}
}
