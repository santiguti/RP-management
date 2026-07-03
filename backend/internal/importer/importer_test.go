package importer

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseClients_OK(t *testing.T) {
	res, err := Parse(KindClients, fixture(t, []any{"name", "phone", "email", "client_type"}, [][]any{
		{"Juan Perez", "+5491155556666", "juan@example.com", "particular"},
		{"Acme SA", "+5491166667777", "ops@acme.com", "empresa"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalRows != 2 || res.ValidCount != 2 || res.InvalidCount != 0 || len(res.Errors) != 0 {
		t.Fatalf("result = %+v, want 2 valid rows", res)
	}
	if len(res.Preview) != 2 {
		t.Fatalf("preview len = %d, want 2", len(res.Preview))
	}
}

func TestParseClients_WithErrors(t *testing.T) {
	res, err := Parse(KindClients, fixture(t, []any{"name", "phone", "email", "client_type"}, [][]any{
		{"Valido", "+5491155556666", "valido@example.com", "particular"},
		{"", "+5491155556666", "bademail", "raro"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalRows != 2 || res.ValidCount != 1 || res.InvalidCount != 1 || len(res.Errors) != 3 {
		t.Fatalf("result = %+v, want 1 valid, 1 invalid, 3 errors", res)
	}
}

func TestParseClients_TrimWhitespace(t *testing.T) {
	res, err := Parse(KindClients, fixture(t, []any{" name ", " email "}, [][]any{
		{"  Juan Perez  ", "  juan@example.com  "},
	}))
	if err != nil {
		t.Fatal(err)
	}
	client := res.ValidRows[0].(ParsedClient)
	if client.Name != "Juan Perez" || client.Email == nil || *client.Email != "juan@example.com" {
		t.Fatalf("client = %+v, want trimmed values", client)
	}
}

func TestParseClients_DefaultsClientType(t *testing.T) {
	res, err := Parse(KindClients, fixture(t, []any{"name"}, [][]any{
		{"Sin Tipo"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	client := res.ValidRows[0].(ParsedClient)
	if client.ClientType != "particular" {
		t.Fatalf("client_type = %q, want particular", client.ClientType)
	}
}

func TestParseParts_OK(t *testing.T) {
	res, err := Parse(KindParts, fixture(t, []any{"name", "sku", "unit", "reorder_level", "default_cost", "default_sale_price"}, [][]any{
		{"Capacitor", "CAP-1", "unidad", "2", "100.50", "150.00"},
		{"Cable HDMI", "", "", "", "", ""},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalRows != 2 || res.ValidCount != 2 || res.InvalidCount != 0 {
		t.Fatalf("result = %+v, want 2 valid parts", res)
	}
	part := res.ValidRows[1].(ParsedPart)
	if part.Unit != "unidad" {
		t.Fatalf("unit = %q, want unidad", part.Unit)
	}
}

func TestParseParts_RejectsNegativeReorderLevel(t *testing.T) {
	res, err := Parse(KindParts, fixture(t, []any{"name", "reorder_level"}, [][]any{
		{"Capacitor", "-1"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.ValidCount != 0 || res.InvalidCount != 1 || len(res.Errors) != 1 || res.Errors[0].Column != "reorder_level" {
		t.Fatalf("result = %+v, want reorder_level error", res)
	}
}

func TestParseTransactions_OK(t *testing.T) {
	res, err := Parse(KindTransactions, fixture(t, []any{
		"transaction_type", "amount", "currency", "transaction_date", "payment_method", "category", "counterparty_type", "client_phone", "supplier_name", "wo_number", "description",
	}, [][]any{
		{"income", "1000.00", "ars", "2026-07-02", "cash", "wo_payment", "client", "+5491155556666", "", "WO-1", "Pago"},
		{"expense", "250.00", "", "", "transfer", "supplies", "supplier", "", "Proveedor SA", "", ""},
		{"expense", "50.00", "ARS", "2026-07-03", "other", "other_expense", "none", "", "", "", ""},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalRows != 3 || res.ValidCount != 3 || res.InvalidCount != 0 {
		t.Fatalf("result = %+v, want 3 valid transactions", res)
	}
	tx := res.ValidRows[0].(ParsedTransaction)
	if tx.Currency != "ARS" || tx.ClientPhone == nil || *tx.ClientPhone != "+5491155556666" {
		t.Fatalf("transaction = %+v, want normalized currency/phone", tx)
	}
}

func TestParseTransactions_RejectsCounterpartyMismatch(t *testing.T) {
	res, err := Parse(KindTransactions, fixture(t, []any{
		"transaction_type", "amount", "payment_method", "category", "counterparty_type", "client_phone",
	}, [][]any{
		{"income", "1000.00", "cash", "wo_payment", "client", ""},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.ValidCount != 0 || res.InvalidCount != 1 || len(res.Errors) == 0 || res.Errors[0].Column != "client_phone" {
		t.Fatalf("result = %+v, want client_phone error", res)
	}
}

func TestParseUnknownKind(t *testing.T) {
	_, err := Parse(Kind("unknown"), fixture(t, []any{"name"}, [][]any{{"x"}}))
	if !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("err = %v, want ErrUnknownKind", err)
	}
}

func TestParseEmptyFile(t *testing.T) {
	res, err := Parse(KindClients, emptyFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != KindClients || res.TotalRows != 0 || res.ValidCount != 0 || res.InvalidCount != 0 {
		t.Fatalf("result = %+v, want empty clients result", res)
	}
}

func fixture(t *testing.T, headers []any, rows [][]any) *bytes.Reader {
	t.Helper()
	f := excelize.NewFile()
	if err := f.SetSheetRow("Sheet1", "A1", &headers); err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	return workbookReader(t, f)
}

func emptyFixture(t *testing.T) *bytes.Reader {
	t.Helper()
	f := excelize.NewFile()
	return workbookReader(t, f)
}

func workbookReader(t *testing.T, f *excelize.File) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}
