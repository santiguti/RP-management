package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestImport_ClientsDryRun_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "clients", false, clientsImportWorkbook(t, [][]any{
		{"Cliente Uno", "+5491155556666", "uno@example.com", "particular"},
		{"Cliente Dos", "+5491166667777", "dos@example.com", "empresa"},
	}))
	defer res.Body.Close()
	body := decodeImportDryRun(t, res, http.StatusOK)
	if body.Valid != 2 || body.Invalid != 0 || body.Committed || len(body.Preview) != 2 {
		t.Fatalf("body = %+v, want 2 valid dry-run rows", body)
	}
	if got := countImportClients(t); got != 0 {
		t.Fatalf("clients = %d, want 0 dry-run inserts", got)
	}
}

func TestImport_ClientsDryRun_WithErrors(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "clients", false, clientsImportWorkbook(t, [][]any{
		{"Cliente Uno", "+5491155556666", "uno@example.com", "particular"},
		{"Cliente Dos", "+5491166667777", "dos@example.com", "empresa"},
		{"", "+5491177778888", "bademail", "raro"},
	}))
	defer res.Body.Close()
	body := decodeImportDryRun(t, res, http.StatusOK)
	if body.Valid != 2 || body.Invalid != 1 || len(body.Errors) < 1 || len(body.Preview) != 2 {
		t.Fatalf("body = %+v, want 2 valid, 1 invalid", body)
	}
}

func TestImport_ClientsConfirm_InsertsAll(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "clients", true, clientsImportWorkbook(t, [][]any{
		{"Cliente Uno", "+5491155556666", "uno@example.com", "particular"},
		{"Cliente Dos", "+5491166667777", "dos@example.com", "empresa"},
	}))
	defer res.Body.Close()
	body := decodeImportCommit(t, res, http.StatusCreated)
	if !body.Committed || body.Valid != 2 || len(body.InsertedUcodes) != 2 {
		t.Fatalf("body = %+v, want committed 2 rows", body)
	}
	if got := countImportClients(t); got != 2 {
		t.Fatalf("clients = %d, want 2", got)
	}
	if got := countImportAudit(t); got != 1 {
		t.Fatalf("import audit rows = %d, want 1", got)
	}
}

func TestImport_ClientsConfirm_DuplicatePhone_AbortsBatch(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedClient(t, q, "Existente", "+5491155556666")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "clients", true, clientsImportWorkbook(t, [][]any{
		{"Duplicado", "+5491155556666", "dup@example.com", "particular"},
		{"Nuevo", "+5491166667777", "nuevo@example.com", "empresa"},
	}))
	defer res.Body.Close()
	body := decodeImportConflict(t, res)
	if body.Error != "commit_failed" || len(body.Errors) == 0 {
		t.Fatalf("body = %+v, want commit_failed with errors", body)
	}
	if got := countImportClients(t); got != 1 {
		t.Fatalf("clients = %d, want only preseeded client", got)
	}
	if got := countImportAudit(t); got != 0 {
		t.Fatalf("import audit rows = %d, want 0", got)
	}
}

func TestImport_PartsConfirm_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "parts", true, workbook(t,
		[]any{"name", "sku", "unit", "reorder_level", "default_cost", "default_sale_price"},
		[][]any{
			{"Capacitor", "CAP-IMPORT-1", "unidad", "2", "100.00", "150.00"},
			{"Cable", "CAB-IMPORT-1", "", "", "", ""},
		},
	))
	defer res.Body.Close()
	body := decodeImportCommit(t, res, http.StatusCreated)
	if body.Valid != 2 || len(body.InsertedUcodes) != 2 {
		t.Fatalf("body = %+v, want 2 inserted parts", body)
	}
}

func TestImport_TransactionsConfirm_ResolvesCounterparties(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Import Tx", "IMPORT-TX")
	seedClient(t, q, "Cliente Import Pago", "+5491155556666")
	supplier := seedSupplier(t, q, "Proveedor Import")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	workOrder := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	res := postImportXLSX(t, client, ts.URL, csrf, "transactions", true, transactionsImportWorkbook(t, [][]any{
		{"income", "1000.00", "ARS", "2026-07-02", "cash", "wo_payment", "client", "+5491155556666", "", workOrder.WorkOrder.WoNumber, "Pago"},
		{"expense", "250.00", "ARS", "2026-07-02", "transfer", "supplies", "supplier", "", supplier.Name, "", "Insumos"},
	}))
	defer res.Body.Close()
	body := decodeImportCommit(t, res, http.StatusCreated)
	if body.Valid != 2 || len(body.InsertedUcodes) != 2 {
		t.Fatalf("body = %+v, want 2 inserted transactions", body)
	}
}

