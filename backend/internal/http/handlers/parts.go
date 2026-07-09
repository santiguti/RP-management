package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/audit"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/money"
	partdomain "github.com/santiguti/rp-management/backend/internal/domain/parts"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type partDTO struct {
	Ucode            string  `json:"ucode"`
	Sku              *string `json:"sku,omitempty"`
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	Unit             string  `json:"unit"`
	CurrentStock     string  `json:"current_stock"`
	ReorderLevel     *string `json:"reorder_level,omitempty"`
	DefaultCost      *string `json:"default_cost,omitempty"`
	DefaultSalePrice *string `json:"default_sale_price,omitempty"`
	LowStock         bool    `json:"low_stock"`
	CreatedTs        string  `json:"created_ts"`
}

type createPartReq struct {
	Sku              *string `json:"sku" validate:"omitempty,min=1,max=64"`
	Name             string  `json:"name" validate:"required,min=1,max=200"`
	Description      *string `json:"description" validate:"omitempty,max=4000"`
	Unit             *string `json:"unit" validate:"omitempty,min=1,max=32"`
	ReorderLevel     *string `json:"reorder_level" validate:"omitempty"`
	DefaultCost      *string `json:"default_cost" validate:"omitempty"`
	DefaultSalePrice *string `json:"default_sale_price" validate:"omitempty"`
}

type updatePartReq struct {
	Sku              *string `json:"sku" validate:"omitempty,min=1,max=64"`
	Name             *string `json:"name" validate:"omitempty,min=1,max=200"`
	Description      *string `json:"description" validate:"omitempty,max=4000"`
	Unit             *string `json:"unit" validate:"omitempty,min=1,max=32"`
	ReorderLevel     *string `json:"reorder_level" validate:"omitempty"`
	DefaultCost      *string `json:"default_cost" validate:"omitempty"`
	DefaultSalePrice *string `json:"default_sale_price" validate:"omitempty"`
}

type movementDTO struct {
	Ucode        string           `json:"ucode"`
	MovementType string           `json:"movement_type"`
	Quantity     string           `json:"quantity"`
	UnitCost     *string          `json:"unit_cost,omitempty"`
	Supplier     *counterpartyRef `json:"supplier,omitempty"`
	WorkOrder    *workOrderRef    `json:"work_order,omitempty"`
	Transaction  *movementTxnRef  `json:"transaction,omitempty"`
	Notes        *string          `json:"notes,omitempty"`
	CreatedTs    string           `json:"created_ts"`
}

type movementTxnRef struct {
	Ucode string `json:"ucode"`
}

type createMovementReq struct {
	MovementType    string  `json:"movement_type" validate:"required,oneof=purchase adjustment return"`
	Quantity        string  `json:"quantity" validate:"required"`
	AdjustmentOut   *bool   `json:"adjustment_out" validate:"omitempty"`
	UnitCost        *string `json:"unit_cost" validate:"omitempty"`
	PaymentMethod   *string `json:"payment_method" validate:"omitempty,oneof=cash transfer card mercadopago other"`
	SupplierUcode   *string `json:"supplier_ucode" validate:"omitempty"`
	Notes           *string `json:"notes" validate:"omitempty,max=2000"`
	LinkTransaction *bool   `json:"link_transaction" validate:"omitempty"`
}

type Parts struct {
	queries *sqlc.Queries
	pool    *pgxpool.Pool
	val     *validator.Validate
}

func NewParts(q *sqlc.Queries, pool *pgxpool.Pool) *Parts {
	return &Parts{queries: q, pool: pool, val: validator.New()}
}

