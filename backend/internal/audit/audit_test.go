package audit

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestRecord_InsertsRow(t *testing.T) {
	pool := auditTestPool(t)
	q := sqlc.New(pool)
	action := "test.audit.record.insert"
	t.Cleanup(func() { cleanupAuditAction(t, pool, action) })

	Record(context.Background(), q, httptest.NewRequest("POST", "/", nil), Entry{
		Action:     action,
		EntityType: "client",
		After: map[string]string{
			"name": "Cliente Audit",
		},
	})

	var gotAction, gotEntityType string
	if err := pool.QueryRow(context.Background(), `
SELECT action, entity_type
FROM rp.audit_log
WHERE action = $1
`, action).Scan(&gotAction, &gotEntityType); err != nil {
		t.Fatal(err)
	}
	if gotAction != action || gotEntityType != "client" {
		t.Fatalf("audit row = %s/%s, want %s/client", gotAction, gotEntityType, action)
	}
}

func TestRecord_FailsSilently(t *testing.T) {
	pool := auditTestPool(t)
	pool.Close()

	Record(context.Background(), sqlc.New(pool), httptest.NewRequest("POST", "/", nil), Entry{
		Action:     "test.audit.closed_pool",
		EntityType: "client",
	})
}

func TestRecord_NilBeforeAfter(t *testing.T) {
	pool := auditTestPool(t)
	q := sqlc.New(pool)
	action := "test.audit.record.nil_json"
	t.Cleanup(func() { cleanupAuditAction(t, pool, action) })

	Record(context.Background(), q, httptest.NewRequest("POST", "/", nil), Entry{
		Action:     action,
		EntityType: "client",
	})

	var beforeIsNull, afterIsNull bool
	if err := pool.QueryRow(context.Background(), `
SELECT before_json IS NULL, after_json IS NULL
FROM rp.audit_log
WHERE action = $1
`, action).Scan(&beforeIsNull, &afterIsNull); err != nil {
		t.Fatal(err)
	}
	if !beforeIsNull || !afterIsNull {
		t.Fatalf("json null flags = before %v after %v, want both true", beforeIsNull, afterIsNull)
	}
}

func auditTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("RP_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("no DATABASE_URL")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanupAuditAction(t *testing.T, pool *pgxpool.Pool, action string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `DELETE FROM rp.audit_log WHERE action = $1`, action)
	if err != nil {
		t.Fatal(err)
	}
}
