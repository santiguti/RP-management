package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestExport_TransactionsCSV_Headers(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "100.00", Category: "rent", Date: "2026-01-05", Description: "Alquiler"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	records := getCSV(t, client, ts.URL+"/api/v1/transactions.csv")
	if strings.Join(records[0], ",") != "Fecha,Tipo,Categoría,Método,Contraparte,Orden,Monto,Moneda,Descripción" {
		t.Fatalf("header = %v", records[0])
	}
	if len(records) < 2 || records[1][1] != "Egreso" || records[1][2] != "Alquiler" {
		t.Fatalf("records = %v, want translated transaction row", records)
	}
}

func TestExport_TransactionsCSV_FiltersHonored(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedTransaction(t, q, transactionSeed{Type: "income", Amount: "100.00", Category: "other_income", Date: "2026-01-05"})
	seedTransaction(t, q, transactionSeed{Type: "expense", Amount: "50.00", Category: "rent", Date: "2026-01-05"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	records := getCSV(t, client, ts.URL+"/api/v1/transactions.csv?type=income")
	if len(records) != 2 || records[1][1] != "Ingreso" {
		t.Fatalf("records = %v, want only income row", records)
	}
}

func TestExport_TransactionsCSV_PagesPastLimit(t *testing.T) {
	previousPageSize := exportPageSize
	exportPageSize = 3
	t.Cleanup(func() { exportPageSize = previousPageSize })

	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	for i := 0; i < 12; i++ {
		seedTransaction(t, q, transactionSeed{
			Category:    "rent",
			Date:        fmt.Sprintf("2026-05-%02d", i+1),
			Description: fmt.Sprintf("tx page %02d", i),
		})
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	records := getCSV(t, client, ts.URL+"/api/v1/transactions.csv")
	if len(records) != 13 {
		t.Fatalf("records len = %d, want 13: %v", len(records), records)
	}
	descriptions := make(map[string]bool, len(records)-1)
	for _, record := range records[1:] {
		descriptions[record[8]] = true
	}
	for i := 0; i < 12; i++ {
		want := fmt.Sprintf("tx page %02d", i)
		if !descriptions[want] {
			t.Fatalf("missing description %q in %v", want, descriptions)
		}
	}
}

func TestExport_ClientsCSV_Smoke(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedClient(t, q, "Cliente CSV", "+5491155556666")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	records := getCSV(t, client, ts.URL+"/api/v1/clients.csv")
	if len(records) != 2 || records[0][0] != "Nombre" || records[1][0] != "Cliente CSV" {
		t.Fatalf("records = %v, want client csv row", records)
	}
}

func TestExport_PartsCSV_Smoke(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedPart(t, q, partSeed{Name: "Parte CSV", Sku: "CSV-1", DefaultCost: "10.00"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	records := getCSV(t, client, ts.URL+"/api/v1/parts.csv")
	if len(records) != 2 || records[0][0] != "Nombre" || records[1][0] != "Parte CSV" {
		t.Fatalf("records = %v, want part csv row", records)
	}
}

func TestExport_WorkOrdersCSV_Smoke(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente WO CSV", "WO-CSV")
	seedWorkOrder(t, q, fixture.client.ID, fixture.device.ID)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	records := getCSV(t, client, ts.URL+"/api/v1/work-orders.csv")
	if len(records) != 2 || records[0][0] != "Número" || records[1][1] != "Recibido" {
		t.Fatalf("records = %v, want work order csv row", records)
	}
}

func TestExport_CSV_HasUTF8BOM(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/clients.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("missing UTF-8 BOM: %v", raw[:min(3, len(raw))])
	}
}

func getCSV(t *testing.T, client *http.Client, url string) [][]string {
	t.Helper()
	res, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	records, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}
