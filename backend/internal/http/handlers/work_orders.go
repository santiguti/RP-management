package handlers

import (
	"encoding/json"
	"errors"
	"io"
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

type updateWorkOrderReq struct {
	ServiceType   *string `json:"service_type" validate:"omitempty,oneof=in_shop on_site"`
	ReportedIssue *string `json:"reported_issue" validate:"omitempty,min=1,max=2000"`
	Diagnosis     *string `json:"diagnosis" validate:"omitempty,max=4000"`
	IntakeNotes   *string `json:"intake_notes" validate:"omitempty,max=4000"`
	Accessories   *string `json:"accessories" validate:"omitempty,max=2000"`
	DevicePin     *string `json:"device_pin" validate:"omitempty,max=64"`
}

type transitionReq struct {
	QuoteAmount   *string `json:"quote_amount" validate:"omitempty"`
	QuoteCurrency *string `json:"quote_currency" validate:"omitempty,len=3"`
	Diagnosis     *string `json:"diagnosis" validate:"omitempty,max=4000"`
	LaborAmount   *string `json:"labor_amount" validate:"omitempty"`
	PartsAmount   *string `json:"parts_amount" validate:"omitempty"`
	FinalAmount   *string `json:"final_amount" validate:"omitempty"`
	CancelReason  *string `json:"cancel_reason" validate:"omitempty,max=2000"`
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

func (w *WorkOrders) Update(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	current, ok := w.workOrderByUcode(rw, r, ucode)
	if !ok {
		return
	}

	var req updateWorkOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimUpdateWorkOrderReq(&req)
	if err := w.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdateWorkOrderFieldsParams{
		ID:                 current.WorkOrder.ID,
		ServiceType:        current.WorkOrder.ServiceType,
		ReportedIssue:      current.WorkOrder.ReportedIssue,
		Diagnosis:          current.WorkOrder.Diagnosis,
		IntakeNotes:        current.WorkOrder.IntakeNotes,
		Accessories:        current.WorkOrder.Accessories,
		DevicePinEncrypted: current.WorkOrder.DevicePinEncrypted,
	}
	if req.ServiceType != nil {
		params.ServiceType = *req.ServiceType
	}
	if req.ReportedIssue != nil {
		params.ReportedIssue = *req.ReportedIssue
	}
	if req.Diagnosis != nil {
		params.Diagnosis = textFromPtr(req.Diagnosis)
	}
	if req.IntakeNotes != nil {
		params.IntakeNotes = textFromPtr(req.IntakeNotes)
	}
	if req.Accessories != nil {
		params.Accessories = textFromPtr(req.Accessories)
	}
	if req.DevicePin != nil {
		params.DevicePinEncrypted = textFromPtr(req.DevicePin)
	}

	out, err := w.queries.UpdateWorkOrderFields(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update work order fields: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	updated, ok := w.workOrderByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	writeJSON(rw, http.StatusOK, map[string]workOrderDTO{"work_order": toWorkOrderDTO(updated)})
}

func (w *WorkOrders) Transition(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	event := workorder.Event(strings.TrimSpace(chi.URLParam(r, "event")))
	if !isKnownWorkOrderEvent(event) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_event"})
		return
	}
	current, ok := w.workOrderByUcode(rw, r, ucode)
	if !ok {
		return
	}
	currentStatus := workorder.Status(current.WorkOrder.Status)
	newStatus, err := workorder.Next(currentStatus, event)
	if errors.Is(err, workorder.ErrInvalidTransition) {
		writeJSON(rw, http.StatusConflict, map[string]any{
			"error":          "invalid_transition",
			"from":           string(currentStatus),
			"event":          string(event),
			"allowed_events": allowedEventStrings(currentStatus),
		})
		return
	}
	if errors.Is(err, workorder.ErrUnknownEvent) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_event"})
		return
	}
	if errors.Is(err, workorder.ErrUnknownStatus) {
		log.Printf("work order %s has unknown status %q", current.WorkOrder.WoNumber, current.WorkOrder.Status)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if err != nil {
		log.Printf("work order transition: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var req transitionReq
	if err := decodeOptionalJSON(r.Body, &req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimTransitionReq(&req)
	if err := w.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	out, ok := w.applyTransition(rw, r, current.WorkOrder, event, newStatus, req)
	if !ok {
		return
	}
	updated, ok := w.workOrderByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	writeJSON(rw, http.StatusOK, map[string]workOrderDTO{"work_order": toWorkOrderDTO(updated)})
}

func (w *WorkOrders) applyTransition(rw http.ResponseWriter, r *http.Request, current sqlc.WorkOrder, event workorder.Event, newStatus workorder.Status, req transitionReq) (sqlc.WorkOrder, bool) {
	switch event {
	case workorder.EventQuote:
		if req.QuoteAmount == nil || strings.TrimSpace(*req.QuoteAmount) == "" {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return sqlc.WorkOrder{}, false
		}
		quoteAmount, err := stringToNumeric(*req.QuoteAmount)
		if err != nil {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return sqlc.WorkOrder{}, false
		}
		out, err := w.queries.SetWorkOrderQuote(r.Context(), sqlc.SetWorkOrderQuoteParams{
			ID:            current.ID,
			Diagnosis:     textFromPtr(req.Diagnosis),
			QuoteAmount:   quoteAmount,
			QuoteCurrency: textFromPtr(req.QuoteCurrency),
		})
		return w.transitionResult(rw, out, err, "set work order quote")
	case workorder.EventApprove, workorder.EventReject:
		out, err := w.queries.SetWorkOrderQuoteOutcome(r.Context(), sqlc.SetWorkOrderQuoteOutcomeParams{
			ID:     current.ID,
			Status: string(newStatus),
		})
		return w.transitionResult(rw, out, err, "set work order quote outcome")
	case workorder.EventMarkReady:
		laborAmount, ok := numericFromPtr(rw, req.LaborAmount)
		if !ok {
			return sqlc.WorkOrder{}, false
		}
		partsAmount, ok := numericFromPtr(rw, req.PartsAmount)
		if !ok {
			return sqlc.WorkOrder{}, false
		}
		finalAmount, ok := numericFromPtr(rw, req.FinalAmount)
		if !ok {
			return sqlc.WorkOrder{}, false
		}
		params := sqlc.SetWorkOrderFinalsParams{
			ID:          current.ID,
			Diagnosis:   textFromPtr(req.Diagnosis),
			LaborAmount: laborAmount,
			PartsAmount: partsAmount,
			FinalAmount: finalAmount,
		}
		out, err := w.queries.SetWorkOrderFinals(r.Context(), params)
		return w.transitionResult(rw, out, err, "set work order finals")
	case workorder.EventCancel:
		out, err := w.queries.UpdateWorkOrderStatus(r.Context(), sqlc.UpdateWorkOrderStatusParams{
			ID:           current.ID,
			Status:       string(newStatus),
			CancelReason: textFromPtr(req.CancelReason),
		})
		return w.transitionResult(rw, out, err, "cancel work order")
	default:
		out, err := w.queries.UpdateWorkOrderStatus(r.Context(), sqlc.UpdateWorkOrderStatusParams{
			ID:     current.ID,
			Status: string(newStatus),
		})
		return w.transitionResult(rw, out, err, "update work order status")
	}
}

func (w *WorkOrders) transitionResult(rw http.ResponseWriter, out sqlc.WorkOrder, err error, logPrefix string) (sqlc.WorkOrder, bool) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.WorkOrder{}, false
	}
	if err != nil {
		log.Printf("%s: %v", logPrefix, err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.WorkOrder{}, false
	}
	return out, true
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

func stringToNumeric(raw string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(strings.TrimSpace(raw)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

func numericFromPtr(rw http.ResponseWriter, raw *string) (pgtype.Numeric, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		if raw != nil {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
			return pgtype.Numeric{}, false
		}
		return pgtype.Numeric{}, true
	}
	n, err := stringToNumeric(*raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return pgtype.Numeric{}, false
	}
	return n, true
}

func decodeOptionalJSON(r io.Reader, dst any) error {
	err := json.NewDecoder(r).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
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

func isKnownWorkOrderEvent(event workorder.Event) bool {
	switch event {
	case workorder.EventStartDiagnosis,
		workorder.EventQuote,
		workorder.EventApprove,
		workorder.EventReject,
		workorder.EventStartRepair,
		workorder.EventMarkWaitingParts,
		workorder.EventResumeRepair,
		workorder.EventMarkReady,
		workorder.EventDeliver,
		workorder.EventCancel:
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

func trimUpdateWorkOrderReq(req *updateWorkOrderReq) {
	trimStringPtr(req.ServiceType)
	trimStringPtr(req.ReportedIssue)
	trimStringPtr(req.Diagnosis)
	trimStringPtr(req.IntakeNotes)
	trimStringPtr(req.Accessories)
	trimStringPtr(req.DevicePin)
}

func trimTransitionReq(req *transitionReq) {
	trimStringPtr(req.QuoteAmount)
	if req.QuoteCurrency != nil {
		*req.QuoteCurrency = strings.ToUpper(strings.TrimSpace(*req.QuoteCurrency))
	}
	trimStringPtr(req.Diagnosis)
	trimStringPtr(req.LaborAmount)
	trimStringPtr(req.PartsAmount)
	trimStringPtr(req.FinalAmount)
	trimStringPtr(req.CancelReason)
}
