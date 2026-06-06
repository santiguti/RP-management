package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/money"
)

func TestRecurring_CreateAsOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := createRecurringExpense(t, client, ts.URL, csrf, map[string]any{
		"name":         "Alquiler",
		"amount":       "200000.00",
		"day_of_month": 10,
		"category":     "rent",
	}, http.StatusCreated)
	if body.RecurringExpense.Ucode == "" {
		t.Fatal("recurring_expense.ucode is empty")
	}
	if !body.RecurringExpense.Active {
		t.Fatal("active = false, want default true")
	}
	if body.RecurringExpense.Currency != "ARS" || body.RecurringExpense.PaymentMethod != "transfer" {
		t.Fatalf("currency/payment_method = %q/%q, want ARS/transfer", body.RecurringExpense.Currency, body.RecurringExpense.PaymentMethod)
	}
}

func TestRecurring_CreateAsEmployee_403(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	employee := seedUserWithRole(t, q, "employee")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, employee.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/recurring-expenses", baseRecurringPayload(), csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestRecurring_RejectsDayOfMonth30(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	payload := baseRecurringPayload()
	payload["day_of_month"] = 30
	res := postJSON(t, client, ts.URL+"/api/v1/recurring-expenses", payload, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestRecurring_List_OrdersActiveFirst(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	inactive := seedRecurringExpense(t, q, recurringSeed{Name: "Inactivo", DayOfMonth: 1, Active: boolPtr(false)})
	active := seedRecurringExpense(t, q, recurringSeed{Name: "Activo", DayOfMonth: 28, Active: boolPtr(true)})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := listRecurringExpenses(t, client, ts.URL)
	if len(body.RecurringExpenses) < 2 {
		t.Fatalf("recurring_expenses = %+v, want at least 2", body.RecurringExpenses)
	}
	if body.RecurringExpenses[0].Ucode != uuidString(active.Ucode) || body.RecurringExpenses[1].Ucode != uuidString(inactive.Ucode) {
		t.Fatalf("order = %+v, want active first", body.RecurringExpenses[:2])
	}
}

func TestRecurring_UpdateAsOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	rule := seedRecurringExpense(t, q, recurringSeed{Name: "Servicio", Amount: "100.00", Active: boolPtr(true)})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := patchJSON(t, client, ts.URL+"/api/v1/recurring-expenses/"+uuidString(rule.Ucode), map[string]any{
		"amount": "250.50",
		"active": false,
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body recurringBody
	decodeJSON(t, res.Body, &body)
	if body.RecurringExpense.Amount != "250.50" || body.RecurringExpense.Active {
		t.Fatalf("recurring_expense = %+v, want amount changed and inactive", body.RecurringExpense)
	}
}

func TestRecurring_RunNow_GeneratesTransaction(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	rule := seedRecurringExpense(t, q, recurringSeed{Name: "Internet", Amount: "15000.00", DayOfMonth: 1})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := runRecurringNow(t, client, ts.URL, csrf, uuidString(rule.Ucode), http.StatusOK)
	if body.Transaction == nil {
		t.Fatal("transaction = nil, want generated transaction")
	}
	if body.Transaction.RecurringExpense == nil || body.Transaction.RecurringExpense.Name != rule.Name {
		t.Fatalf("recurring_expense ref = %+v, want %q", body.Transaction.RecurringExpense, rule.Name)
	}
	if got := countTransactionsForRecurring(t, rule.ID); got != 1 {
		t.Fatalf("transactions for recurring = %d, want 1", got)
	}
	updated, err := q.GetRecurringExpenseByUcode(context.Background(), rule.Ucode)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.RecurringExpense.LastGeneratedDate.Valid {
		t.Fatal("last_generated_date is null, want due date")
	}
}

func TestRecurring_RunNow_IdempotentForSameDueDate(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	rule := seedRecurringExpense(t, q, recurringSeed{Name: "Luz", Amount: "8000.00", DayOfMonth: 1})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	first := runRecurringNow(t, client, ts.URL, csrf, uuidString(rule.Ucode), http.StatusOK)
	if first.Transaction == nil {
		t.Fatal("first transaction = nil, want generated")
	}
	second := runRecurringNow(t, client, ts.URL, csrf, uuidString(rule.Ucode), http.StatusOK)
	if second.Transaction != nil || second.Reason != "already_generated_for_due_date" {
		t.Fatalf("second response = %+v, want already generated null transaction", second)
	}
	if got := countTransactionsForRecurring(t, rule.ID); got != 1 {
		t.Fatalf("transactions for recurring = %d, want 1", got)
	}
}

func TestRecurring_RunNow_AsEmployee_403(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	employee := seedUserWithRole(t, q, "employee")
	rule := seedRecurringExpense(t, q, recurringSeed{Name: "Impuestos", Amount: "12000.00"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, employee.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/recurring-expenses/"+uuidString(rule.Ucode)+"/run-now", map[string]any{}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestRecurring_DeleteIdempotent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	rule := seedRecurringExpense(t, q, recurringSeed{Name: "Borrable"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	for i := 0; i < 2; i++ {
		res := deleteReq(t, client, ts.URL+"/api/v1/recurring-expenses/"+uuidString(rule.Ucode), csrf)
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %d status = %d, want %d: %s", i+1, res.StatusCode, http.StatusNoContent, readBody(t, res))
		}
	}
}

type recurringBody struct {
	RecurringExpense recurringExpenseDTO `json:"recurring_expense"`
}

type recurringListBody struct {
	RecurringExpenses []recurringExpenseDTO `json:"recurring_expenses"`
}

type runNowBody struct {
	Transaction *transactionDTO `json:"transaction"`
	Reason      string          `json:"reason"`
}

type recurringSeed struct {
	Name        string
	Amount      string
	DayOfMonth  int
	Category    string
	Active      *bool
	SupplierID  int64
	Description string
}

func createRecurringExpense(t *testing.T, client *http.Client, baseURL, csrf string, payload map[string]any, status int) recurringBody {
	t.Helper()
	res := postJSON(t, client, baseURL+"/api/v1/recurring-expenses", payload, csrf)
	defer res.Body.Close()
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
	var body recurringBody
	decodeJSON(t, res.Body, &body)
	return body
}

func listRecurringExpenses(t *testing.T, client *http.Client, baseURL string) recurringListBody {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/recurring-expenses")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body recurringListBody
	decodeJSON(t, res.Body, &body)
	return body
}

func runRecurringNow(t *testing.T, client *http.Client, baseURL, csrf, ucode string, status int) runNowBody {
	t.Helper()
	res := postJSON(t, client, baseURL+"/api/v1/recurring-expenses/"+ucode+"/run-now", map[string]any{}, csrf)
	defer res.Body.Close()
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
	var body runNowBody
	decodeJSON(t, res.Body, &body)
	return body
}

func seedRecurringExpense(t *testing.T, q *sqlc.Queries, seed recurringSeed) sqlc.RecurringExpense {
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
	if seed.Category == "" {
		seed.Category = "rent"
	}
	active := true
	if seed.Active != nil {
		active = *seed.Active
	}
	amount, err := money.StringToNumeric(seed.Amount)
	if err != nil {
		t.Fatal(err)
	}
	var supplierID pgtype.Int8
	if seed.SupplierID != 0 {
		supplierID = pgtype.Int8{Int64: seed.SupplierID, Valid: true}
	}
	rule, err := q.CreateRecurringExpense(context.Background(), sqlc.CreateRecurringExpenseParams{
		Name:          seed.Name,
		Amount:        amount,
		Currency:      "ARS",
		DayOfMonth:    int32(seed.DayOfMonth),
		Category:      seed.Category,
		PaymentMethod: "transfer",
		SupplierID:    supplierID,
		Description:   textFromPtr(&seed.Description),
		Active:        active,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule
}

func countTransactionsForRecurring(t *testing.T, recurringID int64) int64 {
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

func baseRecurringPayload() map[string]any {
	return map[string]any{
		"name":         "Recurrente",
		"amount":       "100.00",
		"day_of_month": 1,
		"category":     "rent",
	}
}

func boolPtr(value bool) *bool {
	return &value
}
