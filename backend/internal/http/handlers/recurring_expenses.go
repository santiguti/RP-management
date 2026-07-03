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

	"github.com/santiguti/rp-management/backend/internal/audit"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	recurringdomain "github.com/santiguti/rp-management/backend/internal/domain/recurring"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type recurringExpenseDTO struct {
	Ucode             string           `json:"ucode"`
	Name              string           `json:"name"`
	Amount            string           `json:"amount"`
	Currency          string           `json:"currency"`
	DayOfMonth        int              `json:"day_of_month"`
	Category          string           `json:"category"`
	PaymentMethod     string           `json:"payment_method"`
	Supplier          *counterpartyRef `json:"supplier,omitempty"`
	Description       *string          `json:"description,omitempty"`
	Active            bool             `json:"active"`
	LastGeneratedDate *string          `json:"last_generated_date,omitempty"`
	CreatedTs         string           `json:"created_ts"`
}

type createRecurringReq struct {
	Name          string  `json:"name" validate:"required,min=1,max=200"`
	Amount        string  `json:"amount" validate:"required"`
	Currency      *string `json:"currency" validate:"omitempty,len=3"`
	DayOfMonth    int     `json:"day_of_month" validate:"required,min=1,max=28"`
	Category      string  `json:"category" validate:"required,oneof=rent utilities salary taxes supplies other_expense"`
	PaymentMethod *string `json:"payment_method" validate:"omitempty,oneof=cash transfer card mercadopago other"`
	SupplierUcode *string `json:"supplier_ucode" validate:"omitempty"`
	Description   *string `json:"description" validate:"omitempty,max=2000"`
	Active        *bool   `json:"active" validate:"omitempty"`
}

type updateRecurringReq struct {
	Name          *string `json:"name" validate:"omitempty,min=1,max=200"`
	Amount        *string `json:"amount" validate:"omitempty"`
	Currency      *string `json:"currency" validate:"omitempty,len=3"`
	DayOfMonth    *int    `json:"day_of_month" validate:"omitempty,min=1,max=28"`
	Category      *string `json:"category" validate:"omitempty,oneof=rent utilities salary taxes supplies other_expense"`
	PaymentMethod *string `json:"payment_method" validate:"omitempty,oneof=cash transfer card mercadopago other"`
	SupplierUcode *string `json:"supplier_ucode" validate:"omitempty"`
	Description   *string `json:"description" validate:"omitempty,max=2000"`
	Active        *bool   `json:"active" validate:"omitempty"`
}

type RecurringExpenses struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

type recurringExpenseEnriched struct {
	recurringExpense sqlc.RecurringExpense
	supplierUcode    pgtype.UUID
	supplierName     pgtype.Text
}

func NewRecurringExpenses(q *sqlc.Queries) *RecurringExpenses {
	return &RecurringExpenses{queries: q, val: validator.New()}
}

