package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

type Parts struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewParts(q *sqlc.Queries) *Parts {
	return &Parts{queries: q, val: validator.New()}
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
	writeJSON(w, http.StatusCreated, map[string]partDTO{"part": toPartDTO(created)})
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
	writeJSON(w, http.StatusOK, map[string]partDTO{"part": toPartDTO(out)})
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
	w.WriteHeader(http.StatusNoContent)
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
		CurrentStock:     partNumericToString(part.CurrentStock),
		ReorderLevel:     partNumericToStringPtr(part.ReorderLevel),
		DefaultCost:      partNumericToStringPtr(part.DefaultCost),
		DefaultSalePrice: partNumericToStringPtr(part.DefaultSalePrice),
		LowStock:         lowStock,
		CreatedTs:        timeString(part.CreatedTs),
	}
}

func partNumericToString(n pgtype.Numeric) string {
	return partdomain.NumericToRat(n).FloatString(2)
}

func partNumericToStringPtr(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	out := partNumericToString(n)
	return &out
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
