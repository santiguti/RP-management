package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/money"
)

func TestParts_CreateOK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]string{
		"name": "  Cable flex  ",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Part partDTO `json:"part"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Part.Ucode == "" {
		t.Fatal("part.ucode is empty")
	}
	if body.Part.Name != "Cable flex" {
		t.Fatalf("part.name = %q, want trimmed name", body.Part.Name)
	}
	if body.Part.Unit != "unidad" {
		t.Fatalf("part.unit = %q, want unidad", body.Part.Unit)
	}
	if body.Part.CurrentStock != "0.00" {
		t.Fatalf("part.current_stock = %q, want 0.00", body.Part.CurrentStock)
	}
	if body.Part.LowStock {
		t.Fatal("part.low_stock = true, want false")
	}
}

func TestParts_CreateRequiresOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedUserWithRole(t, q, "employee")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]string{
		"name": "Cable flex",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestParts_CreateWithSkuOK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]any{
		"sku":                "  FLEX-001  ",
		"name":               "Flex LCD",
		"unit":               "metro",
		"reorder_level":      "2.50",
		"default_cost":       "1000.00",
		"default_sale_price": "1500.00",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Part partDTO `json:"part"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Part.Sku == nil || *body.Part.Sku != "FLEX-001" {
		t.Fatalf("part.sku = %v, want FLEX-001", body.Part.Sku)
	}
	if body.Part.Unit != "metro" {
		t.Fatalf("part.unit = %q, want metro", body.Part.Unit)
	}
	if body.Part.ReorderLevel == nil || *body.Part.ReorderLevel != "2.50" {
		t.Fatalf("part.reorder_level = %v, want 2.50", body.Part.ReorderLevel)
	}
}

func TestParts_DuplicateSku_409(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	first := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]string{
		"sku":  "BAT-001",
		"name": "Bateria A",
	}, csrf)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d: %s", first.StatusCode, http.StatusCreated, readBody(t, first))
	}

	duplicate := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]string{
		"sku":  "BAT-001",
		"name": "Bateria B",
	}, csrf)
	defer duplicate.Body.Close()
	assertError(t, duplicate, http.StatusConflict, "already_exists")
}

func TestParts_RejectsEmptyName_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]string{"name": "   "}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestParts_RejectsNegativeReorderLevel_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts", map[string]string{
		"name":          "Bateria",
		"reorder_level": "-1.00",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestParts_ListExcludesVoided(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	one := seedPart(t, q, partSeed{Name: "Display"})
	two := seedPart(t, q, partSeed{Name: "Parlante"})
	three := seedPart(t, q, partSeed{Name: "Pin de carga"})
	if err := q.SoftDeletePart(context.Background(), sqlc.SoftDeletePartParams{
		ID:             two.ID,
		VoidedByUserID: pgtype.Int8{},
	}); err != nil {
		t.Fatal(err)
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	body := decodePartSearch(t, res)
	if len(body.Parts) != 2 {
		t.Fatalf("parts = %+v, want 2 live parts", body.Parts)
	}
	names := partNames(body.Parts)
	if !hasName(names, one.Name) || !hasName(names, three.Name) || hasName(names, two.Name) {
		t.Fatalf("part names = %v, want %q and %q only", names, one.Name, three.Name)
	}
}

func TestParts_LowStockFilter(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	low := seedPart(t, q, partSeed{Name: "Bateria baja", ReorderLevel: "10.00"})
	okStock := seedPart(t, q, partSeed{Name: "Bateria ok", ReorderLevel: "10.00"})
	insertPartMovement(t, low.ID, "purchase", "5.00")
	insertPartMovement(t, okStock.ID, "purchase", "15.00")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts?low_stock=true")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	body := decodePartSearch(t, res)
	if len(body.Parts) != 1 {
		t.Fatalf("parts = %+v, want only low stock part", body.Parts)
	}
	if body.Parts[0].Name != low.Name || !body.Parts[0].LowStock {
		t.Fatalf("part = %+v, want low stock %q", body.Parts[0], low.Name)
	}
}

func TestParts_SearchByName(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedPart(t, q, partSeed{Name: "Modulo Samsung"})
	seedPart(t, q, partSeed{Name: "Bateria Motorola"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts?q=sam")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	body := decodePartSearch(t, res)
	if len(body.Parts) != 1 || body.Parts[0].Name != "Modulo Samsung" {
		t.Fatalf("parts = %+v, want Modulo Samsung only", body.Parts)
	}
}

func TestParts_Pagination(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	seedPart(t, q, partSeed{Name: "A Cable"})
	seedPart(t, q, partSeed{Name: "B Cable"})
	seedPart(t, q, partSeed{Name: "C Cable"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts?page=2&page_size=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	body := decodePartSearch(t, res)
	if body.Total != 3 || body.Page != 2 || body.PageSize != 1 {
		t.Fatalf("pagination = total %d page %d page_size %d, want 3/2/1", body.Total, body.Page, body.PageSize)
	}
	if len(body.Parts) != 1 || body.Parts[0].Name != "B Cable" {
		t.Fatalf("parts = %+v, want B Cable", body.Parts)
	}
}

func TestParts_GetNotFound_404(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

func TestParts_UpdateOK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Nombre viejo", Sku: "OLD"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	patch := patchJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode), map[string]any{
		"name":               "Nombre nuevo",
		"sku":                "NEW",
		"unit":               "metro",
		"default_sale_price": "2500.00",
	}, csrf)
	defer patch.Body.Close()
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want %d: %s", patch.StatusCode, http.StatusOK, readBody(t, patch))
	}

	var body struct {
		Part partDTO `json:"part"`
	}
	decodeJSON(t, patch.Body, &body)
	if body.Part.Name != "Nombre nuevo" || body.Part.Sku == nil || *body.Part.Sku != "NEW" || body.Part.Unit != "metro" {
		t.Fatalf("patched part = %+v, want updated values", body.Part)
	}
	if body.Part.DefaultSalePrice == nil || *body.Part.DefaultSalePrice != "2500.00" {
		t.Fatalf("default_sale_price = %v, want 2500.00", body.Part.DefaultSalePrice)
	}
}

func TestParts_UpdateRequiresOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedUserWithRole(t, q, "employee")
	part := seedPart(t, q, partSeed{Name: "Nombre viejo", Sku: "OLD"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := patchJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode), map[string]any{
		"name": "Nombre nuevo",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestParts_DeleteIdempotent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Borrable"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	for i := 0; i < 2; i++ {
		res := deleteReq(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode), csrf)
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %d status = %d, want %d: %s", i+1, res.StatusCode, http.StatusNoContent, readBody(t, res))
		}
	}
}