func TestImport_TransactionsConfirm_UnknownSupplier_AbortsBatch(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "transactions", true, transactionsImportWorkbook(t, [][]any{
		{"expense", "250.00", "ARS", "2026-07-02", "transfer", "supplies", "supplier", "", "No Existe SA", "", "Insumos"},
	}))
	defer res.Body.Close()
	body := decodeImportConflict(t, res)
	if body.Error != "commit_failed" || len(body.Errors) != 1 || body.Errors[0].Column != "supplier_name" {
		t.Fatalf("body = %+v, want supplier_name commit error", body)
	}
	if got := countImportTransactions(t); got != 0 {
		t.Fatalf("transactions = %d, want 0", got)
	}
}

func TestImport_RejectsInvalidKind_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "bad", false, clientsImportWorkbook(t, [][]any{{"Cliente", "", "", ""}}))
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_kind")
}

func TestImport_RejectsMissingFile_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/import/excel?kind=clients", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "missing_file")
}

func TestImport_AsEmployee_403(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	employee := seedUserWithRole(t, q, "employee")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, employee.Username)

	res := postImportXLSX(t, client, ts.URL, csrf, "clients", false, clientsImportWorkbook(t, [][]any{{"Cliente", "", "", ""}}))
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

type importDryRunBody struct {
	Committed bool              `json:"committed"`
	Valid     int               `json:"valid"`
	Invalid   int               `json:"invalid"`
	Preview   []map[string]any  `json:"preview"`
	Errors    []importRowErrDTO `json:"errors"`
}

type importCommitBody struct {
	Committed      bool     `json:"committed"`
	Valid          int      `json:"valid"`
	InsertedUcodes []string `json:"inserted_ucodes"`
}

type importConflictBody struct {
	Error  string            `json:"error"`
	Errors []importRowErrDTO `json:"errors"`
}

type importRowErrDTO struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

func postImportXLSX(t *testing.T, client *http.Client, baseURL, csrf, kind string, confirm bool, content []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	url := baseURL + "/api/v1/import/excel?kind=" + kind
	if confirm {
		url += "&confirm=true"
	}
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decodeImportDryRun(t *testing.T, res *http.Response, status int) importDryRunBody {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
	var body importDryRunBody
	decodeJSON(t, res.Body, &body)
	return body
}

func decodeImportCommit(t *testing.T, res *http.Response, status int) importCommitBody {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
	var body importCommitBody
	decodeJSON(t, res.Body, &body)
	return body
}

func decodeImportConflict(t *testing.T, res *http.Response) importConflictBody {
	t.Helper()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusConflict, readBody(t, res))
	}
	var body importConflictBody
	decodeJSON(t, res.Body, &body)
	return body
}

func clientsImportWorkbook(t *testing.T, rows [][]any) []byte {
	t.Helper()
	return workbook(t, []any{"name", "phone", "email", "client_type"}, rows)
}

func transactionsImportWorkbook(t *testing.T, rows [][]any) []byte {
	t.Helper()
	return workbook(t, []any{
		"transaction_type", "amount", "currency", "transaction_date", "payment_method", "category", "counterparty_type", "client_phone", "supplier_name", "wo_number", "description",
	}, rows)
}

func workbook(t *testing.T, headers []any, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	if err := f.SetSheetRow("Sheet1", "A1", &headers); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if err := f.SetSheetRow("Sheet1", fmt.Sprintf("A%d", i+2), &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func countImportClients(t *testing.T) int64 {
	t.Helper()
	return countImportRows(t, `SELECT count(*)::bigint FROM rp.clients WHERE voided_ts IS NULL`)
}

func countImportTransactions(t *testing.T) int64 {
	t.Helper()
	return countImportRows(t, `SELECT count(*)::bigint FROM rp.transactions WHERE voided_ts IS NULL`)
}

func countImportAudit(t *testing.T) int64 {
	t.Helper()
	return countImportRows(t, `SELECT count(*)::bigint FROM rp.audit_log WHERE action = 'import.commit'`)
}

func countImportRows(t *testing.T, query string) int64 {
	t.Helper()
	var count int64
	if err := testPool.QueryRow(context.Background(), query).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
