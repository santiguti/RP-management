package handlers

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/storage"
)

func TestAttachment_UploadPNG_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "PNG")
	defer ts.Close()
	pngBytes := attachmentPNG(t)

	res := uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "intake", "equipo.png", pngBytes)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	body := decodeAttachmentBody(t, res)
	if body.Attachment.MimeType != "image/png" || body.Attachment.SizeBytes != int64(len(pngBytes)) {
		t.Fatalf("attachment = %+v, want png with size %d", body.Attachment, len(pngBytes))
	}
	if body.Attachment.Width == nil || *body.Attachment.Width != 2 || body.Attachment.Height == nil || *body.Attachment.Height != 2 {
		t.Fatalf("dimensions = %v/%v, want 2/2", body.Attachment.Width, body.Attachment.Height)
	}
	attachment, err := q.GetAttachmentByUcode(context.Background(), mustUUID(t, body.Attachment.Ucode))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testAttachmentStore.Open(attachment.StoragePath); err != nil {
		t.Fatalf("stored file missing at %q: %v", attachment.StoragePath, err)
	}
}

func TestAttachment_UploadJPEG_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "JPEG")
	defer ts.Close()

	res := uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "diagnosis", "equipo.jpg", attachmentJPEG(t))
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	body := decodeAttachmentBody(t, res)
	if body.Attachment.MimeType != "image/jpeg" {
		t.Fatalf("mime_type = %q, want image/jpeg", body.Attachment.MimeType)
	}
}

func TestAttachment_RejectsPDF_415(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "PDF")
	defer ts.Close()

	res := uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "intake", "archivo.pdf", []byte("%PDF-1.4"))
	defer res.Body.Close()
	assertError(t, res, http.StatusUnsupportedMediaType, "unsupported_mime")
}

func TestAttachment_RejectsTooLarge_413(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "LARGE")
	defer ts.Close()
	content := append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0}, int(storage.MaxUploadBytes)+1)...)

	res := uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "intake", "grande.jpg", content)
	defer res.Body.Close()
	assertError(t, res, http.StatusRequestEntityTooLarge, "file_too_large")
}

func TestAttachment_RejectsInvalidPhase_400(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "PHASE")
	defer ts.Close()

	res := uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "otro", "equipo.png", attachmentPNG(t))
	defer res.Body.Close()
	assertError(t, res, http.StatusBadRequest, "invalid_phase")
}

func TestAttachment_UploadRemovesFileWhenDBInsertFails(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	fixture := seedClientAndDevice(t, q, "Cliente Orphan", "WO-ATT-ORPHAN")
	workOrder := seedWorkOrder(t, q, fixture.client.ID, fixture.device.ID)
	filesBefore := countStoredAttachmentFiles(t)

	failingQueries := sqlc.New(failCreateAttachmentDB{DBTX: testPool})
	ts, client := newCookieServer(t, failingQueries)
	defer ts.Close()
	csrf := login(t, client, ts.URL, user.Username)

	res := uploadAttachment(t, client, ts.URL, csrf, uuidString(workOrder.Ucode), "intake", "equipo.png", attachmentPNG(t))
	defer res.Body.Close()
	assertError(t, res, http.StatusInternalServerError, "internal")

	if filesAfter := countStoredAttachmentFiles(t); filesAfter != filesBefore {
		t.Fatalf("stored files = %d, want %d", filesAfter, filesBefore)
	}
}

func TestAttachment_List_OrdersByCreatedAsc(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "LIST")
	defer ts.Close()
	first := uploadAttachmentBody(t, uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "intake", "primero.png", attachmentPNG(t)))
	second := uploadAttachmentBody(t, uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "delivery", "segundo.jpg", attachmentJPEG(t)))

	res, err := client.Get(ts.URL + "/api/v1/work-orders/" + workOrder.WorkOrder.Ucode + "/attachments")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	var body struct {
		Attachments []attachmentDTO `json:"attachments"`
	}
	decodeJSON(t, res.Body, &body)
	if len(body.Attachments) != 2 || body.Attachments[0].Ucode != first.Attachment.Ucode || body.Attachments[1].Ucode != second.Attachment.Ucode {
		t.Fatalf("attachments = %+v, want created order", body.Attachments)
	}
}

func TestAttachment_Download_OK(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "DOWNLOAD")
	defer ts.Close()
	pngBytes := attachmentPNG(t)
	uploaded := uploadAttachmentBody(t, uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "intake", "equipo.png", pngBytes))

	res, err := client.Get(ts.URL + "/api/v1/work-orders/" + workOrder.WorkOrder.Ucode + "/attachments/" + uploaded.Attachment.Ucode)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}
	if got := res.Header.Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", got)
	}
	if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "private, max-age=3600" {
		t.Fatalf("Cache-Control = %q, want private, max-age=3600", got)
	}
	if res.ContentLength != int64(len(pngBytes)) {
		t.Fatalf("ContentLength = %d, want %d", res.ContentLength, len(pngBytes))
	}
	got, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Fatal("downloaded bytes differ from uploaded bytes")
	}
}

