package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/workorder"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type workOrderDTO struct {
	Ucode           string      `json:"ucode"`
	WoNumber        string      `json:"wo_number"`
	Status          string      `json:"status"`
	ServiceType     string      `json:"service_type"`
	Client          wOClientDTO `json:"client"`
	Device          wODeviceDTO `json:"device"`
	ReportedIssue   string      `json:"reported_issue"`
	Diagnosis       *string     `json:"diagnosis,omitempty"`
	QuoteAmount     *string     `json:"quote_amount,omitempty"`
	QuoteCurrency   string      `json:"quote_currency"`
	LaborAmount     *string     `json:"labor_amount,omitempty"`
	PartsAmount     *string     `json:"parts_amount,omitempty"`
	FinalAmount     *string     `json:"final_amount,omitempty"`
	IntakeNotes     *string     `json:"intake_notes,omitempty"`
	Accessories     *string     `json:"accessories,omitempty"`
	DevicePin       *string     `json:"device_pin,omitempty"`
	CancelReason    *string     `json:"cancel_reason,omitempty"`
	ReceivedTs      string      `json:"received_ts"`
	StartedTs       *string     `json:"started_ts,omitempty"`
	QuoteSentTs     *string     `json:"quote_sent_ts,omitempty"`
	QuoteApprovedTs *string     `json:"quote_approved_ts,omitempty"`
	QuoteRejectedTs *string     `json:"quote_rejected_ts,omitempty"`
	ReadyTs         *string     `json:"ready_ts,omitempty"`
	DeliveredTs     *string     `json:"delivered_ts,omitempty"`
	CancelledTs     *string     `json:"cancelled_ts,omitempty"`
	AllowedEvents   []string    `json:"allowed_events"`
}

type wOClientDTO struct {
	Ucode string  `json:"ucode"`
	Name  string  `json:"name"`
	Phone *string `json:"phone,omitempty"`
}

type wODeviceDTO struct {
	Ucode           string  `json:"ucode"`
	BrandName       string  `json:"brand_name"`
	ModelName       *string `json:"model_name,omitempty"`
	ArticleTypeName string  `json:"article_type_name"`
	SerialNumber    *string `json:"serial_number,omitempty"`
}

type intakeReq struct {
	ClientUcode   string  `json:"client_ucode" validate:"required"`
	DeviceUcode   string  `json:"device_ucode" validate:"required"`
	ServiceType   string  `json:"service_type" validate:"required,oneof=in_shop on_site"`
	ReportedIssue string  `json:"reported_issue" validate:"required,min=1,max=2000"`
	IntakeNotes   *string `json:"intake_notes" validate:"omitempty,max=4000"`
	Accessories   *string `json:"accessories" validate:"omitempty,max=2000"`
	DevicePin     *string `json:"device_pin" validate:"omitempty,max=64"`
}

type WorkOrders struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewWorkOrders(q *sqlc.Queries) *WorkOrders {
	return &WorkOrders{queries: q, val: validator.New()}
}

