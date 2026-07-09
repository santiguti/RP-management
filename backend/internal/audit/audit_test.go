package audit

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

func TestRecord_InsertsRow(t *testing.T) {
	q, tx := auditTxQueries(t)
	action := "test.audit.record.insert"

	Record(context.Background(), q, httptest.NewRequest("POST", "/", nil), Entry{
		Action:     action,
		EntityType: "client",
		After: map[string]string{
			"name": "Cliente Audit",
		},
	})

	var gotAction, gotEntityType string
	if err := tx.QueryRow(context.Background(), `
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

func TestRecord_SurvivesCanceledContext(t *testing.T) {
	q, tx := auditTxQueries(t)
	action := "test.audit.record.canceled_context"
	actor := seedAuditUser(t, tx)
	ctx, cancel := context.WithCancel(middleware.WithUser(context.Background(), &actor))
	cancel()

	Record(ctx, q, httptest.NewRequest("POST", "/", nil), Entry{
		Action:     action,
		EntityType: "client",
		After: map[string]string{
			"name": "Cliente Audit",
		},
	})

	var actorUserID int64
	if err := tx.QueryRow(context.Background(), `
SELECT actor_user_id
FROM rp.audit_log
WHERE action = $1
`, action).Scan(&actorUserID); err != nil {
		t.Fatal(err)
	}
	if actorUserID != actor.ID {
		t.Fatalf("actor_user_id = %d, want %d", actorUserID, actor.ID)
	}
}

func TestRecord_NilBeforeAfter(t *testing.T) {
	q, tx := auditTxQueries(t)
	action := "test.audit.record.nil_json"

	Record(context.Background(), q, httptest.NewRequest("POST", "/", nil), Entry{
		Action:     action,
		EntityType: "client",
	})

	var beforeIsNull, afterIsNull bool
	if err := tx.QueryRow(context.Background(), `
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

func auditTxQueries(t *testing.T) (*sqlc.Queries, pgx.Tx) {
	t.Helper()
	pool := auditTestPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return sqlc.New(pool).WithTx(tx), tx
}

func seedAuditUser(t *testing.T, tx pgx.Tx) sqlc.User {
	t.Helper()
	row := tx.QueryRow(context.Background(), `
INSERT INTO rp.users (username, password_hash, full_name, role)
VALUES ('audit-canceled-context', 'hash', 'Audit Canceled Context', 'owner')
RETURNING id, ucode, created_ts, created_by_user_id, voided_ts, voided_by_user_id, username, password_hash, full_name, role, last_login_ts
`)
	var user sqlc.User
	if err := row.Scan(
		&user.ID,
		&user.Ucode,
		&user.CreatedTs,
		&user.CreatedByUserID,
		&user.VoidedTs,
		&user.VoidedByUserID,
		&user.Username,
		&user.PasswordHash,
		&user.FullName,
		&user.Role,
		&user.LastLoginTs,
	); err != nil {
		t.Fatal(err)
	}
	return user
}
