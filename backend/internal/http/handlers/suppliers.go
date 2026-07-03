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
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type supplierDTO struct {
	Ucode     string  `json:"ucode"`
	Name      string  `json:"name"`
	Phone     *string `json:"phone,omitempty"`
	Email     *string `json:"email,omitempty"`
	Notes     *string `json:"notes,omitempty"`
	CreatedTs string  `json:"created_ts"`
}

type createSupplierReq struct {
	Name  string  `json:"name" validate:"required,min=1,max=200"`
	Phone *string `json:"phone" validate:"omitempty,min=3,max=32"`
	Email *string `json:"email" validate:"omitempty,email,max=200"`
	Notes *string `json:"notes" validate:"omitempty,max=2000"`
}

type updateSupplierReq struct {
	Name  *string `json:"name" validate:"omitempty,min=1,max=200"`
	Phone *string `json:"phone" validate:"omitempty,min=3,max=32"`
	Email *string `json:"email" validate:"omitempty,email,max=200"`
	Notes *string `json:"notes" validate:"omitempty,max=2000"`
}

type Suppliers struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewSuppliers(q *sqlc.Queries) *Suppliers {
	return &Suppliers{queries: q, val: validator.New()}
}

func (s *Suppliers) Create(w http.ResponseWriter, r *http.Request) {
	var req createSupplierReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := s.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	user, _ := middleware.UserFromContext(r.Context())
	out, err := s.queries.CreateSupplier(r.Context(), sqlc.CreateSupplierParams{
		Name:            req.Name,
		Phone:           textFromPtr(req.Phone),
		Email:           textFromPtr(req.Email),
		Notes:           textFromPtr(req.Notes),
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if err != nil {
		log.Printf("create supplier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	dto := toSupplierDTO(out)
	audit.Record(r.Context(), s.queries, r, audit.Entry{
		Action:      "supplier.create",
		EntityType:  "supplier",
		EntityID:    &out.ID,
		EntityUcode: &out.Ucode,
		After:       dto,
	})
	writeJSON(w, http.StatusCreated, map[string]supplierDTO{"supplier": dto})
}

func (s *Suppliers) List(w http.ResponseWriter, r *http.Request) {
	rows, err := s.queries.ListSuppliers(r.Context())
	if err != nil {
		log.Printf("list suppliers: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]supplierDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSupplierDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string][]supplierDTO{"suppliers": out})
}

func (s *Suppliers) Get(w http.ResponseWriter, r *http.Request) {
	supplier, ok := s.supplierByUcode(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]supplierDTO{"supplier": toSupplierDTO(supplier)})
}

func (s *Suppliers) Update(w http.ResponseWriter, r *http.Request) {
	supplier, ok := s.supplierByUcode(w, r)
	if !ok {
		return
	}

	var req updateSupplierReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
	}
	if err := s.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdateSupplierParams{
		ID:    supplier.ID,
		Name:  supplier.Name,
		Phone: supplier.Phone,
		Email: supplier.Email,
		Notes: supplier.Notes,
	}
	if req.Name != nil {
		params.Name = *req.Name
	}
	if req.Phone != nil {
		params.Phone = textFromPtr(req.Phone)
	}
	if req.Email != nil {
		params.Email = textFromPtr(req.Email)
	}
	if req.Notes != nil {
		params.Notes = textFromPtr(req.Notes)
	}

	beforeDTO := toSupplierDTO(supplier)
	out, err := s.queries.UpdateSupplier(r.Context(), params)
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update supplier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	afterDTO := toSupplierDTO(out)
	audit.Record(r.Context(), s.queries, r, audit.Entry{
		Action:      "supplier.update",
		EntityType:  "supplier",
		EntityID:    &out.ID,
		EntityUcode: &out.Ucode,
		Before:      beforeDTO,
		After:       afterDTO,
	})
	writeJSON(w, http.StatusOK, map[string]supplierDTO{"supplier": afterDTO})
}

func (s *Suppliers) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	supplier, err := s.queries.GetSupplierByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("get supplier for delete: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	user, _ := middleware.UserFromContext(r.Context())
	if err := s.queries.SoftDeleteSupplier(r.Context(), sqlc.SoftDeleteSupplierParams{
		ID:             supplier.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete supplier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	audit.Record(r.Context(), s.queries, r, audit.Entry{
		Action:      "supplier.delete",
		EntityType:  "supplier",
		EntityID:    &supplier.ID,
		EntityUcode: &supplier.Ucode,
		Before:      toSupplierDTO(supplier),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Suppliers) supplierByUcode(w http.ResponseWriter, r *http.Request) (sqlc.Supplier, bool) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return sqlc.Supplier{}, false
	}
	supplier, err := s.queries.GetSupplierByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.Supplier{}, false
	}
	if err != nil {
		log.Printf("get supplier: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Supplier{}, false
	}
	return supplier, true
}

func toSupplierDTO(s sqlc.Supplier) supplierDTO {
	return supplierDTO{
		Ucode:     uuidString(s.Ucode),
		Name:      s.Name,
		Phone:     stringPtrFromText(s.Phone),
		Email:     stringPtrFromText(s.Email),
		Notes:     stringPtrFromText(s.Notes),
		CreatedTs: s.CreatedTs.Time.Format(time.RFC3339),
	}
}
