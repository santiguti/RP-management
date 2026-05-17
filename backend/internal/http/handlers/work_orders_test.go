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

func TestUpdate_ChangesDiagnosis(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Update", "WO-UPDATE")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	res := patchJSON(t, client, ts.URL+"/api/v1/work-orders/"+created.WorkOrder.Ucode, map[string]any{
		"diagnosis": "placa madre quemada",
	}, csrf)
	defer res.Body.Close()
	body := decodeWorkOrderBody(t, res, http.StatusOK)
	if body.WorkOrder.Diagnosis == nil || *body.WorkOrder.Diagnosis != "placa madre quemada" {
		t.Fatalf("diagnosis = %v, want placa madre quemada", body.WorkOrder.Diagnosis)
	}
}

func TestUpdate_RejectsInvalidServiceType(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Update Invalid", "WO-UPDATE-INVALID")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	res := patchJSON(t, client, ts.URL+"/api/v1/work-orders/"+created.WorkOrder.Ucode, map[string]any{
		"service_type": "weird",
	}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestTransition_StartDiagnosis(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Diagnostico", "WO-DIAG")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	body := transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "start_diagnosis", map[string]any{}, http.StatusOK)
	if body.WorkOrder.Status != "diagnosing" {
		t.Fatalf("status = %q, want diagnosing", body.WorkOrder.Status)
	}
}

func TestTransition_Quote_RequiresAmount(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Quote Req", "WO-QUOTE-REQ")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "start_diagnosis", map[string]any{}, http.StatusOK)

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders/"+created.WorkOrder.Ucode+"/transitions/quote", map[string]any{}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_body")
}

func TestTransition_Quote_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Quote OK", "WO-QUOTE-OK")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")
	transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "start_diagnosis", map[string]any{}, http.StatusOK)

	body := transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "quote", map[string]any{
		"quote_amount": "15000.00",
		"diagnosis":    "fuente quemada",
	}, http.StatusOK)
	if body.WorkOrder.Status != "quoted" {
		t.Fatalf("status = %q, want quoted", body.WorkOrder.Status)
	}
	if body.WorkOrder.QuoteSentTs == nil {
		t.Fatal("quote_sent_ts is nil")
	}
	if body.WorkOrder.QuoteAmount == nil || *body.WorkOrder.QuoteAmount != "15000.00" {
		t.Fatalf("quote_amount = %v, want 15000.00", body.WorkOrder.QuoteAmount)
	}
}

func TestTransition_Approve(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Approve", "WO-APPROVE")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	quoted := quotedWorkOrder(t, client, ts.URL, csrf, fixture)

	body := transitionWorkOrder(t, client, ts.URL, csrf, quoted.WorkOrder.Ucode, "approve", map[string]any{}, http.StatusOK)
	if body.WorkOrder.Status != "approved" {
		t.Fatalf("status = %q, want approved", body.WorkOrder.Status)
	}
	if body.WorkOrder.QuoteApprovedTs == nil {
		t.Fatal("quote_approved_ts is nil")
	}
}

func TestTransition_Reject(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Reject", "WO-REJECT")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	quoted := quotedWorkOrder(t, client, ts.URL, csrf, fixture)

	body := transitionWorkOrder(t, client, ts.URL, csrf, quoted.WorkOrder.Ucode, "reject", map[string]any{}, http.StatusOK)
	if body.WorkOrder.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", body.WorkOrder.Status)
	}
	if body.WorkOrder.QuoteRejectedTs == nil {
		t.Fatal("quote_rejected_ts is nil")
	}
}

func TestTransition_StartRepair_FromApproved(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Repair", "WO-REPAIR")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	quoted := quotedWorkOrder(t, client, ts.URL, csrf, fixture)
	approved := transitionWorkOrder(t, client, ts.URL, csrf, quoted.WorkOrder.Ucode, "approve", map[string]any{}, http.StatusOK)

	body := transitionWorkOrder(t, client, ts.URL, csrf, approved.WorkOrder.Ucode, "start_repair", map[string]any{}, http.StatusOK)
	if body.WorkOrder.Status != "in_repair" {
		t.Fatalf("status = %q, want in_repair", body.WorkOrder.Status)
	}
	if body.WorkOrder.StartedTs == nil {
		t.Fatal("started_ts is nil")
	}
}

func TestTransition_MarkReady_WithFinals(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Ready", "WO-READY")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	quoted := quotedWorkOrder(t, client, ts.URL, csrf, fixture)
	approved := transitionWorkOrder(t, client, ts.URL, csrf, quoted.WorkOrder.Ucode, "approve", map[string]any{}, http.StatusOK)
	repair := transitionWorkOrder(t, client, ts.URL, csrf, approved.WorkOrder.Ucode, "start_repair", map[string]any{}, http.StatusOK)

	body := transitionWorkOrder(t, client, ts.URL, csrf, repair.WorkOrder.Ucode, "mark_ready", map[string]any{
		"labor_amount": "10000.00",
		"parts_amount": "5000.00",
		"final_amount": "15000.00",
	}, http.StatusOK)
	if body.WorkOrder.Status != "ready" {
		t.Fatalf("status = %q, want ready", body.WorkOrder.Status)
	}
	if body.WorkOrder.ReadyTs == nil {
		t.Fatal("ready_ts is nil")
	}
	assertMoneyPtr(t, body.WorkOrder.LaborAmount, "10000.00")
	assertMoneyPtr(t, body.WorkOrder.PartsAmount, "5000.00")
	assertMoneyPtr(t, body.WorkOrder.FinalAmount, "15000.00")
}

