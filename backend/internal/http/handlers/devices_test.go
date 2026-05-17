package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestCreateDevice_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/devices", map[string]string{
		"client_ucode":       uuidString(fixture.client.Ucode),
		"brand_ucode":        uuidString(fixture.brand.Ucode),
		"article_type_ucode": uuidString(fixture.articleType.Ucode),
		"serial_number":      "ABC123",
		"color":              "negro",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	var body struct {
		Device deviceDTO `json:"device"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Device.ClientUcode != uuidString(fixture.client.Ucode) ||
		body.Device.BrandUcode != uuidString(fixture.brand.Ucode) ||
		body.Device.ArticleTypeUcode != uuidString(fixture.articleType.Ucode) {
		t.Fatalf("device = %+v, want fixture ucodes", body.Device)
	}
	if body.Device.SerialNumber == nil || *body.Device.SerialNumber != "ABC123" {
		t.Fatalf("serial_number = %v, want ABC123", body.Device.SerialNumber)
	}
}

func TestCreateDevice_UnknownClient(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/devices", map[string]string{
		"client_ucode":       "00000000-0000-0000-0000-000000000000",
		"brand_ucode":        uuidString(fixture.brand.Ucode),
		"article_type_ucode": uuidString(fixture.articleType.Ucode),
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "unknown_client")
}

func TestCreateDevice_UnknownBrand(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/devices", map[string]string{
		"client_ucode":       uuidString(fixture.client.Ucode),
		"brand_ucode":        "00000000-0000-0000-0000-000000000000",
		"article_type_ucode": uuidString(fixture.articleType.Ucode),
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "unknown_brand")
}

func TestCreateDevice_OptionalModel(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/devices", map[string]string{
		"client_ucode":       uuidString(fixture.client.Ucode),
		"brand_ucode":        uuidString(fixture.brand.Ucode),
		"article_type_ucode": uuidString(fixture.articleType.Ucode),
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	var body struct {
		Device deviceDTO `json:"device"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Device.ModelUcode != nil {
		t.Fatalf("model_ucode = %v, want nil", body.Device.ModelUcode)
	}
}

func TestSearchDevices_ByClient(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	other := seedClient(t, q, "Otro Cliente", "")
	seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "AAA111")
	seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "AAA222")
	seedDevice(t, q, other.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "AAA333")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/devices?client_ucode=" + uuidString(fixture.client.Ucode))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeDeviceSearch(t, res)
	if len(body.Devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(body.Devices))
	}
	for _, device := range body.Devices {
		if device.ClientUcode != uuidString(fixture.client.Ucode) {
			t.Fatalf("device client_ucode = %q, want %q", device.ClientUcode, uuidString(fixture.client.Ucode))
		}
	}
}

func TestSearchDevices_BySerial(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "ABC111")
	seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "ABC222")
	seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "XYZ333")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/devices?serial=ABC")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeDeviceSearch(t, res)
	if len(body.Devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(body.Devices))
	}
	for _, device := range body.Devices {
		if device.SerialNumber == nil || !strings.HasPrefix(*device.SerialNumber, "ABC") {
			t.Fatalf("serial_number = %v, want ABC prefix", device.SerialNumber)
		}
	}
}

func TestSearchDevices_NoParams_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/devices")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "client_ucode_or_serial_required")
}

func TestUpdateDevice_ChangeClient(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	newClient := seedClient(t, q, "Nuevo Duenio", "")
	device := seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "MOV123")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := patchJSON(t, client, ts.URL+"/api/v1/devices/"+uuidString(device.Ucode), map[string]string{
		"client_ucode": uuidString(newClient.Ucode),
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body struct {
		Device deviceDTO `json:"device"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Device.ClientUcode != uuidString(newClient.Ucode) {
		t.Fatalf("client_ucode = %q, want %q", body.Device.ClientUcode, uuidString(newClient.Ucode))
	}
}

func TestDeleteDevice_Idempotent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedDeviceFixture(t, q)
	device := seedDevice(t, q, fixture.client.ID, fixture.brand.ID, pgtype.Int8{}, fixture.articleType.ID, "DEL123")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	url := ts.URL + "/api/v1/devices/" + uuidString(device.Ucode)

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

type deviceFixture struct {
	client      sqlc.Client
	brand       sqlc.Brand
	articleType sqlc.ArticleType
}

func seedDeviceFixture(t *testing.T, q *sqlc.Queries) deviceFixture {
	t.Helper()
	return deviceFixture{
		client:      seedClient(t, q, "Cliente Equipo", ""),
		brand:       lookupBrandByName(t, q, "Samsung"),
		articleType: lookupArticleTypeByName(t, q, "celular"),
	}
}

func seedDevice(t *testing.T, q *sqlc.Queries, clientID, brandID int64, modelID pgtype.Int8, articleTypeID int64, serial string) sqlc.Device {
	t.Helper()
	device, err := q.CreateDevice(context.Background(), sqlc.CreateDeviceParams{
		ClientID:      clientID,
		BrandID:       brandID,
		ModelID:       modelID,
		ArticleTypeID: articleTypeID,
		SerialNumber:  pgtype.Text{String: serial, Valid: serial != ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	return device
}

func lookupBrandByName(t *testing.T, q *sqlc.Queries, name string) sqlc.Brand {
	t.Helper()
	brands, err := q.ListBrands(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, brand := range brands {
		if brand.Name == name {
			return brand
		}
	}
	t.Fatalf("missing brand %q", name)
	return sqlc.Brand{}
}

func lookupArticleTypeByName(t *testing.T, q *sqlc.Queries, name string) sqlc.ArticleType {
	t.Helper()
	types, err := q.ListArticleTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, articleType := range types {
		if articleType.Name == name {
			return articleType
		}
	}
	t.Fatalf("missing article type %q", name)
	return sqlc.ArticleType{}
}

type deviceSearchBody struct {
	Devices []deviceDTO `json:"devices"`
}

func decodeDeviceSearch(t *testing.T, res *http.Response) deviceSearchBody {
	t.Helper()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body deviceSearchBody
	decodeJSON(t, res.Body, &body)
	return body
}
