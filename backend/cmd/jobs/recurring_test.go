package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("RP_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Println("skipping: no DATABASE_URL")
		os.Exit(0)
	}
	if os.Getenv("DATABASE_URL") == "" {
		_ = os.Setenv("DATABASE_URL", dsn)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		panic(err)
	}
	testPool = pool
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func TestRunRecurring_CreatesTransactionForDueRule(t *testing.T) {
	q := resetJobsTestDB(t)
	rule := seedJobRecurringExpense(t, q, jobRecurringSeed{Name: "Alquiler", DayOfMonth: 1})

	if err := runRecurring([]string{"--at", "2026-02-15"}); err != nil {
		t.Fatal(err)
	}

	if got := countJobTransactionsForRecurring(t, rule.ID); got != 1 {
		t.Fatalf("transactions = %d, want 1", got)
	}
	updated, err := q.GetRecurringExpenseByUcode(context.Background(), rule.Ucode)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RecurringExpense.LastGeneratedDate.Time.Format("2006-01-02") != "2026-02-01" {
		t.Fatalf("last_generated_date = %s, want 2026-02-01", updated.RecurringExpense.LastGeneratedDate.Time.Format("2006-01-02"))
	}
}

func TestRunRecurring_SkipsAlreadyGenerated(t *testing.T) {
	q := resetJobsTestDB(t)
	rule := seedJobRecurringExpense(t, q, jobRecurringSeed{Name: "Expensas", DayOfMonth: 1, LastGeneratedDate: "2026-02-01"})

	if err := runRecurring([]string{"--at", "2026-02-15"}); err != nil {
		t.Fatal(err)
	}

	if got := countJobTransactionsForRecurring(t, rule.ID); got != 0 {
		t.Fatalf("transactions = %d, want 0", got)
	}
}

func TestRunRecurring_BackfillsMultipleMonthsIsOutOfScope(t *testing.T) {
	t.Skip("v1.4 generates only the most-recent missed month; multi-month backfill is v1.5 if needed.")
}

func TestRunRecurring_SkipsInactive(t *testing.T) {
	q := resetJobsTestDB(t)
	active := seedJobRecurringExpense(t, q, jobRecurringSeed{Name: "Activo", DayOfMonth: 1, Active: boolPtr(true)})
	inactive := seedJobRecurringExpense(t, q, jobRecurringSeed{Name: "Inactivo", DayOfMonth: 1, Active: boolPtr(false)})

	if err := runRecurring([]string{"--at", "2026-02-15"}); err != nil {
		t.Fatal(err)
	}

	if got := countJobTransactionsForRecurring(t, active.ID); got != 1 {
		t.Fatalf("active transactions = %d, want 1", got)
	}
	if got := countJobTransactionsForRecurring(t, inactive.ID); got != 0 {
		t.Fatalf("inactive transactions = %d, want 0", got)
	}
}

func TestRunNow_SingleRule(t *testing.T) {
	q := resetJobsTestDB(t)
	rule := seedJobRecurringExpense(t, q, jobRecurringSeed{Name: "Forzado", DayOfMonth: 1, Active: boolPtr(false)})

	if err := runRecurring([]string{"--rule", uuidString(rule.Ucode), "--at", "2026-02-15"}); err != nil {
		t.Fatal(err)
	}

	if got := countJobTransactionsForRecurring(t, rule.ID); got != 1 {
		t.Fatalf("transactions = %d, want 1", got)
	}
}

type jobRecurringSeed struct {
	Name              string
	Amount            string
	DayOfMonth        int
	Active            *bool
	LastGeneratedDate string
}

func resetJobsTestDB(t *testing.T) *sqlc.Queries {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
TRUNCATE
  rp.sessions,
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
	return sqlc.New(testPool)
}

func seedJobRecurringExpense(t *testing.T, q *sqlc.Queries, seed jobRecurringSeed) sqlc.RecurringExpense {
	t.Helper()
	if seed.Name == "" {
		seed.Name = "Recurrente"
	}
	if seed.Amount == "" {
		seed.Amount = "100.00"
	}
	if seed.DayOfMonth == 0 {
		seed.DayOfMonth = 1
	}
	active := true
	if seed.Active != nil {
		active = *seed.Active
	}
	amount := mustJobNumeric(t, seed.Amount)
	rule, err := q.CreateRecurringExpense(context.Background(), sqlc.CreateRecurringExpenseParams{
		Name:          seed.Name,
		Amount:        amount,
		Currency:      "ARS",
		DayOfMonth:    int32(seed.DayOfMonth),
		Category:      "rent",
		PaymentMethod: "transfer",
		Active:        active,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seed.LastGeneratedDate != "" {
		date := mustJobDate(t, seed.LastGeneratedDate)
		if err := q.MarkRecurringExpenseGenerated(context.Background(), sqlc.MarkRecurringExpenseGeneratedParams{
			ID:                rule.ID,
			LastGeneratedDate: date,
		}); err != nil {
			t.Fatal(err)
		}
		rule.LastGeneratedDate = date
	}
	return rule
}

func countJobTransactionsForRecurring(t *testing.T, recurringID int64) int64 {
	t.Helper()
	var count int64
	if err := testPool.QueryRow(context.Background(), `
SELECT count(*)::bigint
FROM rp.transactions
WHERE recurring_expense_id = $1
  AND voided_ts IS NULL
`, recurringID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func mustJobNumeric(t *testing.T, raw string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(raw); err != nil {
		t.Fatal(err)
	}
	return n
}

func mustJobDate(t *testing.T, raw string) pgtype.Date {
	t.Helper()
	var date pgtype.Date
	if err := date.Scan(raw); err != nil {
		t.Fatal(err)
	}
	return date
}

func uuidString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func boolPtr(value bool) *bool {
	return &value
}