func TestTransition_Deliver(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Deliver", "WO-DELIVER")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	ready := readyWorkOrder(t, client, ts.URL, csrf, fixture)

	body := transitionWorkOrder(t, client, ts.URL, csrf, ready.WorkOrder.Ucode, "deliver", map[string]any{}, http.StatusOK)
	if body.WorkOrder.Status != "delivered" {
		t.Fatalf("status = %q, want delivered", body.WorkOrder.Status)
	}
	if body.WorkOrder.DeliveredTs == nil {
		t.Fatal("delivered_ts is nil")
	}
}

func TestTransition_Cancel_WithReason(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Cancel", "WO-CANCEL")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	body := transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "cancel", map[string]any{
		"cancel_reason": "cliente arrepintio",
	}, http.StatusOK)
	if body.WorkOrder.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", body.WorkOrder.Status)
	}
	if body.WorkOrder.CancelledTs == nil {
		t.Fatal("cancelled_ts is nil")
	}
	if body.WorkOrder.CancelReason == nil || *body.WorkOrder.CancelReason != "cliente arrepintio" {
		t.Fatalf("cancel_reason = %v, want cliente arrepintio", body.WorkOrder.CancelReason)
	}
}

func TestTransition_FromDelivered_AnyEvent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Delivered Invalid", "WO-DELIVERED-INVALID")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	ready := readyWorkOrder(t, client, ts.URL, csrf, fixture)
	delivered := transitionWorkOrder(t, client, ts.URL, csrf, ready.WorkOrder.Ucode, "deliver", map[string]any{}, http.StatusOK)

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders/"+delivered.WorkOrder.Ucode+"/transitions/start_repair", map[string]any{}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusConflict, readBody(t, res))
	}
	var body struct {
		Error         string   `json:"error"`
		From          string   `json:"from"`
		AllowedEvents []string `json:"allowed_events"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Error != "invalid_transition" || body.From != "delivered" || len(body.AllowedEvents) != 0 {
		t.Fatalf("body = %+v, want invalid_transition from delivered with no allowed events", body)
	}
}

func TestTransition_UnknownEvent(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Unknown Event", "WO-UNKNOWN-EVENT")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders/"+created.WorkOrder.Ucode+"/transitions/nonsense", map[string]any{}, csrf)
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "unknown_event")
}

func TestTransition_FromReceived_Approve(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Invalid Edge", "WO-INVALID-EDGE")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	res := postJSON(t, client, ts.URL+"/api/v1/work-orders/"+created.WorkOrder.Ucode+"/transitions/approve", map[string]any{}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusConflict, readBody(t, res))
	}
	var body struct {
		Error string `json:"error"`
		From  string `json:"from"`
		Event string `json:"event"`
	}
	decodeJSON(t, res.Body, &body)
	if body.Error != "invalid_transition" || body.From != "received" || body.Event != "approve" {
		t.Fatalf("body = %+v, want invalid approve from received", body)
	}
}

func TestTransition_OnSite_FastTrack(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente On Site Flow", "WO-ONSITE-FLOW")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "on_site")

	repair := transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "start_repair", map[string]any{}, http.StatusOK)
	ready := transitionWorkOrder(t, client, ts.URL, csrf, repair.WorkOrder.Ucode, "mark_ready", map[string]any{}, http.StatusOK)
	delivered := transitionWorkOrder(t, client, ts.URL, csrf, ready.WorkOrder.Ucode, "deliver", map[string]any{}, http.StatusOK)
	if repair.WorkOrder.Status != "in_repair" || ready.WorkOrder.Status != "ready" || delivered.WorkOrder.Status != "delivered" {
		t.Fatalf("statuses = %q/%q/%q, want in_repair/ready/delivered", repair.WorkOrder.Status, ready.WorkOrder.Status, delivered.WorkOrder.Status)
	}
	if delivered.WorkOrder.QuoteSentTs != nil {
		t.Fatalf("quote_sent_ts = %v, want nil", delivered.WorkOrder.QuoteSentTs)
	}
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

func transitionWorkOrder(t *testing.T, client *http.Client, baseURL, csrf, ucode, event string, payload any, status int) workOrderBody {
	t.Helper()
	res := postJSON(t, client, baseURL+"/api/v1/work-orders/"+ucode+"/transitions/"+event, payload, csrf)
	defer res.Body.Close()
	return decodeWorkOrderBody(t, res, status)
}

func quotedWorkOrder(t *testing.T, client *http.Client, baseURL, csrf string, fixture workOrderFixture) workOrderBody {
	t.Helper()
	created := intakeWorkOrder(t, client, baseURL, csrf, fixture.client, fixture.device, "in_shop")
	transitionWorkOrder(t, client, baseURL, csrf, created.WorkOrder.Ucode, "start_diagnosis", map[string]any{}, http.StatusOK)
	return transitionWorkOrder(t, client, baseURL, csrf, created.WorkOrder.Ucode, "quote", map[string]any{
		"quote_amount": "15000.00",
		"diagnosis":    "fuente quemada",
	}, http.StatusOK)
}

func readyWorkOrder(t *testing.T, client *http.Client, baseURL, csrf string, fixture workOrderFixture) workOrderBody {
	t.Helper()
	quoted := quotedWorkOrder(t, client, baseURL, csrf, fixture)
	approved := transitionWorkOrder(t, client, baseURL, csrf, quoted.WorkOrder.Ucode, "approve", map[string]any{}, http.StatusOK)
	repair := transitionWorkOrder(t, client, baseURL, csrf, approved.WorkOrder.Ucode, "start_repair", map[string]any{}, http.StatusOK)
	return transitionWorkOrder(t, client, baseURL, csrf, repair.WorkOrder.Ucode, "mark_ready", map[string]any{}, http.StatusOK)
}

func assertMoneyPtr(t *testing.T, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("money = %v, want %s", got, want)
	}
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