func (re *RecurringExpenses) List(rw http.ResponseWriter, r *http.Request) {
	rows, err := re.queries.ListRecurringExpenses(r.Context())
	if err != nil {
		log.Printf("list recurring expenses: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]recurringExpenseDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRecurringExpenseDTO(enrichedFromListRecurring(row)))
	}
	writeJSON(rw, http.StatusOK, map[string][]recurringExpenseDTO{"recurring_expenses": out})
}

func (re *RecurringExpenses) Create(rw http.ResponseWriter, r *http.Request) {
	var req createRecurringReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimCreateRecurringReq(&req)
	if err := re.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	amount, ok := parsePositiveNumeric(rw, req.Amount, "invalid_amount")
	if !ok {
		return
	}
	supplierID, ok := re.resolveOptionalSupplier(rw, r, req.SupplierUcode)
	if !ok {
		return
	}
	currency := "ARS"
	if req.Currency != nil {
		currency = *req.Currency
	}
	paymentMethod := "transfer"
	if req.PaymentMethod != nil {
		paymentMethod = *req.PaymentMethod
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	user, _ := middleware.UserFromContext(r.Context())
	out, err := re.queries.CreateRecurringExpense(r.Context(), sqlc.CreateRecurringExpenseParams{
		Name:            req.Name,
		Amount:          amount,
		Currency:        currency,
		DayOfMonth:      int32(req.DayOfMonth),
		Category:        req.Category,
		PaymentMethod:   paymentMethod,
		SupplierID:      supplierID,
		Description:     textFromPtr(req.Description),
		Active:          active,
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("create recurring expense: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	row, ok := re.recurringByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	dto := toRecurringExpenseDTO(row)
	audit.Record(r.Context(), re.queries, r, audit.Entry{
		Action:      "recurring.create",
		EntityType:  "recurring_expense",
		EntityID:    &row.recurringExpense.ID,
		EntityUcode: &row.recurringExpense.Ucode,
		After:       dto,
	})
	writeJSON(rw, http.StatusCreated, map[string]recurringExpenseDTO{"recurring_expense": dto})
}

func (re *RecurringExpenses) Get(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	row, ok := re.recurringByUcode(rw, r, ucode)
	if !ok {
		return
	}
	writeJSON(rw, http.StatusOK, map[string]recurringExpenseDTO{"recurring_expense": toRecurringExpenseDTO(row)})
}

func (re *RecurringExpenses) Update(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	current, ok := re.recurringByUcode(rw, r, ucode)
	if !ok {
		return
	}

	var req updateRecurringReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimUpdateRecurringReq(&req)
	if err := re.val.Struct(req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdateRecurringExpenseParams{
		ID:            current.recurringExpense.ID,
		Name:          current.recurringExpense.Name,
		Amount:        current.recurringExpense.Amount,
		Currency:      current.recurringExpense.Currency,
		DayOfMonth:    current.recurringExpense.DayOfMonth,
		Category:      current.recurringExpense.Category,
		PaymentMethod: current.recurringExpense.PaymentMethod,
		SupplierID:    current.recurringExpense.SupplierID,
		Description:   current.recurringExpense.Description,
		Active:        current.recurringExpense.Active,
	}
	if req.Name != nil {
		params.Name = *req.Name
	}
	if req.Amount != nil {
		amount, ok := parsePositiveNumeric(rw, *req.Amount, "invalid_amount")
		if !ok {
			return
		}
		params.Amount = amount
	}
	if req.Currency != nil {
		params.Currency = *req.Currency
	}
	if req.DayOfMonth != nil {
		params.DayOfMonth = int32(*req.DayOfMonth)
	}
	if req.Category != nil {
		params.Category = *req.Category
	}
	if req.PaymentMethod != nil {
		params.PaymentMethod = *req.PaymentMethod
	}
	if req.SupplierUcode != nil {
		supplierID, ok := re.resolveOptionalSupplier(rw, r, req.SupplierUcode)
		if !ok {
			return
		}
		params.SupplierID = supplierID
	}
	if req.Description != nil {
		params.Description = textFromPtr(req.Description)
	}
	if req.Active != nil {
		params.Active = *req.Active
	}

	beforeDTO := toRecurringExpenseDTO(current)
	out, err := re.queries.UpdateRecurringExpense(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update recurring expense: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	row, ok := re.recurringByUcode(rw, r, out.Ucode)
	if !ok {
		return
	}
	afterDTO := toRecurringExpenseDTO(row)
	audit.Record(r.Context(), re.queries, r, audit.Entry{
		Action:      "recurring.update",
		EntityType:  "recurring_expense",
		EntityID:    &row.recurringExpense.ID,
		EntityUcode: &row.recurringExpense.Ucode,
		Before:      beforeDTO,
		After:       afterDTO,
	})
	writeJSON(rw, http.StatusOK, map[string]recurringExpenseDTO{"recurring_expense": afterDTO})
}

func (re *RecurringExpenses) Delete(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	current, err := re.queries.GetRecurringExpenseByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		rw.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("get recurring expense for delete: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := re.queries.SoftDeleteRecurringExpense(r.Context(), sqlc.SoftDeleteRecurringExpenseParams{
		ID:             current.RecurringExpense.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete recurring expense: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	before := toRecurringExpenseDTO(enrichedFromGetRecurring(current))
	audit.Record(r.Context(), re.queries, r, audit.Entry{
		Action:      "recurring.delete",
		EntityType:  "recurring_expense",
		EntityID:    &current.RecurringExpense.ID,
		EntityUcode: &current.RecurringExpense.Ucode,
		Before:      before,
	})
	rw.WriteHeader(http.StatusNoContent)
}

func (re *RecurringExpenses) RunNow(rw http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(rw, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	current, ok := re.recurringByUcode(rw, r, ucode)
	if !ok {
		return
	}
	result, err := recurringdomain.ProcessOne(r.Context(), re.queries, current.recurringExpense, time.Now().UTC())
	if err != nil {
		log.Printf("run recurring expense now: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if !result.Generated {
		writeJSON(rw, http.StatusOK, map[string]any{
			"transaction": nil,
			"reason":      result.Reason,
		})
		return
	}
	row, err := re.queries.GetTransactionByUcode(r.Context(), result.Transaction.Ucode)
	if err != nil {
		log.Printf("get recurring transaction: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	dto := toTransactionDTO(enrichedFromGetTransaction(row))
	audit.Record(r.Context(), re.queries, r, audit.Entry{
		Action:      "recurring.run_now",
		EntityType:  "recurring_expense",
		EntityID:    &current.recurringExpense.ID,
		EntityUcode: &current.recurringExpense.Ucode,
		After: map[string]string{
			"transaction_ucode": uuidString(row.Transaction.Ucode),
		},
	})
	writeJSON(rw, http.StatusOK, map[string]transactionDTO{"transaction": dto})
}

func (re *RecurringExpenses) resolveOptionalSupplier(rw http.ResponseWriter, r *http.Request, raw *string) (pgtype.Int8, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.Int8{}, true
	}
	ucode, err := uuidFromString(*raw)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_supplier"})
		return pgtype.Int8{}, false
	}
	supplier, err := re.queries.GetSupplierByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "unknown_supplier"})
		return pgtype.Int8{}, false
	}
	if err != nil {
		log.Printf("resolve recurring supplier: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return pgtype.Int8{}, false
	}
	return pgtype.Int8{Int64: supplier.ID, Valid: true}, true
}

func (re *RecurringExpenses) recurringByUcode(rw http.ResponseWriter, r *http.Request, ucode pgtype.UUID) (recurringExpenseEnriched, bool) {
	row, err := re.queries.GetRecurringExpenseByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "not_found"})
		return recurringExpenseEnriched{}, false
	}
	if err != nil {
		log.Printf("get recurring expense: %v", err)
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return recurringExpenseEnriched{}, false
	}
	return enrichedFromGetRecurring(row), true
}

func toRecurringExpenseDTO(row recurringExpenseEnriched) recurringExpenseDTO {
	return recurringExpenseDTO{
		Ucode:             uuidString(row.recurringExpense.Ucode),
		Name:              row.recurringExpense.Name,
		Amount:            numericToString(row.recurringExpense.Amount),
		Currency:          row.recurringExpense.Currency,
		DayOfMonth:        int(row.recurringExpense.DayOfMonth),
		Category:          row.recurringExpense.Category,
		PaymentMethod:     row.recurringExpense.PaymentMethod,
		Supplier:          counterpartyRefFrom(row.supplierUcode, row.supplierName),
		Description:       stringPtrFromText(row.recurringExpense.Description),
		Active:            row.recurringExpense.Active,
		LastGeneratedDate: dateStringPtr(row.recurringExpense.LastGeneratedDate),
		CreatedTs:         timeString(row.recurringExpense.CreatedTs),
	}
}

func enrichedFromGetRecurring(row sqlc.GetRecurringExpenseByUcodeRow) recurringExpenseEnriched {
	return recurringExpenseEnriched{
		recurringExpense: row.RecurringExpense,
		supplierUcode:    row.SupplierUcode,
		supplierName:     row.SupplierName,
	}
}

func enrichedFromListRecurring(row sqlc.ListRecurringExpensesRow) recurringExpenseEnriched {
	return recurringExpenseEnriched{
		recurringExpense: row.RecurringExpense,
		supplierUcode:    row.SupplierUcode,
		supplierName:     row.SupplierName,
	}
}

func dateStringPtr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	out := d.Time.Format("2006-01-02")
	return &out
}

func trimCreateRecurringReq(req *createRecurringReq) {
	req.Name = strings.TrimSpace(req.Name)
	req.Amount = strings.TrimSpace(req.Amount)
	if req.Currency != nil {
		*req.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	req.Category = strings.TrimSpace(req.Category)
	trimStringPtr(req.PaymentMethod)
	trimStringPtr(req.SupplierUcode)
	trimStringPtr(req.Description)
}

func trimUpdateRecurringReq(req *updateRecurringReq) {
	trimStringPtr(req.Name)
	trimStringPtr(req.Amount)
	if req.Currency != nil {
		*req.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	trimStringPtr(req.Category)
	trimStringPtr(req.PaymentMethod)
	trimStringPtr(req.SupplierUcode)
	trimStringPtr(req.Description)
}