func TestAttachment_DownloadNotFound_404(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, _ := attachmentWorkOrder(t, q, user.Username, "NOTFOUND")
	defer ts.Close()

	res, err := client.Get(ts.URL + "/api/v1/work-orders/" + workOrder.WorkOrder.Ucode + "/attachments/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

func TestAttachment_DownloadCrossWO_404(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	a, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "CROSS-A")
	defer ts.Close()
	bFixture := seedClientAndDevice(t, q, "Cliente Cross B", "WO-ATT-CROSS-B")
	b := intakeWorkOrder(t, client, ts.URL, csrf, bFixture.client, bFixture.device, "in_shop")
	uploaded := uploadAttachmentBody(t, uploadAttachment(t, client, ts.URL, csrf, a.WorkOrder.Ucode, "intake", "equipo.png", attachmentPNG(t)))

	res, err := client.Get(ts.URL + "/api/v1/work-orders/" + b.WorkOrder.Ucode + "/attachments/" + uploaded.Attachment.Ucode)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	assertError(t, res, http.StatusNotFound, "not_found")
}

func TestAttachment_DeleteSoftDeletes(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	user := seedOwner(t, q)
	workOrder, ts, client, csrf := attachmentWorkOrder(t, q, user.Username, "DELETE")
	defer ts.Close()
	uploaded := uploadAttachmentBody(t, uploadAttachment(t, client, ts.URL, csrf, workOrder.WorkOrder.Ucode, "intake", "equipo.png", attachmentPNG(t)))

	res := deleteReq(t, client, ts.URL+"/api/v1/work-orders/"+workOrder.WorkOrder.Ucode+"/attachments/"+uploaded.Attachment.Ucode, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusNoContent, readBody(t, res))
	}
	list, err := client.Get(ts.URL + "/api/v1/work-orders/" + workOrder.WorkOrder.Ucode + "/attachments")
	if err != nil {
		t.Fatal(err)
	}
	defer list.Body.Close()
	var body struct {
		Attachments []attachmentDTO `json:"attachments"`
	}
	decodeJSON(t, list.Body, &body)
	if len(body.Attachments) != 0 {
		t.Fatalf("attachments = %+v, want empty after delete", body.Attachments)
	}
}

func TestAttachments_DeleteRequiresOwner(t *testing.T) {
	q, cleanup := newTxQueries(t)
	defer cleanup()
	owner := seedOwner(t, q)
	employee := seedUserWithRole(t, q, "employee")
	workOrder, ts, client, ownerCSRF := attachmentWorkOrder(t, q, owner.Username, "DELETE-OWNER")
	defer ts.Close()
	uploaded := uploadAttachmentBody(t, uploadAttachment(t, client, ts.URL, ownerCSRF, workOrder.WorkOrder.Ucode, "intake", "equipo.png", attachmentPNG(t)))

	employeeCSRF := login(t, client, ts.URL, employee.Username)
	res := deleteReq(t, client, ts.URL+"/api/v1/work-orders/"+workOrder.WorkOrder.Ucode+"/attachments/"+uploaded.Attachment.Ucode, employeeCSRF)
	defer res.Body.Close()
	assertError(t, res, http.StatusForbidden, "forbidden")
}

type attachmentBody struct {
	Attachment attachmentDTO `json:"attachment"`
}

func attachmentWorkOrder(t *testing.T, q *sqlc.Queries, username, suffix string) (workOrderBody, *httptest.Server, *http.Client, string) {
	t.Helper()
	fixture := seedClientAndDevice(t, q, "Cliente Adjuntos "+suffix, "WO-ATT-"+suffix)
	ts, client := newCookieServer(t, q)
	csrf := login(t, client, ts.URL, username)
	return intakeWorkOrder(t, client, ts.URL, csrf, fixture.client, fixture.device, "in_shop"), ts, client, csrf
}

func uploadAttachment(t *testing.T, client *http.Client, baseURL, csrf, workOrderUcode, phase, filename string, content []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("phase", phase); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/work-orders/"+workOrderUcode+"/attachments", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func uploadAttachmentBody(t *testing.T, res *http.Response) attachmentBody {
	t.Helper()
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusCreated, readBody(t, res))
	}
	return decodeAttachmentBody(t, res)
}

func decodeAttachmentBody(t *testing.T, res *http.Response) attachmentBody {
	t.Helper()
	var body attachmentBody
	decodeJSON(t, res.Body, &body)
	return body
}

func attachmentPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func attachmentJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{G: 255, A: 255})
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func countStoredAttachmentFiles(t *testing.T) int {
	t.Helper()
	var count int
	err := filepath.WalkDir(testAttachmentStore.Root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

type failCreateAttachmentDB struct {
	sqlc.DBTX
}

func (db failCreateAttachmentDB) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	if strings.Contains(query, "INSERT INTO rp.attachments") {
		return errRow{err: errors.New("forced attachment insert failure")}
	}
	return db.DBTX.QueryRow(ctx, query, args...)
}

func (db failCreateAttachmentDB) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return db.DBTX.Query(ctx, query, args...)
}

func (db failCreateAttachmentDB) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return db.DBTX.Exec(ctx, query, args...)
}

type errRow struct {
	err error
}

func (r errRow) Scan(...interface{}) error {
	return r.err
}
