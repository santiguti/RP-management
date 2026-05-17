package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestSuppliers_CreateOK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/suppliers", map[string]string{
		"name":  "  Repuestos del Sur  ",
		"phone": "11 4444-5555",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Supplier supplierDTO `json:"supplier"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Supplier.Ucode == "" {
		t.Fatal("supplier.ucode is empty")
	}
	if body.Supplier.Name != "Repuestos del Sur" {
		t.Fatalf("supplier.name = %q, want trimmed name", body.Supplier.Name)
	}
	if body.Supplier.Phone == nil || *body.Supplier.Phone != "11 4444-5555" {
		t.Fatalf("supplier.phone = %v, want provided phone", body.Supplier.Phone)
	}
}

func TestSuppliers_CreateDuplicate_409(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	first := postJSON(t, client, ts.URL+"/api/v1/suppliers", map[string]string{"name": "Repuestos SA"}, csrf)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d: %s", first.StatusCode, http.StatusCreated, readBody(t, first))
	}

	duplicate := postJSON(t, client, ts.URL+"/api/v1/suppliers", map[string]string{"name": "repuestos sa"}, csrf)
	defer duplicate.Body.Close()
	assertError(t, duplicate, http.StatusConflict, "already_exists")
}

func TestSuppliers_CreateInvalidEmail_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/suppliers", map[string]string{
		"name":  "Proveedor",
		"email": "no-es-email",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestSuppliers_ListExcludesVoided(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	one := seedSupplier(t, q, "Proveedor Uno")
	two := seedSupplier(t, q, "Proveedor Dos")
	three := seedSupplier(t, q, "Proveedor Tres")
	if err := q.SoftDeleteSupplier(context.Background(), sqlc.SoftDeleteSupplierParams{
		ID:             two.ID,
		VoidedByUserID: pgtype.Int8{},
	}); err != nil {
		t.Fatal(err)
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/suppliers")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	var body struct {
		Suppliers []supplierDTO `json:"suppliers"`
	}
	decodeJSON(t, res.Body, &body)
	if len(body.Suppliers) != 2 {
		t.Fatalf("suppliers = %+v, want 2 live suppliers", body.Suppliers)
	}
	names := supplierNames(body.Suppliers)
	if !hasName(names, one.Name) || !hasName(names, three.Name) || hasName(names, two.Name) {
		t.Fatalf("supplier names = %v, want %q and %q only", names, one.Name, three.Name)
	}
}

func TestSuppliers_GetNotFound_404(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/suppliers/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

func TestSuppliers_UpdateOK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	supplier := seedSupplier(t, q, "Proveedor Viejo")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	updatedName := "Proveedor Nuevo"
	patch := patchJSON(t, client, ts.URL+"/api/v1/suppliers/"+uuidString(supplier.Ucode), map[string]string{
		"name": updatedName,
	}, csrf)
	defer patch.Body.Close()
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want %d: %s", patch.StatusCode, http.StatusOK, readBody(t, patch))
	}
	var patchBody struct {
		Supplier supplierDTO `json:"supplier"`
	}
	decodeJSON(t, patch.Body, &patchBody)
	if patchBody.Supplier.Name != updatedName {
		t.Fatalf("patched supplier.name = %q, want %q", patchBody.Supplier.Name, updatedName)
	}

	get, err := client.Get(ts.URL + "/api/v1/suppliers/" + uuidString(supplier.Ucode))
	if err != nil {
		t.Fatal(err)
	}
	defer get.Body.Close()
	if get.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want %d: %s", get.StatusCode, http.StatusOK, readBody(t, get))
	}
	var getBody struct {
		Supplier supplierDTO `json:"supplier"`
	}
	decodeJSON(t, get.Body, &getBody)
	if getBody.Supplier.Name != updatedName {
		t.Fatalf("fetched supplier.name = %q, want %q", getBody.Supplier.Name, updatedName)
	}
}

func TestSuppliers_DeleteIdempotent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	supplier := seedSupplier(t, q, "Proveedor Borrable")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	for i := 0; i < 2; i++ {
		res := deleteReq(t, client, ts.URL+"/api/v1/suppliers/"+uuidString(supplier.Ucode), csrf)
		defer res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %d status = %d, want %d: %s", i+1, res.StatusCode, http.StatusNoContent, readBody(t, res))
		}
	}
}

func seedSupplier(t *testing.T, q *sqlc.Queries, name string) sqlc.Supplier {
	t.Helper()

	supplier, err := q.CreateSupplier(context.Background(), sqlc.CreateSupplierParams{
		Name:            name,
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return supplier
}

func supplierNames(items []supplierDTO) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}
