package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestCreateClient_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/clients", map[string]any{
		"name":  "Juan Perez",
		"phone": "+54 9 11 1234 5678",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Client clientDTO `json:"client"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Client.Ucode == "" {
		t.Fatal("client.ucode is empty")
	}
	if body.Client.Phone == nil || *body.Client.Phone != "+5491112345678" {
		t.Fatalf("client.phone = %v, want +5491112345678", body.Client.Phone)
	}
}

func TestCreateClient_NormalizesPhone(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/clients", map[string]any{
		"name":  "Ana Lopez",
		"phone": "11 1234-5678",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var body struct {
		Client clientDTO `json:"client"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Client.Phone == nil || *body.Client.Phone != "+541112345678" {
		t.Fatalf("client.phone = %v, want +541112345678", body.Client.Phone)
	}
}

func TestCreateClient_DuplicatePhone(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	payload := map[string]any{"name": "Juan Perez", "phone": "+54 9 11 1234 5678"}
	first := postJSON(t, client, ts.URL+"/api/v1/clients", payload, csrf)
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first status = %d, want %d: %s", first.StatusCode, http.StatusCreated, readBody(t, first))
	}

	second := postJSON(t, client, ts.URL+"/api/v1/clients", map[string]any{
		"name":  "Otro Cliente",
		"phone": "+5491112345678",
	}, csrf)
	defer second.Body.Close()
	assertError(t, second, http.StatusConflict, "phone_already_exists")
}

func TestCreateClient_InvalidPhone(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/clients", map[string]any{
		"name":  "Juan Perez",
		"phone": "abc",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_phone")
}

func TestSearchClients_ByName(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)
	seedClient(t, q, "Ana Lopez", "")
	seedClient(t, q, "Juan Perez", "")
	seedClient(t, q, "Ana Garcia", "")

	res, err := client.Get(ts.URL + "/api/v1/clients?q=ana")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	body := decodeClientSearch(t, res)
	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}
	got := clientNames(body.Clients)
	want := []string{"Ana Garcia", "Ana Lopez"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestSearchClients_ByPhone(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)
	seedClient(t, q, "Juan Perez", "+5491112345678")
	seedClient(t, q, "Ana Garcia", "+5491112345679")

	res, err := client.Get(ts.URL + "/api/v1/clients?q=%2B5491112345678")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body := decodeClientSearch(t, res)
	if body.Total != 1 || len(body.Clients) != 1 || body.Clients[0].Name != "Juan Perez" {
		t.Fatalf("search result = total %d clients %v, want one Juan Perez", body.Total, clientNames(body.Clients))
	}
}

func TestSearchClients_ExcludesVoided(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)
	active := seedClient(t, q, "Ana Activa", "")
	voided := seedClient(t, q, "Ana Borrada", "")
	if err := q.SoftDeleteClient(context.Background(), sqlc.SoftDeleteClientParams{
		ID:             voided.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	res, err := client.Get(ts.URL + "/api/v1/clients?q=ana")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeClientSearch(t, res)
	if body.Total != 1 || len(body.Clients) != 1 || body.Clients[0].Ucode != uuidString(active.Ucode) {
		t.Fatalf("clients = %+v, want only active client", body.Clients)
	}
}

func TestSearchClients_Pagination(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)
	prefix := uniqueUsername(t) + "_cliente"
	for i := 0; i < 30; i++ {
		seedClient(t, q, fmt.Sprintf("%s %02d", prefix, i), "")
	}

	res, err := client.Get(ts.URL + "/api/v1/clients?q=" + prefix + "&page_size=10&page=2")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeClientSearch(t, res)
	if body.Total != 30 {
		t.Fatalf("total = %d, want 30", body.Total)
	}
	if len(body.Clients) != 10 {
		t.Fatalf("len(clients) = %d, want 10", len(body.Clients))
	}
	if body.Page != 2 || body.PageSize != 10 {
		t.Fatalf("page = %d page_size = %d, want 2/10", body.Page, body.PageSize)
	}
}

