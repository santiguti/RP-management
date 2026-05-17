package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestTransaction_CreateIncomeWithClient(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	clientRow := seedClient(t, q, "Cliente Pago", "")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := createTransaction(t, client, ts.URL, csrf, map[string]any{
		"transaction_type":  "income",
		"amount":            "12000.00",
		"payment_method":    "cash",
		"category":          "wo_payment",
		"counterparty_type": "client",
		"client_ucode":      uuidString(clientRow.Ucode),
	}, http.StatusCreated)
	if body.Transaction.Client == nil || body.Transaction.Client.Name != clientRow.Name {
		t.Fatalf("client = %+v, want %q", body.Transaction.Client, clientRow.Name)
	}
	if body.Transaction.Supplier != nil {
		t.Fatalf("supplier = %+v, want nil", body.Transaction.Supplier)
	}
	assertTransactionAmount(t, body.Transaction, "12000.00")
}

func TestTransaction_CreateExpenseWithSupplier(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	supplier := seedSupplier(t, q, "Proveedor Repuestos")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := createTransaction(t, client, ts.URL, csrf, map[string]any{
		"transaction_type":  "expense",
		"amount":            "5400.50",
		"payment_method":    "transfer",
		"category":          "part_purchase",
		"counterparty_type": "supplier",
		"supplier_ucode":    uuidString(supplier.Ucode),
	}, http.StatusCreated)
	if body.Transaction.Supplier == nil || body.Transaction.Supplier.Name != supplier.Name {
		t.Fatalf("supplier = %+v, want %q", body.Transaction.Supplier, supplier.Name)
	}
	if body.Transaction.Client != nil {
		t.Fatalf("client = %+v, want nil", body.Transaction.Client)
	}
}

func TestTransaction_CreateNoneCounterparty(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := createTransaction(t, client, ts.URL, csrf, map[string]any{
		"transaction_type":  "expense",
		"amount":            "200000.00",
		"payment_method":    "transfer",
		"category":          "rent",
		"counterparty_type": "none",
	}, http.StatusCreated)
	if body.Transaction.Client != nil || body.Transaction.Supplier != nil {
		t.Fatalf("client/supplier = %+v/%+v, want nil/nil", body.Transaction.Client, body.Transaction.Supplier)
	}
}

func TestTransaction_CreateWoPaymentWithWorkOrder(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente WO Pago", "TX-WO-PAY")
	workOrder := seedWorkOrder(t, q, fixture.client.ID, fixture.device.ID)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := createTransaction(t, client, ts.URL, csrf, map[string]any{
		"transaction_type":  "income",
		"amount":            "9900.00",
		"payment_method":    "mercadopago",
		"category":          "wo_payment",
		"counterparty_type": "client",
		"client_ucode":      uuidString(fixture.client.Ucode),
		"work_order_ucode":  uuidString(workOrder.Ucode),
	}, http.StatusCreated)
	if body.Transaction.Client == nil || body.Transaction.WorkOrder == nil {
		t.Fatalf("client/work_order = %+v/%+v, want both populated", body.Transaction.Client, body.Transaction.WorkOrder)
	}
	if body.Transaction.WorkOrder.WoNumber != workOrder.WoNumber {
		t.Fatalf("wo_number = %q, want %q", body.Transaction.WorkOrder.WoNumber, workOrder.WoNumber)
	}
}

func TestTransaction_RejectsMissingClientUcode(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/transactions", baseTransactionPayload("income", "100.00", "wo_payment", "client"), csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "counterparty_mismatch")
}

func TestTransaction_RejectsBothCounterparties(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	clientRow := seedClient(t, q, "Cliente Doble", "")
	supplier := seedSupplier(t, q, "Proveedor Doble")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	payload := baseTransactionPayload("income", "100.00", "wo_payment", "client")
	payload["client_ucode"] = uuidString(clientRow.Ucode)
	payload["supplier_ucode"] = uuidString(supplier.Ucode)
	res := postJSON(t, client, ts.URL+"/api/v1/transactions", payload, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "counterparty_mismatch")
}

func TestTransaction_RejectsInvalidCategory(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/transactions", baseTransactionPayload("expense", "100.00", "bad", "none"), csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestTransaction_RejectsZeroAmount(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/transactions", baseTransactionPayload("expense", "0", "rent", "none"), csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_amount")
}

func TestTransaction_RejectsNegativeAmount(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/transactions", baseTransactionPayload("expense", "-1.00", "rent", "none"), csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_amount")
}

