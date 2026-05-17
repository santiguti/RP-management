package handlers

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestReportBalance_EmptyRange(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := getBalanceReport(t, client, ts.URL, "?from=2026-01-01&to=2026-01-31")
	if body.IncomeArs != "0.00" || body.ExpenseArs != "0.00" || body.NetArs != "0.00" || body.TransactionCount != 0 {
		t.Fatalf("balance = %+v, want zeros", body)
	}
}

func TestReportBalance_IncomeMinusExpense(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "100.00", Category: "other_income", Date: "2026-01-05"})
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "250.00", Category: "other_income", Date: "2026-01-06"})
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "50.00", Category: "other_income", Date: "2026-01-07"})
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "80.00", Category: "rent", Date: "2026-01-08"})
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "20.00", Category: "utilities", Date: "2026-01-09"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := getBalanceReport(t, client, ts.URL, "?from=2026-01-01&to=2026-01-31")
	if body.IncomeArs != "400.00" || body.ExpenseArs != "100.00" || body.NetArs != "300.00" || body.TransactionCount != 5 {
		t.Fatalf("balance = %+v, want income 400 expense 100 net 300 count 5", body)
	}
}

func TestReportBalance_ExcludesVoided(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	keep := seedTransaction(t, q, transactionSeed{Type: "income", Amount: "100.00", Category: "other_income", Date: "2026-01-05"})
	voided := seedTransaction(t, q, transactionSeed{Type: "income", Amount: "900.00", Category: "other_income", Date: "2026-01-06"})
	if err := q.SoftDeleteTransaction(context.Background(), sqlc.SoftDeleteTransactionParams{ID: voided.ID}); err != nil {
		t.Fatal(err)
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := getBalanceReport(t, client, ts.URL, "?from=2026-01-01&to=2026-01-31")
	if body.IncomeArs != "100.00" || body.TransactionCount != 1 {
		t.Fatalf("balance = %+v, want only %s", body, uuidString(keep.Ucode))
	}
}

func TestReportPnL_BucketsByType(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "300.00", Category: "wo_payment", Date: "2026-02-01"})
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "100.00", Category: "other_income", Date: "2026-02-02"})
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "200.00", Category: "rent", Date: "2026-02-03"})
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "50.00", Category: "utilities", Date: "2026-02-04"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := getPnLReport(t, client, ts.URL, "?from=2026-02-01&to=2026-02-28")
	if len(body.Income) != 2 || len(body.Expense) != 2 {
		t.Fatalf("pnl = %+v, want two income and two expense buckets", body)
	}
	if body.Income[0].Category != "wo_payment" || body.Income[0].TotalArs != "300.00" {
		t.Fatalf("income = %+v, want wo_payment first", body.Income)
	}
	if body.Expense[0].Category != "rent" || body.Expense[0].TotalArs != "200.00" {
		t.Fatalf("expense = %+v, want rent first", body.Expense)
	}
}

func TestReportDashboard_StructureAndCounts(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	today := time.Now().UTC().Format("2006-01-02")
	clientA := seedClient(t, q, "Cliente Top A", "")
	clientB := seedClient(t, q, "Cliente Top B", "")
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "500.00", Category: "wo_payment", ClientID: clientA.ID, Date: today})
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "250.00", Category: "wo_payment", ClientID: clientB.ID, Date: today})
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "100.00", Category: "rent", Date: today})
	fixtureOld := seedClientAndDevice(t, q, "Cliente Ready Viejo", "REPORT-OLD")
	fixtureFresh := seedClientAndDevice(t, q, "Cliente Ready Nuevo", "REPORT-NEW")
	oldReady := seedReadyWorkOrderAt(t, fixtureOld.client.ID, fixtureOld.device.ID, time.Now().AddDate(0, 0, -10))
	seedReadyWorkOrderAt(t, fixtureFresh.client.ID, fixtureFresh.device.ID, time.Now())
	seedWorkOrder(t, q, clientA.ID, seedDevice(t, q, clientA.ID, lookupBrandByName(t, q, "Samsung").ID, pgtype.Int8{}, lookupArticleTypeByName(t, q, "celular").ID, "REPORT-OPEN").ID)
	ts, httpClient := newCookieServer(t, q)
	defer ts.Close()
	login(t, httpClient, ts.URL, user.Username)

	body := getDashboardReport(t, httpClient, ts.URL)
	if body.Today.IncomeArs != "750.00" || body.Today.ExpenseArs != "100.00" || body.Today.NetArs != "650.00" {
		t.Fatalf("today = %+v, want 750/100/650", body.Today)
	}
	if body.OpenWorkOrdersByStatus["ready"] != 2 || body.OpenWorkOrdersByStatus["received"] != 1 {
		t.Fatalf("status counts = %+v, want ready=2 received=1", body.OpenWorkOrdersByStatus)
	}
	if len(body.AgingReadyWorkOrders) != 1 || body.AgingReadyWorkOrders[0].Ucode != uuidString(oldReady) {
		t.Fatalf("aging = %+v, want only old ready WO", body.AgingReadyWorkOrders)
	}
	if len(body.TopClients90d) < 2 || body.TopClients90d[0].Name != clientA.Name || body.TopClients90d[0].TotalArs != "500.00" {
		t.Fatalf("top clients = %+v, want client A first", body.TopClients90d)
	}
}

func TestReportDashboard_EmptyDB(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := getDashboardReport(t, client, ts.URL)
	if body.Today.IncomeArs != "0.00" || body.Month.ExpenseArs != "0.00" {
		t.Fatalf("dashboard = %+v, want zero money", body)
	}
	if len(body.OpenWorkOrdersByStatus) != 0 || len(body.AgingReadyWorkOrders) != 0 || len(body.TopClients90d) != 0 {
		t.Fatalf("dashboard = %+v, want empty collections", body)
	}
}

func getBalanceReport(t *testing.T, client *http.Client, baseURL, query string) balanceReportDTO {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/reports/balance" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body balanceReportDTO
	decodeJSON(t, res.Body, &body)
	return body
}

func getPnLReport(t *testing.T, client *http.Client, baseURL, query string) pnlReportDTO {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/reports/pnl" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body pnlReportDTO
	decodeJSON(t, res.Body, &body)
	return body
}

func getDashboardReport(t *testing.T, client *http.Client, baseURL string) dashboardReportDTO {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/reports/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body dashboardReportDTO
	decodeJSON(t, res.Body, &body)
	return body
}

func seedReadyWorkOrderAt(t *testing.T, clientID, deviceID int64, readyAt time.Time) pgtype.UUID {
	t.Helper()
	var ucode pgtype.UUID
	if err := testPool.QueryRow(context.Background(), `
INSERT INTO rp.work_orders (
  client_id,
  device_id,
  service_type,
  status,
  reported_issue,
  ready_ts
)
VALUES ($1, $2, 'in_shop', 'ready', 'Listo para retirar', $3)
RETURNING ucode
`, clientID, deviceID, pgtype.Timestamptz{Time: readyAt, Valid: true}).Scan(&ucode); err != nil {
		t.Fatal(err)
	}
	return ucode
}
