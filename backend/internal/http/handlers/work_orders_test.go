package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/workorder"
)

func TestIntake_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente WO", "WO-OK")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	if !regexp.MustCompile(`^\d{4}-\d{4}$`).MatchString(body.WorkOrder.WoNumber) {
		t.Fatalf("wo_number = %q, want YYYY-NNNN", body.WorkOrder.WoNumber)
	}
	if body.WorkOrder.Status != string(workorder.StatusReceived) {
		t.Fatalf("status = %q, want received", body.WorkOrder.Status)
	}
	wantEvents := []string{"start_diagnosis", "start_repair", "cancel"}
	if !sameStringSet(body.WorkOrder.AllowedEvents, wantEvents) {
		t.Fatalf("allowed_events = %v, want %v", body.WorkOrder.AllowedEvents, wantEvents)
	}
}

func TestIntake_UnknownClient(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente WO", "WO-UNKNOWN-CLIENT")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders", map[string]any{
		"client_ucode":   "00000000-0000-0000-0000-000000000000",
		"device_ucode":   uuidString(fixture.device.Ucode),
		"service_type":   "in_shop",
		"reported_issue": "No enciende",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "unknown_client")
}

func TestIntake_UnknownDevice(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente WO", "WO-UNKNOWN-DEVICE")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders", map[string]any{
		"client_ucode":   uuidString(fixture.client.Ucode),
		"device_ucode":   "00000000-0000-0000-0000-000000000000",
		"service_type":   "in_shop",
		"reported_issue": "No enciende",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "unknown_device")
}

func TestIntake_DeviceClientMismatch(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	a := seedClientAndDevice(t, q, "Cliente A", "WO-A")
	b := seedClientAndDevice(t, q, "Cliente B", "WO-B")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders", map[string]any{
		"client_ucode":   uuidString(a.client.Ucode),
		"device_ucode":   uuidString(b.device.Ucode),
		"service_type":   "in_shop",
		"reported_issue": "No enciende",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "device_client_mismatch")
}

func TestIntake_OnSiteServiceType(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente On Site", "WO-ONSITE")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	body := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "on_site")
	if body.WorkOrder.ServiceType != "on_site" {
		t.Fatalf("service_type = %q, want on_site", body.WorkOrder.ServiceType)
	}
}

func TestIntake_GeneratesSequentialNumbers(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Secuencia", "WO-SEQ")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	got := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		body := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
		got = append(got, body.WorkOrder.WoNumber)
	}
	year := time.Now().Year()
	want := []string{
		fmt.Sprintf("%d-0001", year),
		fmt.Sprintf("%d-0002", year),
		fmt.Sprintf("%d-0003", year),
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("wo_numbers = %v, want %v", got, want)
	}
}

func TestSearchWorkOrders_All(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Busqueda", "WO-SEARCH")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	for i := 0; i < 3; i++ {
		intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	}

	body := searchWorkOrders(t, client, ts.URL, "")
	if body.Total != 3 || len(body.WorkOrders) != 3 {
		t.Fatalf("total/len = %d/%d, want 3/3", body.Total, len(body.WorkOrders))
	}
}

func TestSearchWorkOrders_ByStatus(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Estado", "WO-STATUS")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	cancelled := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	row, err := q.GetWorkOrderByUcode(context.Background(), mustUUID(t, cancelled.WorkOrder.Ucode))
	if err != nil {
		t.Fatal(err)
	}
	_, err = q.UpdateWorkOrderStatus(context.Background(), sqlc.UpdateWorkOrderStatusParams{
		ID:     row.WorkOrder.ID,
		Status: string(workorder.StatusCancelled),
	})
	if err != nil {
		t.Fatal(err)
	}

	body := searchWorkOrders(t, client, ts.URL, "?status=cancelled")
	if body.Total != 1 || len(body.WorkOrders) != 1 || body.WorkOrders[0].Status != "cancelled" {
		t.Fatalf("search result = total %d rows %+v, want one cancelled", body.Total, body.WorkOrders)
	}
}

func TestSearchWorkOrders_ByClient(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	a := seedClientAndDevice(t, q, "Cliente A", "WO-CLIENT-A")
	b := seedClientAndDevice(t, q, "Cliente B", "WO-CLIENT-B")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	intakeWorkOrder(t, client, ts.URL, csrf, a.client, a.device, "in_shop")
	intakeWorkOrder(t, client, ts.URL, csrf, a.client, a.device, "in_shop")
	intakeWorkOrder(t, client, ts.URL, csrf, b.client, b.device, "in_shop")

	body := searchWorkOrders(t, client, ts.URL, "?client_ucode="+url.QueryEscape(uuidString(a.client.Ucode)))
	if body.Total != 2 || len(body.WorkOrders) != 2 {
		t.Fatalf("total/len = %d/%d, want 2/2", body.Total, len(body.WorkOrders))
	}
	for _, wo := range body.WorkOrders {
		if wo.Client.Ucode != uuidString(a.client.Ucode) {
			t.Fatalf("client_ucode = %q, want %q", wo.Client.Ucode, uuidString(a.client.Ucode))
		}
	}
}