func TestGetClient_NotFound(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/clients/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

func TestUpdateClient_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	existing := seedClient(t, q, "Nombre Viejo", "+5491112345678")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := patchJSON(t, client, ts.URL+"/api/v1/clients/"+uuidString(existing.Ucode), map[string]any{
		"name":  "Nombre Nuevo",
		"phone": "+54 9 11 8765 4321",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	got, err := q.GetClientByUcode(context.Background(), existing.Ucode)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Nombre Nuevo" || !got.Phone.Valid || got.Phone.String != "+5491187654321" {
		t.Fatalf("updated client = name %q phone %q, want Nombre Nuevo/+5491187654321", got.Name, got.Phone.String)
	}
}

func TestDeleteClient_Idempotent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	existing := seedClient(t, q, "Cliente Borrable", "")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	url := ts.URL + "/api/v1/clients/" + uuidString(existing.Ucode)

	first := deleteReq(t, client, url, csrf)
	defer first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d: %s", first.StatusCode, http.StatusNoContent, readBody(t, first))
	}
	second := deleteReq(t, client, url, csrf)
	defer second.Body.Close()
	if second.StatusCode != http.StatusNoContent {
		t.Fatalf("second status = %d, want %d: %s", second.StatusCode, http.StatusNoContent, readBody(t, second))
	}
}

func TestListClientDevices_Empty(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	existing := seedClient(t, q, "Cliente Sin Equipos", "")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/clients/" + uuidString(existing.Ucode) + "/devices")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body struct {
		Devices []deviceDTO `json:"devices"`
	}
	decodeJSON(t, res.Body, &body)
	if len(body.Devices) != 0 {
		t.Fatalf("len(devices) = %d, want 0", len(body.Devices))
	}
}

func TestListClientDevices_EnrichedNoN1(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	existing := seedClient(t, q, "Cliente Con Equipos", "")
	brand := lookupBrandByName(t, q, "Samsung")
	articleType := lookupArticleTypeByName(t, q, "celular")
	for i := 0; i < 3; i++ {
		seedDevice(t, q, existing.ID, brand.ID, pgtype.Int8{}, articleType.ID, fmt.Sprintf("NO-N1-%d", i))
	}
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/clients/" + uuidString(existing.Ucode) + "/devices")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body struct {
		Devices []deviceDTO `json:"devices"`
	}
	decodeJSON(t, res.Body, &body)
	if len(body.Devices) != 3 {
		t.Fatalf("len(devices) = %d, want 3", len(body.Devices))
	}
	for _, device := range body.Devices {
		if device.Ucode == "" || device.BrandUcode == "" || device.ArticleTypeUcode == "" {
			t.Fatalf("device missing enriched ucodes: %+v", device)
		}
	}
}

func seedClient(t *testing.T, q *sqlc.Queries, name, phone string) sqlc.Client {
	t.Helper()

	var pgPhone pgtype.Text
	if phone != "" {
		pgPhone = pgtype.Text{String: phone, Valid: true}
	}
	client, err := q.CreateClient(context.Background(), sqlc.CreateClientParams{
		Name:            name,
		Phone:           pgPhone,
		ClientType:      "particular",
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func patchJSON(t *testing.T, client *http.Client, url string, payload any, csrf string) *http.Response {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func deleteReq(t *testing.T, client *http.Client, url, csrf string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

type clientSearchBody struct {
	Clients  []clientDTO `json:"clients"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func decodeClientSearch(t *testing.T, res *http.Response) clientSearchBody {
	t.Helper()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body clientSearchBody
	decodeJSON(t, res.Body, &body)
	return body
}

func clientNames(clients []clientDTO) []string {
	names := make([]string, 0, len(clients))
	for _, client := range clients {
		names = append(names, client.Name)
	}
	return names
}

func uniquePhone(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("+54911%08d", time.Now().UnixNano()%100000000)
}