func (p *Parts) Create(w http.ResponseWriter, r *http.Request) {
	var req createPartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimCreatePartReq(&req)
	if err := p.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	reorderLevel, ok := parseOptionalNonNegativeNumeric(w, req.ReorderLevel)
	if !ok {
		return
	}
	defaultCost, ok := parseOptionalNonNegativeNumeric(w, req.DefaultCost)
	if !ok {
		return
	}
	defaultSalePrice, ok := parseOptionalNonNegativeNumeric(w, req.DefaultSalePrice)
	if !ok {
		return
	}

	unit := "unidad"
	if req.Unit != nil {
		unit = *req.Unit
	}
	user, _ := middleware.UserFromContext(r.Context())
	out, err := p.queries.CreatePart(r.Context(), sqlc.CreatePartParams{
		Sku:              textFromPtr(req.Sku),
		Name:             req.Name,
		Description:      textFromPtr(req.Description),
		Unit:             unit,
		ReorderLevel:     reorderLevel,
		DefaultCost:      defaultCost,
		DefaultSalePrice: defaultSalePrice,
		CreatedByUserID:  pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if err != nil {
		log.Printf("create part: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	created, err := p.queries.GetPartByUcode(r.Context(), out.Ucode)
	if err != nil {
		log.Printf("refetch created part: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	dto := toPartDTO(created)
	audit.Record(r.Context(), p.queries, r, audit.Entry{
		Action:      "part.create",
		EntityType:  "part",
		EntityID:    &created.ID,
		EntityUcode: &created.Ucode,
		After:       dto,
	})
	writeJSON(w, http.StatusCreated, map[string]partDTO{"part": dto})
}

func (p *Parts) Search(w http.ResponseWriter, r *http.Request) {
	params, countParams, page, pageSize := parsePartSearchParams(r)
	total, err := p.queries.CountParts(r.Context(), countParams)
	if err != nil {
		log.Printf("count parts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	rows, err := p.queries.SearchParts(r.Context(), params)
	if err != nil {
		log.Printf("search parts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]partDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPartDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"parts":     out,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (p *Parts) Get(w http.ResponseWriter, r *http.Request) {
	part, ok := p.partByUcode(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]partDTO{"part": toPartDTO(part)})
}

func (p *Parts) Update(w http.ResponseWriter, r *http.Request) {
	current, ok := p.partByUcode(w, r)
	if !ok {
		return
	}

	var req updatePartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimUpdatePartReq(&req)
	if err := p.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdatePartParams{
		ID:               current.ID,
		Sku:              current.Sku,
		Name:             current.Name,
		Description:      current.Description,
		Unit:             current.Unit,
		ReorderLevel:     current.ReorderLevel,
		DefaultCost:      current.DefaultCost,
		DefaultSalePrice: current.DefaultSalePrice,
	}
	if req.Sku != nil {
		params.Sku = textFromPtr(req.Sku)
	}
	if req.Name != nil {
		params.Name = *req.Name
	}
	if req.Description != nil {
		params.Description = textFromPtr(req.Description)
	}
	if req.Unit != nil {
		params.Unit = *req.Unit
	}
	if req.ReorderLevel != nil {
		params.ReorderLevel, ok = parseOptionalNonNegativeNumeric(w, req.ReorderLevel)
		if !ok {
			return
		}
	}
	if req.DefaultCost != nil {
		params.DefaultCost, ok = parseOptionalNonNegativeNumeric(w, req.DefaultCost)
		if !ok {
			return
		}
	}
	if req.DefaultSalePrice != nil {
		params.DefaultSalePrice, ok = parseOptionalNonNegativeNumeric(w, req.DefaultSalePrice)
		if !ok {
			return
		}
	}

	beforeDTO := toPartDTO(current)
	out, err := p.queries.UpdatePart(r.Context(), params)
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update part: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	afterDTO := toPartDTO(out)
	audit.Record(r.Context(), p.queries, r, audit.Entry{
		Action:      "part.update",
		EntityType:  "part",
		EntityID:    &out.ID,
		EntityUcode: &out.Ucode,
		Before:      beforeDTO,
		After:       afterDTO,
	})
	writeJSON(w, http.StatusOK, map[string]partDTO{"part": afterDTO})
}

func (p *Parts) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	part, err := p.queries.GetPartByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("get part for delete: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := p.queries.SoftDeletePart(r.Context(), sqlc.SoftDeletePartParams{
		ID:             part.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete part: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	audit.Record(r.Context(), p.queries, r, audit.Entry{
		Action:      "part.delete",
		EntityType:  "part",
		EntityID:    &part.ID,
		EntityUcode: &part.Ucode,
		Before:      toPartDTO(part),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (p *Parts) CreateMovement(w http.ResponseWriter, r *http.Request) {
	part, ok := p.partByUcode(w, r)
	if !ok {
		return
	}

	var req createMovementReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimCreateMovementReq(&req)
	if req.MovementType == string(partdomain.MovementUse) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "use_movements_via_work_order_only"})
		return
	}
	if err := p.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	_, quantityRat, ok := parsePositiveMovementQuantity(w, req.Quantity)
	if !ok {
		return
	}
	adjustmentOut := req.AdjustmentOut != nil && *req.AdjustmentOut
	if req.MovementType == string(partdomain.MovementAdjustment) && req.AdjustmentOut == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "adjustment_direction_required"})
		return
	}
	delta, err := partdomain.SignedDelta(partdomain.MovementType(req.MovementType), quantityRat, adjustmentOut)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.MovementType == string(partdomain.MovementAdjustment) && adjustmentOut {
		if err := partdomain.CheckStockSufficient(partdomain.NumericToRat(part.CurrentStock), delta); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "insufficient_stock"})
			return
		}
	}

	unitCost, unitCostRat, ok := parseOptionalMovementCost(w, req.UnitCost)
	if !ok {
		return
	}
	supplierID, ok := p.resolveMovementSupplier(w, r, req.SupplierUcode)
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	params := sqlc.CreatePartMovementParams{
		PartID:          part.ID,
		MovementType:    req.MovementType,
		Quantity:        numericFromRat(delta),
		UnitCost:        unitCost,
		SupplierID:      supplierID,
		Notes:           textFromPtr(req.Notes),
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}

	var movement sqlc.PartMovement
	if req.MovementType == string(partdomain.MovementPurchase) && req.LinkTransaction != nil && *req.LinkTransaction && unitCost.Valid {
		if req.PaymentMethod == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_method_required"})
			return
		}
		movement, err = p.createPurchaseMovementWithTransaction(r, part, params, quantityRat, unitCostRat, supplierID, *req.PaymentMethod)
	} else {
		movement, err = p.queries.CreatePartMovement(r.Context(), params)
	}
	if isCheckViolation(err) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "insufficient_stock"})
		return
	}
	if err != nil {
		log.Printf("create part movement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	row, err := p.queries.GetPartMovementByUcode(r.Context(), movement.Ucode)
	if err != nil {
		log.Printf("refetch created part movement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	dto := movementDTOFrom(row.PartMovement, row.SupplierUcode, row.SupplierName, row.WorkOrderUcode, row.WorkOrderNumber, row.TransactionUcode)
	audit.Record(r.Context(), p.queries, r, audit.Entry{
		Action:      "part.movement",
		EntityType:  "part_movement",
		EntityID:    &row.PartMovement.ID,
		EntityUcode: &row.PartMovement.Ucode,
		After:       dto,
	})
	writeJSON(w, http.StatusCreated, map[string]movementDTO{"movement": dto})
}

func (p *Parts) ListMovements(w http.ResponseWriter, r *http.Request) {
	part, ok := p.partByUcode(w, r)
	if !ok {
		return
	}
	page, pageSize := parseMovementPagination(r)
	total, err := p.queries.CountPartMovements(r.Context(), part.ID)
	if err != nil {
		log.Printf("count part movements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	rows, err := p.queries.ListPartMovements(r.Context(), sqlc.ListPartMovementsParams{
		PartID:     part.ID,
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		log.Printf("list part movements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]movementDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, movementDTOFrom(row.PartMovement, row.SupplierUcode, row.SupplierName, row.WorkOrderUcode, row.WorkOrderNumber, row.TransactionUcode))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"movements": out,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (p *Parts) partByUcode(w http.ResponseWriter, r *http.Request) (sqlc.Part, bool) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return sqlc.Part{}, false
	}
	part, err := p.queries.GetPartByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.Part{}, false
	}
	if err != nil {
		log.Printf("get part: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Part{}, false
	}
	return part, true
}

func (p *Parts) createPurchaseMovementWithTransaction(
	r *http.Request,
	part sqlc.Part,
	movementParams sqlc.CreatePartMovementParams,
	quantity, unitCost *big.Rat,
	supplierID pgtype.Int8,
	paymentMethod string,
) (sqlc.PartMovement, error) {
	tx, err := p.pool.Begin(r.Context())
	if err != nil {
		return sqlc.PartMovement{}, err
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	amount := new(big.Rat).Mul(quantity, unitCost)
	counterpartyType := "none"
	if supplierID.Valid {
		counterpartyType = "supplier"
	}
	description := "Compra de repuesto: " + part.Name
	if movementParams.Notes.Valid {
		description = movementParams.Notes.String
	}
	now := time.Now().UTC()
	txQueries := p.queries.WithTx(tx)
	transaction, err := txQueries.CreateTransaction(r.Context(), sqlc.CreateTransactionParams{
		TransactionType:  "expense",
		Amount:           numericFromRat(amount),
		Currency:         "ARS",
		FxRateToArs:      numericFromRat(big.NewRat(1, 1)),
		TransactionDate:  pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true},
		PaymentMethod:    paymentMethod,
		Category:         "part_purchase",
		CounterpartyType: counterpartyType,
		SupplierID:       supplierID,
		Description:      pgtype.Text{String: description, Valid: true},
		CreatedByUserID:  movementParams.CreatedByUserID,
	})
	if err != nil {
		return sqlc.PartMovement{}, err
	}
	movementParams.TransactionID = pgtype.Int8{Int64: transaction.ID, Valid: true}
	movement, err := txQueries.CreatePartMovement(r.Context(), movementParams)
	if err != nil {
		return sqlc.PartMovement{}, err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return sqlc.PartMovement{}, err
	}
	return movement, nil
}

func (p *Parts) resolveMovementSupplier(w http.ResponseWriter, r *http.Request, raw *string) (pgtype.Int8, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.Int8{}, true
	}
	ucode, err := uuidFromString(*raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_supplier"})
		return pgtype.Int8{}, false
	}
	supplier, err := p.queries.GetSupplierByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_supplier"})
		return pgtype.Int8{}, false
	}
	if err != nil {
		log.Printf("resolve movement supplier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return pgtype.Int8{}, false
	}
	return pgtype.Int8{Int64: supplier.ID, Valid: true}, true
}

func parsePartSearchParams(r *http.Request) (sqlc.SearchPartsParams, sqlc.CountPartsParams, int, int) {
	q := r.URL.Query()
	page := parsePositiveInt(q.Get("page"), 1)
	pageSize := parsePositiveInt(q.Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}
	search := strings.TrimSpace(q.Get("q"))
	lowStock := strings.EqualFold(strings.TrimSpace(q.Get("low_stock")), "true")
	return sqlc.SearchPartsParams{
			Q:          search,
			LowStock:   lowStock,
			PageSize:   int32(pageSize),
			PageOffset: int32((page - 1) * pageSize),
		}, sqlc.CountPartsParams{
			Q:        search,
			LowStock: lowStock,
		}, page, pageSize
}

func toPartDTO(part sqlc.Part) partDTO {
	lowStock := part.ReorderLevel.Valid &&
		partdomain.NumericToRat(part.CurrentStock).Cmp(partdomain.NumericToRat(part.ReorderLevel)) < 0
	return partDTO{
		Ucode:            uuidString(part.Ucode),
		Sku:              stringPtrFromText(part.Sku),
		Name:             part.Name,
		Description:      stringPtrFromText(part.Description),
		Unit:             part.Unit,
		CurrentStock:     money.NumericToString(part.CurrentStock),
		ReorderLevel:     money.NumericToStringPtr(part.ReorderLevel),
		DefaultCost:      money.NumericToStringPtr(part.DefaultCost),
		DefaultSalePrice: money.NumericToStringPtr(part.DefaultSalePrice),
		LowStock:         lowStock,
		CreatedTs:        timeString(part.CreatedTs),
	}
}

func movementDTOFrom(movement sqlc.PartMovement, supplierUcode pgtype.UUID, supplierName pgtype.Text, workOrderUcode pgtype.UUID, workOrderNumber pgtype.Text, transactionUcode pgtype.UUID) movementDTO {
	var transaction *movementTxnRef
	if transactionUcode.Valid {
		transaction = &movementTxnRef{Ucode: uuidString(transactionUcode)}
	}
	return movementDTO{
		Ucode:        uuidString(movement.Ucode),
		MovementType: movement.MovementType,
		Quantity:     money.NumericToString(movement.Quantity),
		UnitCost:     money.NumericToStringPtr(movement.UnitCost),
		Supplier:     counterpartyRefFrom(supplierUcode, supplierName),
		WorkOrder:    workOrderRefFrom(workOrderUcode, workOrderNumber),
		Transaction:  transaction,
		Notes:        stringPtrFromText(movement.Notes),
		CreatedTs:    timeString(movement.CreatedTs),
	}
}

func parseOptionalNonNegativeNumeric(w http.ResponseWriter, raw *string) (pgtype.Numeric, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.Numeric{}, true
	}
	n, err := money.StringToNumeric(*raw)
	if err != nil || !n.Valid || n.Int == nil || n.Int.Sign() < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return pgtype.Numeric{}, false
	}
	return n, true
}

func parsePositiveMovementQuantity(w http.ResponseWriter, raw string) (pgtype.Numeric, *big.Rat, bool) {
	n, err := money.StringToNumeric(raw)
	if err != nil || !n.Valid || n.Int == nil || n.Int.Sign() <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return pgtype.Numeric{}, nil, false
	}
	return n, partdomain.NumericToRat(n), true
}

func parseOptionalMovementCost(w http.ResponseWriter, raw *string) (pgtype.Numeric, *big.Rat, bool) {
	n, ok := parseOptionalNonNegativeNumeric(w, raw)
	if !ok {
		return pgtype.Numeric{}, nil, false
	}
	if !n.Valid {
		return n, nil, true
	}
	return n, partdomain.NumericToRat(n), true
}

func numericFromRat(r *big.Rat) pgtype.Numeric {
	n, err := money.StringToNumeric(r.FloatString(2))
	if err != nil {
		return pgtype.Numeric{}
	}
	return n
}

func parseMovementPagination(r *http.Request) (int, int) {
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func trimCreatePartReq(req *createPartReq) {
	req.Name = strings.TrimSpace(req.Name)
	trimStringPtr(req.Sku)
	trimStringPtr(req.Description)
	trimStringPtr(req.Unit)
	trimStringPtr(req.ReorderLevel)
	trimStringPtr(req.DefaultCost)
	trimStringPtr(req.DefaultSalePrice)
}

func trimUpdatePartReq(req *updatePartReq) {
	trimStringPtr(req.Sku)
	trimStringPtr(req.Name)
	trimStringPtr(req.Description)
	trimStringPtr(req.Unit)
	trimStringPtr(req.ReorderLevel)
	trimStringPtr(req.DefaultCost)
	trimStringPtr(req.DefaultSalePrice)
}

func trimCreateMovementReq(req *createMovementReq) {
	req.MovementType = strings.TrimSpace(req.MovementType)
	req.Quantity = strings.TrimSpace(req.Quantity)
	trimStringPtr(req.UnitCost)
	trimStringPtr(req.PaymentMethod)
	trimStringPtr(req.SupplierUcode)
	trimStringPtr(req.Notes)
}