func TestParts_DeleteRequiresOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedUserWithRole(t, q, "employee")
	part := seedPart(t, q, partSeed{Name: "Borrable"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := deleteReq(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode), csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

func TestMovement_CreatePurchase_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Flex"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]string{
		"movement_type": "purchase",
		"quantity":      "5",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Movement movementDTO `json:"movement"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Movement.MovementType != "purchase" || body.Movement.Quantity != "5.00" {
		t.Fatalf("movement = %+v, want purchase +5.00", body.Movement)
	}
	updated, err := q.GetPartByID(context.Background(), part.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := money.NumericToString(updated.CurrentStock); got != "5.00" {
		t.Fatalf("current_stock = %q, want 5.00", got)
	}
}

func TestMovement_CreatePurchase_WithAutoTransaction(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Display"})
	supplier := seedSupplier(t, q, "Proveedor de displays")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]any{
		"movement_type":    "purchase",
		"quantity":         "5",
		"unit_cost":        "100.00",
		"payment_method":   "cash",
		"supplier_ucode":   uuidString(supplier.Ucode),
		"link_transaction": true,
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Movement movementDTO `json:"movement"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Movement.Transaction == nil || body.Movement.Transaction.Ucode == "" {
		t.Fatalf("movement.transaction = %+v, want linked transaction", body.Movement.Transaction)
	}
	if body.Movement.Supplier == nil || body.Movement.Supplier.Ucode != uuidString(supplier.Ucode) {
		t.Fatalf("movement.supplier = %+v, want supplier", body.Movement.Supplier)
	}

	var amount pgtype.Numeric
	var paymentMethod, category, counterpartyType string
	if err := testPool.QueryRow(context.Background(), `
SELECT amount, payment_method, category, counterparty_type
FROM rp.transactions
WHERE ucode = $1 AND voided_ts IS NULL
`, body.Movement.Transaction.Ucode).Scan(&amount, &paymentMethod, &category, &counterpartyType); err != nil {
		t.Fatal(err)
	}
	if got := money.NumericToString(amount); got != "500.00" || paymentMethod != "cash" || category != "part_purchase" || counterpartyType != "supplier" {
		t.Fatalf("transaction = amount %q payment %q category %q counterparty %q, want 500.00/cash/part_purchase/supplier", got, paymentMethod, category, counterpartyType)
	}
}

func TestMovement_AutoTransactionRequiresPaymentMethod_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Display"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]any{
		"movement_type":    "purchase",
		"quantity":         "5",
		"unit_cost":        "100.00",
		"link_transaction": true,
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "payment_method_required")
}

func TestMovement_AdjustmentIn_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]any{
		"movement_type":  "adjustment",
		"quantity":       "3",
		"adjustment_out": false,
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	updated, err := q.GetPartByID(context.Background(), part.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := money.NumericToString(updated.CurrentStock); got != "3.00" {
		t.Fatalf("current_stock = %q, want 3.00", got)
	}
}

func TestMovement_AdjustmentOut_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	insertPartMovement(t, part.ID, "purchase", "5.00")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]any{
		"movement_type":  "adjustment",
		"quantity":       "2",
		"adjustment_out": true,
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	updated, err := q.GetPartByID(context.Background(), part.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := money.NumericToString(updated.CurrentStock); got != "3.00" {
		t.Fatalf("current_stock = %q, want 3.00", got)
	}
}

func TestMovement_AdjustmentOut_InsufficientStock_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	insertPartMovement(t, part.ID, "purchase", "1.00")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]any{
		"movement_type":  "adjustment",
		"quantity":       "5",
		"adjustment_out": true,
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "insufficient_stock")
}