func TestTransaction_ListByDateRange(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	for i, date := range []string{"2026-05-01", "2026-05-03", "2026-05-05", "2026-05-07", "2026-05-09"} {
		seedTransaction(t, q, transactionSeed{Category: "rent", Date: date, Description: fmt.Sprintf("tx %d", i)})
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := searchTransactions(t, client, ts.URL, "?from=2026-05-03&to=2026-05-07")
	if body.Total != 3 || len(body.Transactions) != 3 {
		t.Fatalf("total/len = %d/%d, want 3/3", body.Total, len(body.Transactions))
	}
	got := []string{body.Transactions[0].TransactionDate, body.Transactions[1].TransactionDate, body.Transactions[2].TransactionDate}
	want := []string{"2026-05-07", "2026-05-05", "2026-05-03"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dates = %v, want %v", got, want)
		}
	}
}

func TestTransaction_ListByType(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedTransaction(t, q, transactionSeed{Type: "income", Category: "other_income", Date: "2026-05-01"})
	seedTransaction(t, q, transactionSeed{Type: "expense", Category: "rent", Date: "2026-05-02"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := searchTransactions(t, client, ts.URL, "?type=income")
	if body.Total != 1 || body.Transactions[0].TransactionType != "income" {
		t.Fatalf("body = %+v, want one income", body)
	}
}

func TestTransaction_ListByCategory(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedTransaction(t, q, transactionSeed{Category: "rent", Date: "2026-05-01"})
	seedTransaction(t, q, transactionSeed{Category: "utilities", Date: "2026-05-02"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := searchTransactions(t, client, ts.URL, "?category=rent")
	if body.Total != 1 || body.Transactions[0].Category != "rent" {
		t.Fatalf("body = %+v, want one rent transaction", body)
	}
}

func TestTransaction_ListPagination(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	for i := 0; i < 30; i++ {
		seedTransaction(t, q, transactionSeed{Category: "rent", Date: fmt.Sprintf("2026-05-%02d", (i%28)+1)})
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := searchTransactions(t, client, ts.URL, "?page_size=10&page=2")
	if body.Total != 30 || len(body.Transactions) != 10 || body.Page != 2 || body.PageSize != 10 {
		t.Fatalf("body = %+v, want page 2 with 10 of 30", body)
	}
}

func TestTransaction_ListByWorkOrder(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	first := seedClientAndDevice(t, q, "Cliente WO Uno", "TX-WO-ONE")
	second := seedClientAndDevice(t, q, "Cliente WO Dos", "TX-WO-TWO")
	firstWO := seedWorkOrder(t, q, first.client.ID, first.device.ID)
	secondWO := seedWorkOrder(t, q, second.client.ID, second.device.ID)
	seedTransaction(t, q, transactionSeed{Type: "income", Category: "wo_payment", ClientID: first.client.ID, WorkOrderID: firstWO.ID, Date: "2026-05-01"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	firstBody := searchWorkOrderTransactions(t, client, ts.URL, uuidString(firstWO.Ucode))
	if len(firstBody.Transactions) != 1 || firstBody.Transactions[0].WorkOrder == nil {
		t.Fatalf("first transactions = %+v, want one work order transaction", firstBody.Transactions)
	}
	secondBody := searchWorkOrderTransactions(t, client, ts.URL, uuidString(secondWO.Ucode))
	if len(secondBody.Transactions) != 0 {
		t.Fatalf("second transactions = %+v, want none", secondBody.Transactions)
	}
}

func TestTransaction_UpdateLimited(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	tx := seedTransaction(t, q, transactionSeed{Category: "rent", Date: "2026-05-01", Description: "vieja"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := patchJSON(t, client, ts.URL+"/api/v1/transactions/"+uuidString(tx.Ucode), map[string]any{
		"transaction_date": "2026-05-10",
		"description":      "actualizada",
		"amount":           "999999.00",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body transactionBody
	decodeJSON(t, res.Body, &body)
	if body.Transaction.TransactionDate != "2026-05-10" || body.Transaction.Description == nil || *body.Transaction.Description != "actualizada" {
		t.Fatalf("transaction = %+v, want updated date and description", body.Transaction)
	}
	assertTransactionAmount(t, body.Transaction, "100.00")
}

func TestTransaction_DeleteIdempotent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	tx := seedTransaction(t, q, transactionSeed{Category: "rent", Date: "2026-05-01"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	for i := 0; i < 2; i++ {
		res := deleteReq(t, client, ts.URL+"/api/v1/transactions/"+uuidString(tx.Ucode), csrf)
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %d status = %d, want %d: %s", i+1, res.StatusCode, http.StatusNoContent, readBody(t, res))
		}
	}
}

func TestTransaction_VoidedExcludedFromList(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	keep := seedTransaction(t, q, transactionSeed{Category: "rent", Date: "2026-05-01"})
	voided := seedTransaction(t, q, transactionSeed{Category: "rent", Date: "2026-05-02"})
	if err := q.SoftDeleteTransaction(context.Background(), sqlc.SoftDeleteTransactionParams{ID: voided.ID}); err != nil {
		t.Fatal(err)
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	body := searchTransactions(t, client, ts.URL, "")
	if body.Total != 1 || len(body.Transactions) != 1 || body.Transactions[0].Ucode != uuidString(keep.Ucode) {
		t.Fatalf("body = %+v, want only non-voided transaction", body)
	}
}

type transactionBody struct {
	Transaction transactionDTO `json:"transaction"`
}

type transactionSearchBody struct {
	Transactions []transactionDTO `json:"transactions"`
	Total        int64            `json:"total"`
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
}

type workOrderTransactionsBody struct {
	Transactions []transactionDTO `json:"transactions"`
}

type transactionSeed struct {
	Type        string
	Amount      string
	Category    string
	Date        string
	Description string
	ClientID    int64
	SupplierID  int64
	WorkOrderID int64
}

func createTransaction(t *testing.T, client *http.Client, baseURL, csrf string, payload map[string]any, status int) transactionBody {
	t.Helper()
	res := postJSON(t, client, baseURL+"/api/v1/transactions", payload, csrf)
	defer res.Body.Close()
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
	var body transactionBody
	decodeJSON(t, res.Body, &body)
	return body
}

func searchTransactions(t *testing.T, client *http.Client, baseURL, query string) transactionSearchBody {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/transactions" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body transactionSearchBody
	decodeJSON(t, res.Body, &body)
	return body
}

func searchWorkOrderTransactions(t *testing.T, client *http.Client, baseURL, workOrderUcode string) workOrderTransactionsBody {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/work-orders/" + url.PathEscape(workOrderUcode) + "/transactions")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body workOrderTransactionsBody
	decodeJSON(t, res.Body, &body)
	return body
}

func seedTransaction(t *testing.T, q *sqlc.Queries, seed transactionSeed) sqlc.Transaction {
	t.Helper()
	if seed.Type == "" {
		seed.Type = "expense"
	}
	if seed.Amount == "" {
		seed.Amount = "100.00"
	}
	if seed.Category == "" {
		seed.Category = "rent"
	}
	if seed.Date == "" {
		seed.Date = "2026-05-17"
	}
	counterpartyType := "none"
	var clientID pgtype.Int8
	var supplierID pgtype.Int8
	if seed.ClientID != 0 {
		counterpartyType = "client"
		clientID = pgtype.Int8{Int64: seed.ClientID, Valid: true}
	}
	if seed.SupplierID != 0 {
		counterpartyType = "supplier"
		supplierID = pgtype.Int8{Int64: seed.SupplierID, Valid: true}
	}
	var workOrderID pgtype.Int8
	if seed.WorkOrderID != 0 {
		workOrderID = pgtype.Int8{Int64: seed.WorkOrderID, Valid: true}
	}
	amount, err := stringToNumeric(seed.Amount)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := stringToNumeric("1")
	if err != nil {
		t.Fatal(err)
	}
	date, ok := parseTransactionDate(noopResponseWriter{}, seed.Date)
	if !ok {
		t.Fatal("invalid seed date")
	}
	tx, err := q.CreateTransaction(context.Background(), sqlc.CreateTransactionParams{
		TransactionType:  seed.Type,
		Amount:           amount,
		Currency:         "ARS",
		FxRateToArs:      fx,
		TransactionDate:  date,
		PaymentMethod:    "transfer",
		Category:         seed.Category,
		CounterpartyType: counterpartyType,
		ClientID:         clientID,
		SupplierID:       supplierID,
		WorkOrderID:      workOrderID,
		Description:      textFromPtr(&seed.Description),
	})
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func seedWorkOrder(t *testing.T, q *sqlc.Queries, clientID, deviceID int64) sqlc.WorkOrder {
	t.Helper()
	wo, err := q.CreateWorkOrder(context.Background(), sqlc.CreateWorkOrderParams{
		ClientID:      clientID,
		DeviceID:      deviceID,
		ServiceType:   "in_shop",
		ReportedIssue: "No enciende",
	})
	if err != nil {
		t.Fatal(err)
	}
	return wo
}

func baseTransactionPayload(transactionType, amount, category, counterpartyType string) map[string]any {
	return map[string]any{
		"transaction_type":  transactionType,
		"amount":            amount,
		"payment_method":    "transfer",
		"category":          category,
		"counterparty_type": counterpartyType,
	}
}

func assertTransactionAmount(t *testing.T, tx transactionDTO, want string) {
	t.Helper()
	if tx.Amount != want {
		t.Fatalf("amount = %q, want %q", tx.Amount, want)
	}
}

type noopResponseWriter struct{}

func (noopResponseWriter) Header() http.Header        { return http.Header{} }
func (noopResponseWriter) Write([]byte) (int, error)  { return 0, nil }
func (noopResponseWriter) WriteHeader(statusCode int) {}
