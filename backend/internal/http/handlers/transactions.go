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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/audit"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/money"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type transactionDTO struct {
	Ucode            string           `json:"ucode"`
	TransactionType  string           `json:"transaction_type"`
	Amount           string           `json:"amount"`
	Currency         string           `json:"currency"`
	FxRateToArs      string           `json:"fx_rate_to_ars"`
	TransactionDate  string           `json:"transaction_date"`
	PaymentMethod    string           `json:"payment_method"`
	Category         string           `json:"category"`
	CounterpartyType string           `json:"counterparty_type"`
	Client           *counterpartyRef `json:"client,omitempty"`
	Supplier         *counterpartyRef `json:"supplier,omitempty"`
	WorkOrder        *workOrderRef    `json:"work_order,omitempty"`
	RecurringExpense *counterpartyRef `json:"recurring_expense,omitempty"`
	Description      *string          `json:"description,omitempty"`
	CreatedTs        string           `json:"created_ts"`
}

type counterpartyRef struct {
	Ucode string `json:"ucode"`
	Name  string `json:"name"`
}

type workOrderRef struct {
	Ucode    string `json:"ucode"`
	WoNumber string `json:"wo_number"`
}

type createTransactionReq struct {
	TransactionType  string  `json:"transaction_type" validate:"required,oneof=income expense"`
	Amount           string  `json:"amount" validate:"required"`
	Currency         *string `json:"currency" validate:"omitempty,len=3"`
	FxRateToArs      *string `json:"fx_rate_to_ars" validate:"omitempty"`
	TransactionDate  *string `json:"transaction_date" validate:"omitempty"`
	PaymentMethod    string  `json:"payment_method" validate:"required,oneof=cash transfer card mercadopago other"`
	Category         string  `json:"category" validate:"required,oneof=wo_payment wo_deposit part_purchase supplies rent utilities salary taxes food transport other_income other_expense"`
	CounterpartyType string  `json:"counterparty_type" validate:"required,oneof=client supplier none"`
	ClientUcode      *string `json:"client_ucode" validate:"omitempty"`
	SupplierUcode    *string `json:"supplier_ucode" validate:"omitempty"`
	WorkOrderUcode   *string `json:"work_order_ucode" validate:"omitempty"`
	Description      *string `json:"description" validate:"omitempty,max=2000"`
}

// Financial identity fields are intentionally absent here. Patching amount,
// type, currency, counterparty, or FKs is a no-op; fix those by delete/create.
type updateTransactionReq struct {
	TransactionDate *string `json:"transaction_date" validate:"omitempty"`
	PaymentMethod   *string `json:"payment_method" validate:"omitempty,oneof=cash transfer card mercadopago other"`
	Category        *string `json:"category" validate:"omitempty,oneof=wo_payment wo_deposit part_purchase supplies rent utilities salary taxes food transport other_income other_expense"`
	Description     *string `json:"description" validate:"omitempty,max=2000"`
}

type Transactions struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

type transactionEnriched struct {
	transaction           sqlc.Transaction
	clientUcode           pgtype.UUID
	clientName            pgtype.Text
	supplierUcode         pgtype.UUID
	supplierName          pgtype.Text
	workOrderUcode        pgtype.UUID
	workOrderNumber       pgtype.Text
	recurringExpenseUcode pgtype.UUID
	recurringExpenseName  pgtype.Text
}

func NewTransactions(q *sqlc.Queries) *Transactions {
	return &Transactions{queries: q, val: validator.New()}
}