func (w *WorkOrders) Intake(rw http.ResponseWriter, r *http.Request) {
	var req intakeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimIntakeReq(&req)
	if err := w.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	client, ok := w.resolveClient(rw, r, req.ClientUcode)
	if !ok {
		return
	}
	device, ok := w.resolveDevice(rw, r, req.DeviceUcode)
	if !ok {
		return
	}
	if device.Device.ClientID != client.ID {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "device_client_mismatch"})
		return
	}

	user, _ := middleware.UserFromContext(r.Context())
	out, err := w.queries.CreateWorkOrder(r.Context(), sqlc.CreateWorkOrderParams{
		ClientID:           client.ID,
		DeviceID:           device.Device.ID,
		ServiceType:        req.ServiceType,
		ReportedIssue:      req.ReportedIssue,
		IntakeNotes:        textFromPtr(req.IntakeNotes),
		Accessories:        textFromPtr(req.Accessories),
		DevicePinEncrypted: textFromPtr(req.DevicePin),
		CreatedByUserID:    pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("create work order: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	detail, ok := w.workOrderByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	writeJSON(rw, http.StatusCreated, map[string]workOrderDTO{"work_order": toWorkOrderDTO(detail)})
}

func (w *WorkOrders) Search(rw http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	clientRaw := strings.TrimSpace(r.URL.Query().Get("client_ucode"))
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}
	if status != "" && !isKnownWorkOrderStatus(status) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_status"})
		return
	}

	params := sqlc.ListWorkOrdersParams{
		Status:     status,
		Q:          q,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	}
	countParams := sqlc.CountWorkOrdersParams{
		Status: status,
		Q:      q,
	}
	if clientRaw != "" {
		client, ok := w.resolveClient(rw, r, clientRaw)
		if !ok {
			return
		}
		params.HasClient = true
		params.ClientID = client.ID
		countParams.HasClient = true
		countParams.ClientID = client.ID
	}

	total, err := w.queries.CountWorkOrders(r.Context(), countParams)
	if err != nil {
		log.Printf("count work orders: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	rows, err := w.queries.ListWorkOrders(r.Context(), params)
	if err != nil {
		log.Printf("list work orders: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]workOrderDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWorkOrderDTOFromList(row))
	}
	writeJSON(rw, http.StatusOK, map[string]any{
		"work_orders": out,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
	})
}

func (w *WorkOrders) Get(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	row, ok := w.workOrderByUcode(rw, r, ucode)
	if !ok {
		return
	}
	writeJSON(rw, http.StatusOK, map[string]workOrderDTO{"work_order": toWorkOrderDTO(row)})
}

func (w *WorkOrders) resolveClient(rw http.ResponseWriter, r *http.Request, raw string) (sqlc.Client, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
		return sqlc.Client{}, false
	}
	client, err := w.queries.GetClientByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
		return sqlc.Client{}, false
	}
	if err != nil {
		log.Printf("resolve work order client: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Client{}, false
	}
	return client, true
}

func (w *WorkOrders) resolveDevice(rw http.ResponseWriter, r *http.Request, raw string) (sqlc.GetDeviceByUcodeRow, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_device"})
		return sqlc.GetDeviceByUcodeRow{}, false
	}
	device, err := w.queries.GetDeviceByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_device"})
		return sqlc.GetDeviceByUcodeRow{}, false
	}
	if err != nil {
		log.Printf("resolve work order device: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.GetDeviceByUcodeRow{}, false
	}
	return device, true
}

func (w *WorkOrders) workOrderByUcode(rw http.ResponseWriter, r *http.Request, ucode pgtype.UUID) (sqlc.GetWorkOrderByUcodeRow, bool) {
	row, err := w.queries.GetWorkOrderByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	if err != nil {
		log.Printf("get work order: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	return row, true
}

func toWorkOrderDTO(row sqlc.GetWorkOrderByUcodeRow) workOrderDTO {
	return workOrderDTO{
		Ucode:           uuidString(row.WorkOrder.Ucode),
		WoNumber:        row.WorkOrder.WoNumber,
		Status:          row.WorkOrder.Status,
		ServiceType:     row.WorkOrder.ServiceType,
		Client:          wOClientDTO{Ucode: uuidString(row.ClientUcode), Name: row.ClientName, Phone: stringPtrFromText(row.ClientPhone)},
		Device:          wODeviceDTO{Ucode: uuidString(row.DeviceUcode), BrandName: row.BrandName, ModelName: stringPtrFromText(row.ModelName), ArticleTypeName: row.ArticleTypeName, SerialNumber: stringPtrFromText(row.DeviceSerial)},
		ReportedIssue:   row.WorkOrder.ReportedIssue,
		Diagnosis:       stringPtrFromText(row.WorkOrder.Diagnosis),
		QuoteAmount:     numericToStringPtr(row.WorkOrder.QuoteAmount),
		QuoteCurrency:   row.WorkOrder.QuoteCurrency,
		LaborAmount:     numericToStringPtr(row.WorkOrder.LaborAmount),
		PartsAmount:     numericToStringPtr(row.WorkOrder.PartsAmount),
		FinalAmount:     numericToStringPtr(row.WorkOrder.FinalAmount),
		IntakeNotes:     stringPtrFromText(row.WorkOrder.IntakeNotes),
		Accessories:     stringPtrFromText(row.WorkOrder.Accessories),
		DevicePin:       stringPtrFromText(row.WorkOrder.DevicePinEncrypted),
		CancelReason:    stringPtrFromText(row.WorkOrder.CancelReason),
		ReceivedTs:      timeString(row.WorkOrder.ReceivedTs),
		StartedTs:       timeStringPtr(row.WorkOrder.StartedTs),
		QuoteSentTs:     timeStringPtr(row.WorkOrder.QuoteSentTs),
		QuoteApprovedTs: timeStringPtr(row.WorkOrder.QuoteApprovedTs),
		QuoteRejectedTs: timeStringPtr(row.WorkOrder.QuoteRejectedTs),
		ReadyTs:         timeStringPtr(row.WorkOrder.ReadyTs),
		DeliveredTs:     timeStringPtr(row.WorkOrder.DeliveredTs),
		CancelledTs:     timeStringPtr(row.WorkOrder.CancelledTs),
		AllowedEvents:   allowedEventStrings(workorder.Status(row.WorkOrder.Status)),
	}
}

func toWorkOrderDTOFromList(row sqlc.ListWorkOrdersRow) workOrderDTO {
	return workOrderDTO{
		Ucode:           uuidString(row.WorkOrder.Ucode),
		WoNumber:        row.WorkOrder.WoNumber,
		Status:          row.WorkOrder.Status,
		ServiceType:     row.WorkOrder.ServiceType,
		Client:          wOClientDTO{Ucode: uuidString(row.ClientUcode), Name: row.ClientName, Phone: stringPtrFromText(row.ClientPhone)},
		Device:          wODeviceDTO{Ucode: uuidString(row.DeviceUcode), BrandName: row.BrandName, ModelName: stringPtrFromText(row.ModelName), ArticleTypeName: row.ArticleTypeName},
		ReportedIssue:   row.WorkOrder.ReportedIssue,
		Diagnosis:       stringPtrFromText(row.WorkOrder.Diagnosis),
		QuoteAmount:     numericToStringPtr(row.WorkOrder.QuoteAmount),
		QuoteCurrency:   row.WorkOrder.QuoteCurrency,
		LaborAmount:     numericToStringPtr(row.WorkOrder.LaborAmount),
		PartsAmount:     numericToStringPtr(row.WorkOrder.PartsAmount),
		FinalAmount:     numericToStringPtr(row.WorkOrder.FinalAmount),
		IntakeNotes:     stringPtrFromText(row.WorkOrder.IntakeNotes),
		Accessories:     stringPtrFromText(row.WorkOrder.Accessories),
		DevicePin:       stringPtrFromText(row.WorkOrder.DevicePinEncrypted),
		CancelReason:    stringPtrFromText(row.WorkOrder.CancelReason),
		ReceivedTs:      timeString(row.WorkOrder.ReceivedTs),
		StartedTs:       timeStringPtr(row.WorkOrder.StartedTs),
		QuoteSentTs:     timeStringPtr(row.WorkOrder.QuoteSentTs),
		QuoteApprovedTs: timeStringPtr(row.WorkOrder.QuoteApprovedTs),
		QuoteRejectedTs: timeStringPtr(row.WorkOrder.QuoteRejectedTs),
		ReadyTs:         timeStringPtr(row.WorkOrder.ReadyTs),
		DeliveredTs:     timeStringPtr(row.WorkOrder.DeliveredTs),
		CancelledTs:     timeStringPtr(row.WorkOrder.CancelledTs),
		AllowedEvents:   allowedEventStrings(workorder.Status(row.WorkOrder.Status)),
	}
}

func numericToStringPtr(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	raw, err := n.MarshalJSON()
	if err != nil || string(raw) == "null" {
		return nil
	}
	out := strings.Trim(string(raw), `"`)
	return &out
}

func timeString(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}

func timeStringPtr(ts pgtype.Timestamptz) *string {
	if !ts.Valid {
		return nil
	}
	out := ts.Time.Format(time.RFC3339)
	return &out
}

func allowedEventStrings(status workorder.Status) []string {
	events := workorder.AllowedEvents(status)
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, string(event))
	}
	return out
}

func isKnownWorkOrderStatus(status string) bool {
	switch workorder.Status(status) {
	case workorder.StatusReceived,
		workorder.StatusDiagnosing,
		workorder.StatusQuoted,
		workorder.StatusApproved,
		workorder.StatusRejected,
		workorder.StatusInRepair,
		workorder.StatusWaitingParts,
		workorder.StatusReady,
		workorder.StatusDelivered,
		workorder.StatusCancelled:
		return true
	default:
		return false
	}
}

func trimIntakeReq(req *intakeReq) {
	req.ClientUcode = strings.TrimSpace(req.ClientUcode)
	req.DeviceUcode = strings.TrimSpace(req.DeviceUcode)
	req.ServiceType = strings.TrimSpace(req.ServiceType)
	req.ReportedIssue = strings.TrimSpace(req.ReportedIssue)
	trimStringPtr(req.IntakeNotes)
	trimStringPtr(req.Accessories)
	trimStringPtr(req.DevicePin)
}
