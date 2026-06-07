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

type partSeed struct {
	Name         string
	Sku          string
	ReorderLevel string
}

func seedPart(t *testing.T, q *sqlc.Queries, seed partSeed) sqlc.Part {
	t.Helper()
	if seed.Name == "" {
		seed.Name = "Repuesto"
	}
	var sku pgtype.Text
	if seed.Sku != "" {
		sku = pgtype.Text{String: seed.Sku, Valid: true}
	}
	part, err := q.CreatePart(context.Background(), sqlc.CreatePartParams{
		Sku:             sku,
		Name:            seed.Name,
		Unit:            "unidad",
		ReorderLevel:    optionalTestNumeric(t, seed.ReorderLevel),
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return part
}

func insertPartMovement(t *testing.T, partID int64, movementType, quantity string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `
INSERT INTO rp.part_movements (part_id, movement_type, quantity)
VALUES ($1, $2, $3)
`, partID, movementType, quantity)
	if err != nil {
		t.Fatal(err)
	}
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