func (t *Transactions) Create(rw http.ResponseWriter, r *http.Request) {
	var req createTransactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimCreateTransactionReq(&req)
	if err := t.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	amount, ok := parsePositiveNumeric(rw, req.Amount, "invalid_amount")
	if !ok {
		return
	}
	fxRaw := "1"
	if req.FxRateToArs != nil {
		fxRaw = *req.FxRateToArs
	}
	fxRate, ok := parsePositiveNumeric(rw, fxRaw, "invalid_fx_rate")
	if !ok {
		return
	}
	transactionDate, ok := parseTransactionDatePtr(rw, req.TransactionDate, true)
	if !ok {
		return
	}

	clientID, supplierID, ok := t.resolveCounterparty(rw, r, req.CounterpartyType, req.ClientUcode, req.SupplierUcode)
	if !ok {
		return
	}
	workOrderID, ok := t.resolveOptionalWorkOrder(rw, r, req.WorkOrderUcode)
	if !ok {
		return
	}

	currency := "ARS"
	if req.Currency != nil {
		currency = *req.Currency
	}
	user, _ := middleware.UserFromContext(r.Context())
	out, err := t.queries.CreateTransaction(r.Context(), sqlc.CreateTransactionParams{
		TransactionType:  req.TransactionType,
		Amount:           amount,
		Currency:         currency,
		FxRateToArs:      fxRate,
		TransactionDate:  transactionDate,
		PaymentMethod:    req.PaymentMethod,
		Category:         req.Category,
		CounterpartyType: req.CounterpartyType,
		ClientID:         clientID,
		SupplierID:       supplierID,
		WorkOrderID:      workOrderID,
		Description:      textFromPtr(req.Description),
		CreatedByUserID:  pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if isForeignKeyViolation(err) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_entity"})
		return
	}
	if err != nil {
		log.Printf("create transaction: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	row, ok := t.transactionByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	dto := toTransactionDTO(row)
	audit.Record(r.Context(), t.queries, r, audit.Entry{
		Action:      "transaction.create",
		EntityType:  "transaction",
		EntityID:    &row.transaction.ID,
		EntityUcode: &row.transaction.Ucode,
		After:       dto,
	})
	writeJSON(rw, http.StatusCreated, map[string]transactionDTO{"transaction": dto})
}

func (t *Transactions) Search(rw http.ResponseWriter, r *http.Request) {
	params, countParams, page, pageSize, ok := t.parseSearchParams(rw, r)
	if !ok {
		return
	}
	total, err := t.queries.CountTransactions(r.Context(), countParams)
	if err != nil {
		log.Printf("count transactions: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	rows, err := t.queries.ListTransactions(r.Context(), params)
	if err != nil {
		log.Printf("list transactions: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]transactionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toTransactionDTO(enrichedFromListTransaction(row)))
	}
	writeJSON(rw, http.StatusOK, map[string]any{
		"transactions": out,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

func (t *Transactions) Get(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	row, ok := t.transactionByUcode(rw, r, ucode)
	if !ok {
		return
	}
	writeJSON(rw, http.StatusOK, map[string]transactionDTO{"transaction": toTransactionDTO(row)})
}

func (t *Transactions) Update(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	current, ok := t.transactionByUcode(rw, r, ucode)
	if !ok {
		return
	}

	var req updateTransactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimUpdateTransactionReq(&req)
	if err := t.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdateTransactionParams{
		ID:              current.transaction.ID,
		TransactionDate: current.transaction.TransactionDate,
		PaymentMethod:   current.transaction.PaymentMethod,
		Category:        current.transaction.Category,
		Description:     current.transaction.Description,
	}
	if req.TransactionDate != nil {
		transactionDate, ok := parseTransactionDatePtr(rw, req.TransactionDate, false)
		if !ok {
			return
		}
		params.TransactionDate = transactionDate
	}
	if req.PaymentMethod != nil {
		params.PaymentMethod = *req.PaymentMethod
	}
	if req.Category != nil {
		params.Category = *req.Category
	}
	if req.Description != nil {
		params.Description = textFromPtr(req.Description)
	}

	beforeDTO := toTransactionDTO(current)
	out, err := t.queries.UpdateTransaction(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update transaction: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	row, ok := t.transactionByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	afterDTO := toTransactionDTO(row)
	audit.Record(r.Context(), t.queries, r, audit.Entry{
		Action:      "transaction.update",
		EntityType:  "transaction",
		EntityID:    &row.transaction.ID,
		EntityUcode: &row.transaction.Ucode,
		Before:      beforeDTO,
		After:       afterDTO,
	})
	writeJSON(rw, http.StatusOK, map[string]transactionDTO{"transaction": afterDTO})
}

func (t *Transactions) Delete(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	current, err := t.queries.GetTransactionByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		rw.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("get transaction for delete: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := t.queries.SoftDeleteTransaction(r.Context(), sqlc.SoftDeleteTransactionParams{
		ID:             current.Transaction.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete transaction: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	currentDTO := toTransactionDTO(enrichedFromGetTransaction(current))
	audit.Record(r.Context(), t.queries, r, audit.Entry{
		Action:      "transaction.delete",
		EntityType:  "transaction",
		EntityID:    &current.Transaction.ID,
		EntityUcode: &current.Transaction.Ucode,
		Before:      currentDTO,
	})
	rw.WriteHeader(http.StatusNoContent)
}

func (t *Transactions) parseSearchParams(rw http.ResponseWriter, r *http.Request) (sqlc.ListTransactionsParams, sqlc.CountTransactionsParams, int, int, bool) {
	q := r.URL.Query()
	page := parsePositiveInt(q.Get("page"), 1)
	pageSize := parsePositiveInt(q.Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}
	transactionType := strings.TrimSpace(q.Get("type"))
	category := strings.TrimSpace(q.Get("category"))
	if transactionType != "" && !isKnownTransactionType(transactionType) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
		return sqlc.ListTransactionsParams{}, sqlc.CountTransactionsParams{}, 0, 0, false
	}
	if category != "" && !isKnownTransactionCategory(category) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
		return sqlc.ListTransactionsParams{}, sqlc.CountTransactionsParams{}, 0, 0, false
	}

	params := sqlc.ListTransactionsParams{
		TransactionType: transactionType,
		Category:        category,
		PageSize:        int32(pageSize),
		PageOffset:      int32((page - 1) * pageSize),
	}
	countParams := sqlc.CountTransactionsParams{
		TransactionType: transactionType,
		Category:        category,
	}
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		date, ok := parseTransactionDate(rw, raw)
		if !ok {
			return sqlc.ListTransactionsParams{}, sqlc.CountTransactionsParams{}, 0, 0, false
		}
		params.HasFrom = true
		params.DateFrom = date
		countParams.HasFrom = true
		countParams.DateFrom = date
	}
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		date, ok := parseTransactionDate(rw, raw)
		if !ok {
			return sqlc.ListTransactionsParams{}, sqlc.CountTransactionsParams{}, 0, 0, false
		}
		params.HasTo = true
		params.DateTo = date
		countParams.HasTo = true
		countParams.DateTo = date
	}
	if raw := strings.TrimSpace(q.Get("work_order_ucode")); raw != "" {
		workOrder, ok := t.resolveWorkOrder(rw, r, raw)
		if !ok {
			return sqlc.ListTransactionsParams{}, sqlc.CountTransactionsParams{}, 0, 0, false
		}
		params.HasWorkOrder = true
		params.WorkOrderID = workOrder.WorkOrder.ID
		countParams.HasWorkOrder = true
		countParams.WorkOrderID = workOrder.WorkOrder.ID
	}
	return params, countParams, page, pageSize, true
}

func (t *Transactions) resolveCounterparty(rw http.ResponseWriter, r *http.Request, counterpartyType string, clientRaw, supplierRaw *string) (pgtype.Int8, pgtype.Int8, bool) {
	hasClient := clientRaw != nil && strings.TrimSpace(*clientRaw) != ""
	hasSupplier := supplierRaw != nil && strings.TrimSpace(*supplierRaw) != ""
	switch counterpartyType {
	case "client":
		if !hasClient || hasSupplier {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "counterparty_mismatch"})
			return pgtype.Int8{}, pgtype.Int8{}, false
		}
		client, ok := t.resolveClient(rw, r, *clientRaw)
		if !ok {
			return pgtype.Int8{}, pgtype.Int8{}, false
		}
		return pgtype.Int8{Int64: client.ID, Valid: true}, pgtype.Int8{}, true
	case "supplier":
		if !hasSupplier || hasClient {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "counterparty_mismatch"})
			return pgtype.Int8{}, pgtype.Int8{}, false
		}
		supplier, ok := t.resolveSupplier(rw, r, *supplierRaw)
		if !ok {
			return pgtype.Int8{}, pgtype.Int8{}, false
		}
		return pgtype.Int8{}, pgtype.Int8{Int64: supplier.ID, Valid: true}, true
	case "none":
		if hasClient || hasSupplier {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "counterparty_mismatch"})
			return pgtype.Int8{}, pgtype.Int8{}, false
		}
		return pgtype.Int8{}, pgtype.Int8{}, true
	default:
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "counterparty_mismatch"})
		return pgtype.Int8{}, pgtype.Int8{}, false
	}
}