func TestSearchWorkOrders_ByQuery_WoNumber(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Numero", "WO-NUMBER")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	for i := 0; i < 3; i++ {
		intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	}

	prefix := fmt.Sprintf("%d-", time.Now().Year())
	body := searchWorkOrders(t, client, ts.URL, "?q="+url.QueryEscape(prefix))
	if body.Total != 3 || len(body.WorkOrders) != 3 {
		t.Fatalf("total/len = %d/%d, want 3/3", body.Total, len(body.WorkOrders))
	}
}

func TestSearchWorkOrders_Pagination(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Pagina", "WO-PAGE")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	for i := 0; i < 5; i++ {
		intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	}

	body := searchWorkOrders(t, client, ts.URL, "?page_size=2&page=2")
	if body.Total != 5 || len(body.WorkOrders) != 2 || body.Page != 2 || body.PageSize != 2 {
		t.Fatalf("search result = total %d len %d page %d size %d, want 5/2/2/2", body.Total, len(body.WorkOrders), body.Page, body.PageSize)
	}
}

func TestGetWorkOrder_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Detalle", "WO-GET")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	res, err := client.Get(ts.URL + "/api/v1/work-orders/" + created.WorkOrder.Ucode)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body := decodeWorkOrderBody(t, res, http.StatusOK)
	if body.WorkOrder.Client.Name != fixture.client.Name {
		t.Fatalf("client.name = %q, want %q", body.WorkOrder.Client.Name, fixture.client.Name)
	}
	if body.WorkOrder.Device.BrandName != "Samsung" || body.WorkOrder.Device.ArticleTypeName != "celular" {
		t.Fatalf("device = %+v, want Samsung/celular", body.WorkOrder.Device)
	}
	if body.WorkOrder.Device.SerialNumber == nil || *body.WorkOrder.Device.SerialNumber != "WO-GET" {
		t.Fatalf("serial_number = %v, want WO-GET", body.WorkOrder.Device.SerialNumber)
	}
}

func TestGetWorkOrder_NotFound(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, user.Username)

	res, err := client.Get(ts.URL + "/api/v1/work-orders/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

type workOrderFixture struct {
	client sqlc.Client
	device sqlc.Device
}

func seedClientAndDevice(t *testing.T, q *sqlc.Queries, clientName, serial string) workOrderFixture {
	t.Helper()
	client := seedClient(t, q, clientName, "")
	brand := lookupBrandByName(t, q, "Samsung")
	articleType := lookupArticleTypeByName(t, q, "celular")
	device := seedDevice(t, q, client.ID, brand.ID, pgtype.Int8{}, articleType.ID, serial)
	return workOrderFixture{client: client, device: device}
}

type workOrderBody struct {
	WorkOrder workOrderDTO `json:"work_order"`
}

type workOrderSearchBody struct {
	WorkOrders []workOrderDTO `json:"work_orders"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}

func intakeWorkOrder(t *testing.T, client *http.Client, baseURL, csrf string, c sqlc.Client, d sqlc.Device, serviceType string) workOrderBody {
	t.Helper()
	res := postJSON(t, client, baseURL+"/api/v1/work-orders", map[string]any{
		"client_ucode":   uuidString(c.Ucode),
		"device_ucode":   uuidString(d.Ucode),
		"service_type":   serviceType,
		"reported_issue": "No enciende",
		"intake_notes":   "Equipo recibido en mostrador",
		"accessories":    "Cargador",
		"device_pin":     "1234",
	}, csrf)
	defer res.Body.Close()
	return decodeWorkOrderBody(t, res, http.StatusCreated)
}

func decodeWorkOrderBody(t *testing.T, res *http.Response, status int) workOrderBody {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}
	var body workOrderBody
	decodeJSON(t, res.Body, &body)
	return body
}

func searchWorkOrders(t *testing.T, client *http.Client, baseURL, query string) workOrderSearchBody {
	t.Helper()
	res, err := client.Get(baseURL + "/api/v1/work-orders" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body workOrderSearchBody
	decodeJSON(t, res.Body, &body)
	return body
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, item := range want {
		if !slices.Contains(got, item) {
			return false
		}
	}
	return true
}

func mustUUID(t *testing.T, raw string) pgtype.UUID {
	t.Helper()
	id, err := uuidFromString(raw)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