func TestMovement_DBConstraintRejectsNegativeStock(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	part := seedPart(t, q, partSeed{Name: "Backstop"})

	_, err := q.CreatePartMovement(context.Background(), sqlc.CreatePartMovementParams{
		PartID:       part.ID,
		MovementType: "use",
		Quantity:     optionalTestNumeric(t, "-1.00"),
	})
	if !isCheckViolation(err) {
		t.Fatalf("err = %v, want check violation", err)
	}
}

func TestMovement_RejectsUseFromManualEndpoint_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]string{
		"movement_type": "use",
		"quantity":      "1",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "use_movements_via_work_order_only")
}

func TestMovement_RejectsZeroQuantity_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/parts/"+uuidString(part.Ucode)+"/movements", map[string]string{
		"movement_type": "purchase",
		"quantity":      "0",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestMovement_List_OrdersByCreatedDesc(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	first := insertPartMovement(t, part.ID, "purchase", "1.00")
	second := insertPartMovement(t, part.ID, "return", "2.00")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts/" + uuidString(part.Ucode) + "/movements")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	body := decodeMovementSearch(t, res)
	if len(body.Movements) != 2 || body.Movements[0].Ucode != uuidString(second.Ucode) || body.Movements[1].Ucode != uuidString(first.Ucode) {
		t.Fatalf("movements = %+v, want newest first", body.Movements)
	}
}

func TestMovement_ListPagination(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	first := insertPartMovement(t, part.ID, "purchase", "1.00")
	insertPartMovement(t, part.ID, "return", "2.00")
	insertPartMovement(t, part.ID, "purchase", "3.00")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts/" + uuidString(part.Ucode) + "/movements?page=2&page_size=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeMovementSearch(t, res)
	if body.Total != 3 || body.Page != 2 || body.PageSize != 1 || len(body.Movements) != 1 || body.Movements[0].Quantity != "2.00" {
		t.Fatalf("movement page = %+v, want second newest row", body)
	}
	if body.Movements[0].Ucode == uuidString(first.Ucode) {
		t.Fatalf("movement = %+v, want second newest row", body.Movements[0])
	}
}

func TestMovement_ListExcludesVoided(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	part := seedPart(t, q, partSeed{Name: "Conector"})
	voided := insertPartMovement(t, part.ID, "purchase", "1.00")
	live := insertPartMovement(t, part.ID, "return", "2.00")
	if err := q.SoftDeletePartMovement(context.Background(), sqlc.SoftDeletePartMovementParams{ID: voided.ID}); err != nil {
		t.Fatal(err)
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/parts/" + uuidString(part.Ucode) + "/movements")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeMovementSearch(t, res)
	if len(body.Movements) != 1 || body.Movements[0].Ucode != uuidString(live.Ucode) {
		t.Fatalf("movements = %+v, want only live movement", body.Movements)
	}
}

type partSeed struct {
	Name         string
	Sku          string
	Unit         string
	ReorderLevel string
	DefaultCost  string
}

func seedPart(t *testing.T, q *sqlc.Queries, seed partSeed) sqlc.Part {
	t.Helper()
	if seed.Name == "" {
		seed.Name = "Repuesto"
	}
	if seed.Unit == "" {
		seed.Unit = "unidad"
	}
	var sku pgtype.Text
	if seed.Sku != "" {
		sku = pgtype.Text{String: seed.Sku, Valid: true}
	}
	part, err := q.CreatePart(context.Background(), sqlc.CreatePartParams{
		Sku:             sku,
		Name:            seed.Name,
		Unit:            seed.Unit,
		ReorderLevel:    optionalTestNumeric(t, seed.ReorderLevel),
		DefaultCost:     optionalTestNumeric(t, seed.DefaultCost),
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return part
}

func insertPartMovement(t *testing.T, partID int64, movementType, quantity string) sqlc.PartMovement {
	t.Helper()
	partMovement, err := sqlc.New(testPool).CreatePartMovement(context.Background(), sqlc.CreatePartMovementParams{
		PartID:       partID,
		MovementType: movementType,
		Quantity:     optionalTestNumeric(t, quantity),
	})
	if err != nil {
		t.Fatal(err)
	}
	return partMovement
}

func optionalTestNumeric(t *testing.T, raw string) pgtype.Numeric {
	t.Helper()
	if raw == "" {
		return pgtype.Numeric{}
	}
	n, err := money.StringToNumeric(raw)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

type partSearchBody struct {
	Parts    []partDTO `json:"parts"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

func decodePartSearch(t *testing.T, res *http.Response) partSearchBody {
	t.Helper()
	var body partSearchBody
	decodeJSON(t, res.Body, &body)
	return body
}

func partNames(items []partDTO) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

type movementSearchBody struct {
	Movements []movementDTO `json:"movements"`
	Total     int64         `json:"total"`
	Page      int           `json:"page"`
	PageSize  int           `json:"page_size"`
}

func decodeMovementSearch(t *testing.T, res *http.Response) movementSearchBody {
	t.Helper()
	var body movementSearchBody
	decodeJSON(t, res.Body, &body)
	return body
}