func (t *Transactions) resolveClient(rw http.ResponseWriter, r *http.Request, raw string) (sqlc.Client, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
		return sqlc.Client{}, false
	}
	client, err := t.queries.GetClientByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
		return sqlc.Client{}, false
	}
	if err != nil {
		log.Printf("resolve transaction client: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Client{}, false
	}
	return client, true
}

func (t *Transactions) resolveSupplier(rw http.ResponseWriter, r *http.Request, raw string) (sqlc.Supplier, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_supplier"})
		return sqlc.Supplier{}, false
	}
	supplier, err := t.queries.GetSupplierByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_supplier"})
		return sqlc.Supplier{}, false
	}
	if err != nil {
		log.Printf("resolve transaction supplier: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Supplier{}, false
	}
	return supplier, true
}

func (t *Transactions) resolveOptionalWorkOrder(rw http.ResponseWriter, r *http.Request, raw *string) (pgtype.Int8, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.Int8{}, true
	}
	workOrder, ok := t.resolveWorkOrder(rw, r, *raw)
	if !ok {
		return pgtype.Int8{}, false
	}
	return pgtype.Int8{Int64: workOrder.WorkOrder.ID, Valid: true}, true
}

func (t *Transactions) resolveWorkOrder(rw http.ResponseWriter, r *http.Request, raw string) (sqlc.GetWorkOrderByUcodeRow, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_work_order"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	workOrder, err := t.queries.GetWorkOrderByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_work_order"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	if err != nil {
		log.Printf("resolve transaction work order: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	return workOrder, true
}

func (t *Transactions) transactionByUcode(rw http.ResponseWriter, r *http.Request, ucode pgtype.UUID) (transactionEnriched, bool) {
	row, err := t.queries.GetTransactionByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return transactionEnriched{}, false
	}
	if err != nil {
		log.Printf("get transaction: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return transactionEnriched{}, false
	}
	return enrichedFromGetTransaction(row), true
}

func toTransactionDTO(row transactionEnriched) transactionDTO {
	return transactionDTO{
		Ucode:            uuidString(row.transaction.Ucode),
		TransactionType:  row.transaction.TransactionType,
		Amount:           numericToString(row.transaction.Amount),
		Currency:         row.transaction.Currency,
		FxRateToArs:      numericToString(row.transaction.FxRateToArs),
		TransactionDate:  dateString(row.transaction.TransactionDate),
		PaymentMethod:    row.transaction.PaymentMethod,
		Category:         row.transaction.Category,
		CounterpartyType: row.transaction.CounterpartyType,
		Client:           counterpartyRefFrom(row.clientUcode, row.clientName),
		Supplier:         counterpartyRefFrom(row.supplierUcode, row.supplierName),
		WorkOrder:        workOrderRefFrom(row.workOrderUcode, row.workOrderNumber),
		RecurringExpense: counterpartyRefFrom(row.recurringExpenseUcode, row.recurringExpenseName),
		Description:      stringPtrFromText(row.transaction.Description),
		CreatedTs:        timeString(row.transaction.CreatedTs),
	}
}

func enrichedFromGetTransaction(row sqlc.GetTransactionByUcodeRow) transactionEnriched {
	return transactionEnriched{
		transaction:           row.Transaction,
		clientUcode:           row.ClientUcode,
		clientName:            row.ClientName,
		supplierUcode:         row.SupplierUcode,
		supplierName:          row.SupplierName,
		workOrderUcode:        row.WorkOrderUcode,
		workOrderNumber:       row.WorkOrderNumber,
		recurringExpenseUcode: row.RecurringExpenseUcode,
		recurringExpenseName:  row.RecurringExpenseName,
	}
}

func enrichedFromListTransaction(row sqlc.ListTransactionsRow) transactionEnriched {
	return transactionEnriched{
		transaction:           row.Transaction,
		clientUcode:           row.ClientUcode,
		clientName:            row.ClientName,
		supplierUcode:         row.SupplierUcode,
		supplierName:          row.SupplierName,
		workOrderUcode:        row.WorkOrderUcode,
		workOrderNumber:       row.WorkOrderNumber,
		recurringExpenseUcode: row.RecurringExpenseUcode,
		recurringExpenseName:  row.RecurringExpenseName,
	}
}

func enrichedFromWorkOrderTransaction(row sqlc.ListWorkOrderTransactionsRow) transactionEnriched {
	return transactionEnriched{
		transaction:           row.Transaction,
		clientUcode:           row.ClientUcode,
		clientName:            row.ClientName,
		supplierUcode:         row.SupplierUcode,
		supplierName:          row.SupplierName,
		workOrderUcode:        row.WorkOrderUcode,
		workOrderNumber:       row.WorkOrderNumber,
		recurringExpenseUcode: row.RecurringExpenseUcode,
		recurringExpenseName:  row.RecurringExpenseName,
	}
}

func counterpartyRefFrom(ucode pgtype.UUID, name pgtype.Text) *counterpartyRef {
	if !ucode.Valid || !name.Valid {
		return nil
	}
	return &counterpartyRef{Ucode: uuidString(ucode), Name: name.String}
}

func workOrderRefFrom(ucode pgtype.UUID, woNumber pgtype.Text) *workOrderRef {
	if !ucode.Valid || !woNumber.Valid {
		return nil
	}
	return &workOrderRef{Ucode: uuidString(ucode), WoNumber: woNumber.String}
}

func parsePositiveNumeric(rw http.ResponseWriter, raw, errorCode string) (pgtype.Numeric, bool) {
	n, err := money.StringToNumeric(raw)
	if err != nil || !n.Valid || n.Int == nil || n.Int.Sign() <= 0 {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": errorCode})
		return pgtype.Numeric{}, false
	}
	return n, true
}

func parseTransactionDatePtr(rw http.ResponseWriter, raw *string, defaultToday bool) (pgtype.Date, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		if !defaultToday {
			writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_date"})
			return pgtype.Date{}, false
		}
		now := time.Now().UTC()
		return pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true}, true
	}
	return parseTransactionDate(rw, *raw)
}

