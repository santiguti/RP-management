package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

func TestAudit_ClientCreate_RecordsRow(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := postJSON(t, client, ts.URL+"/api/v1/clients", map[string]any{
		"name":  "Cliente Audit",
		"phone": "+5491155551111",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}

	var afterName string
	var beforeIsNull bool
	if err := testPool.QueryRow(context.Background(), `
SELECT after_json->>'name', before_json IS NULL
FROM rp.audit_log
WHERE action = 'client.create' AND entity_type = 'client'
ORDER BY id DESC
LIMIT 1
`).Scan(&afterName, &beforeIsNull); err != nil {
		t.Fatal(err)
	}
	if afterName != "Cliente Audit" || !beforeIsNull {
		t.Fatalf("audit create = after %q before_null %v", afterName, beforeIsNull)
	}
}

func TestAudit_ClientUpdate_RecordsBeforeAndAfter(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	created := createAuditClient(t, client, ts.URL, csrf, "Cliente Antes")
	res := patchJSON(t, client, ts.URL+"/api/v1/clients/"+created.Ucode, map[string]any{
		"name": "Cliente Despues",
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	var beforeName, afterName string
	if err := testPool.QueryRow(context.Background(), `
SELECT before_json->>'name', after_json->>'name'
FROM rp.audit_log
WHERE action = 'client.update' AND entity_type = 'client'
ORDER BY id DESC
LIMIT 1
`).Scan(&beforeName, &afterName); err != nil {
		t.Fatal(err)
	}
	if beforeName == afterName || beforeName != "Cliente Antes" || afterName != "Cliente Despues" {
		t.Fatalf("audit update before/after = %q/%q", beforeName, afterName)
	}
}

func TestAudit_ClientDelete_RecordsBeforeOnly(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	created := createAuditClient(t, client, ts.URL, csrf, "Cliente Delete")
	res := deleteReq(t, client, ts.URL+"/api/v1/clients/"+created.Ucode, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d: %s", res.StatusCode, http.StatusNoContent, readBody(t, res))
	}

	var beforeName string
	var afterIsNull bool
	if err := testPool.QueryRow(context.Background(), `
SELECT before_json->>'name', after_json IS NULL
FROM rp.audit_log
WHERE action = 'client.delete' AND entity_type = 'client'
ORDER BY id DESC
LIMIT 1
`).Scan(&beforeName, &afterIsNull); err != nil {
		t.Fatal(err)
	}
	if beforeName != "Cliente Delete" || !afterIsNull {
		t.Fatalf("audit delete = before %q after_null %v", beforeName, afterIsNull)
	}
}

func TestAudit_WOTransition_RecordsStatusChange(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Audit WO", "AUDIT-WO")
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)
	created := intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop")

	transitionWorkOrder(t, client, ts.URL, csrf, created.WorkOrder.Ucode, "start_diagnosis", map[string]any{}, http.StatusOK)

	var beforeStatus, afterStatus, afterEvent string
	if err := testPool.QueryRow(context.Background(), `
SELECT before_json->>'status', after_json->>'status', after_json->>'event'
FROM rp.audit_log
WHERE action = 'wo.transition' AND entity_type = 'work_order'
ORDER BY id DESC
LIMIT 1
`).Scan(&beforeStatus, &afterStatus, &afterEvent); err != nil {
		t.Fatal(err)
	}
	if beforeStatus != "received" || afterStatus != "diagnosing" || afterEvent != "start_diagnosis" {
		t.Fatalf("transition audit = %s -> %s via %s", beforeStatus, afterStatus, afterEvent)
	}
}

func TestAudit_GetEndpoint_OwnerOnly(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	owner := seedOwner(t, q)
	employee := seedUserWithRole(t, q, "employee")
	seedAuditEntry(t, q, owner.ID, "client.create", "client")

	ts, employeeClient := newCookieServer(t, q)
	defer ts.Close()
	login(t, employeeClient, ts.URL, employee.Username)
	res, err := employeeClient.Get(ts.URL + "/api/v1/audit-log")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")

	ownerTs, realOwnerClient := newCookieServer(t, q)
	defer ownerTs.Close()
	login(t, realOwnerClient, ownerTs.URL, owner.Username)
	okRes, err := realOwnerClient.Get(ownerTs.URL + "/api/v1/audit-log")
	if err != nil {
		t.Fatal(err)
	}
	defer okRes.Body.Close()
	if okRes.StatusCode != http.StatusOK {
		t.Fatalf("owner status = %d, want %d: %s", okRes.StatusCode, http.StatusOK, readBody(t, okRes))
	}
}

func TestAudit_GetEndpoint_FiltersByEntityType(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	owner := seedOwner(t, q)
	seedAuditEntry(t, q, owner.ID, "client.create", "client")
	seedAuditEntry(t, q, owner.ID, "supplier.create", "supplier")

	body := getAuditEntries(t, q, owner.Username, "?entity_type=client")
	if body.Total != 1 || len(body.Entries) != 1 || body.Entries[0].EntityType != "client" {
		t.Fatalf("body = %+v, want one client entry", body)
	}
}

func TestAudit_GetEndpoint_FiltersByActor(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	owner := seedOwner(t, q)
	other := seedUserWithRole(t, q, "owner")
	seedAuditEntry(t, q, owner.ID, "client.create", "client")
	seedAuditEntry(t, q, other.ID, "supplier.create", "supplier")

	body := getAuditEntries(t, q, owner.Username, "?actor="+owner.Username)
	if body.Total != 1 || len(body.Entries) != 1 || body.Entries[0].ActorUsername == nil || *body.Entries[0].ActorUsername != owner.Username {
		t.Fatalf("body = %+v, want one entry for %s", body, owner.Username)
	}
}

func TestAudit_GetEndpoint_Pagination(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	owner := seedOwner(t, q)
	seedAuditEntry(t, q, owner.ID, "client.create", "client")
	seedAuditEntry(t, q, owner.ID, "client.update", "client")
	seedAuditEntry(t, q, owner.ID, "client.delete", "client")

	body := getAuditEntries(t, q, owner.Username, "?page=2&page_size=1")
	if body.Total != 3 || body.Page != 2 || body.PageSize != 1 || len(body.Entries) != 1 {
		t.Fatalf("pagination body = %+v, want total 3 page 2 size 1 len 1", body)
	}
}

type auditClientBody struct {
	Client clientDTO `json:"client"`
}

type auditListBody struct {
	Entries  []auditEntryDTO `json:"entries"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

func createAuditClient(t *testing.T, client *http.Client, baseURL, csrf, name string) clientDTO {
	t.Helper()
	res := postJSON(t, client, baseURL+"/api/v1/clients", map[string]any{
		"name": name,
	}, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create client status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	var body auditClientBody
	decodeJSON(t, res.Body, &body)
	return body.Client
}

func getAuditEntries(t *testing.T, q *sqlc.Queries, username, query string) auditListBody {
	t.Helper()
	ts, client := newCookieServer(t, q)
	defer ts.Close()
	login(t, client, ts.URL, username)
	res, err := client.Get(ts.URL + "/api/v1/audit-log" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("audit status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body auditListBody
	decodeJSON(t, res.Body, &body)
	return body
}

func seedAuditEntry(t *testing.T, q *sqlc.Queries, actorID int64, action, entityType string) {
	t.Helper()
	if _, err := q.CreateAuditEntry(context.Background(), sqlc.CreateAuditEntryParams{
		ActorUserID: pgtype.Int8{Int64: actorID, Valid: true},
		Action:      action,
		EntityType:  entityType,
		AfterJson:   []byte(`{"ok":true}`),
	}); err != nil {
		t.Fatal(err)
	}
}