func parseTransactionDate(rw http.ResponseWriter, raw string) (pgtype.Date, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_date"})
		return pgtype.Date{}, false
	}
	return pgtype.Date{Time: parsed, Valid: true}, true
}

func numericToString(n pgtype.Numeric) string {
	if out := numericToStringPtr(n); out != nil {
		return *out
	}
	return ""
}

func dateString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func isKnownTransactionType(value string) bool {
	return value == "income" || value == "expense"
}

func isKnownTransactionCategory(value string) bool {
	switch value {
	case "wo_payment", "wo_deposit", "part_purchase", "supplies", "rent", "utilities", "salary", "taxes", "food", "transport", "other_income", "other_expense":
		return true
	default:
		return false
	}
}

func trimCreateTransactionReq(req *createTransactionReq) {
	req.TransactionType = strings.TrimSpace(req.TransactionType)
	req.Amount = strings.TrimSpace(req.Amount)
	if req.Currency != nil {
		*req.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	trimStringPtr(req.FxRateToArs)
	trimStringPtr(req.TransactionDate)
	req.PaymentMethod = strings.TrimSpace(req.PaymentMethod)
	req.Category = strings.TrimSpace(req.Category)
	req.CounterpartyType = strings.TrimSpace(req.CounterpartyType)
	trimStringPtr(req.ClientUcode)
	trimStringPtr(req.SupplierUcode)
	trimStringPtr(req.WorkOrderUcode)
	trimStringPtr(req.Description)
}

func trimUpdateTransactionReq(req *updateTransactionReq) {
	trimStringPtr(req.TransactionDate)
	trimStringPtr(req.PaymentMethod)
	trimStringPtr(req.Category)
	trimStringPtr(req.Description)
}
