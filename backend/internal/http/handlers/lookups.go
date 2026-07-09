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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type lookupDTO struct {
	Ucode string `json:"ucode"`
	Name  string `json:"name"`
}

type deviceModelDTO struct {
	Ucode      string `json:"ucode"`
	BrandUcode string `json:"brand_ucode"`
	Name       string `json:"name"`
}

type lookupNameReq struct {
	Name string `json:"name" validate:"required,min=1,max=80"`
}

type createDeviceModelReq struct {
	BrandUcode string `json:"brand_ucode" validate:"required"`
	Name       string `json:"name" validate:"required,min=1,max=80"`
}

type Brands struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewBrands(q *sqlc.Queries) *Brands {
	return &Brands{queries: q, val: validator.New()}
}

func (b *Brands) List(w http.ResponseWriter, r *http.Request) {
	rows, err := b.queries.ListBrands(r.Context())
	if err != nil {
		log.Printf("list brands: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]lookupDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBrandDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string][]lookupDTO{"brands": out})
}

func (b *Brands) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeLookupNameReq(w, r, b.val)
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	out, err := b.queries.CreateBrand(r.Context(), sqlc.CreateBrandParams{
		Name:            req.Name,
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if err != nil {
		log.Printf("create brand: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]lookupDTO{"brand": toBrandDTO(out)})
}

func (b *Brands) Update(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	req, ok := decodeLookupNameReq(w, r, b.val)
	if !ok {
		return
	}
	out, err := b.queries.UpdateBrand(r.Context(), sqlc.UpdateBrandParams{
		Ucode: ucode,
		Name:  req.Name,
	})
	writeLookupUpdateResult(w, "update brand", err, map[string]lookupDTO{"brand": toBrandDTO(out)})
}

func (b *Brands) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := b.queries.SoftDeleteBrand(r.Context(), sqlc.SoftDeleteBrandParams{
		Ucode:          ucode,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete brand: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type DeviceModels struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewDeviceModels(q *sqlc.Queries) *DeviceModels {
	return &DeviceModels{queries: q, val: validator.New()}
}

func (d *DeviceModels) ListByBrand(w http.ResponseWriter, r *http.Request) {
	brand, ok := d.brandFromRoute(w, r)
	if !ok {
		return
	}
	rows, err := d.queries.ListDeviceModelsByBrand(r.Context(), brand.ID)
	if err != nil {
		log.Printf("list device models: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]deviceModelDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDeviceModelDTO(row.DeviceModel, row.BrandUcode))
	}
	writeJSON(w, http.StatusOK, map[string][]deviceModelDTO{"device_models": out})
}

func (d *DeviceModels) Create(w http.ResponseWriter, r *http.Request) {
	var req createDeviceModelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.BrandUcode = strings.TrimSpace(req.BrandUcode)
	if err := d.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	brandUcode, ok := parseUcode(w, req.BrandUcode)
	if !ok {
		return
	}
	brand, err := d.queries.GetBrandByUcode(r.Context(), brandUcode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("get brand for device model: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	user, _ := middleware.UserFromContext(r.Context())
	out, err := d.queries.CreateDeviceModel(r.Context(), sqlc.CreateDeviceModelParams{
		BrandID:         brand.ID,
		Name:            req.Name,
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if err != nil {
		log.Printf("create device model: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]deviceModelDTO{
		"device_model": toDeviceModelDTO(out, brand.Ucode),
	})
}

func (d *DeviceModels) Update(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	req, ok := decodeLookupNameReq(w, r, d.val)
	if !ok {
		return
	}
	out, err := d.queries.UpdateDeviceModel(r.Context(), sqlc.UpdateDeviceModelParams{
		Ucode: ucode,
		Name:  req.Name,
	})
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update device model: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	row, err := d.queries.GetDeviceModelByUcode(r.Context(), out.Ucode)
	if err != nil {
		log.Printf("get updated device model: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]deviceModelDTO{
		"device_model": toDeviceModelDTO(row.DeviceModel, row.BrandUcode),
	})
}

func (d *DeviceModels) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := d.queries.SoftDeleteDeviceModel(r.Context(), sqlc.SoftDeleteDeviceModelParams{
		Ucode:          ucode,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete device model: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *DeviceModels) brandFromRoute(w http.ResponseWriter, r *http.Request) (sqlc.Brand, bool) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return sqlc.Brand{}, false
	}
	brand, err := d.queries.GetBrandByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.Brand{}, false
	}
	if err != nil {
		log.Printf("get brand: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Brand{}, false
	}
	return brand, true
}

type ArticleTypes struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewArticleTypes(q *sqlc.Queries) *ArticleTypes {
	return &ArticleTypes{queries: q, val: validator.New()}
}

func (a *ArticleTypes) List(w http.ResponseWriter, r *http.Request) {
	rows, err := a.queries.ListArticleTypes(r.Context())
	if err != nil {
		log.Printf("list article types: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]lookupDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toArticleTypeDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string][]lookupDTO{"article_types": out})
}

func (a *ArticleTypes) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeLookupNameReq(w, r, a.val)
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	out, err := a.queries.CreateArticleType(r.Context(), sqlc.CreateArticleTypeParams{
		Name:            req.Name,
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if err != nil {
		log.Printf("create article type: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]lookupDTO{"article_type": toArticleTypeDTO(out)})
}

func (a *ArticleTypes) Update(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	req, ok := decodeLookupNameReq(w, r, a.val)
	if !ok {
		return
	}
	out, err := a.queries.UpdateArticleType(r.Context(), sqlc.UpdateArticleTypeParams{
		Ucode: ucode,
		Name:  req.Name,
	})
	writeLookupUpdateResult(w, "update article type", err, map[string]lookupDTO{"article_type": toArticleTypeDTO(out)})
}

func (a *ArticleTypes) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := a.queries.SoftDeleteArticleType(r.Context(), sqlc.SoftDeleteArticleTypeParams{
		Ucode:          ucode,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete article type: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeLookupNameReq(w http.ResponseWriter, r *http.Request, val *validator.Validate) (lookupNameReq, bool) {
	var req lookupNameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return lookupNameReq{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return lookupNameReq{}, false
	}
	return req, true
}

func writeLookupUpdateResult(w http.ResponseWriter, logPrefix string, err error, body any) {
	if isUniqueViolation(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already_exists"})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("%s: %v", logPrefix, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isCheckViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

func toBrandDTO(b sqlc.Brand) lookupDTO {
	return lookupDTO{Ucode: uuidString(b.Ucode), Name: b.Name}
}

func toArticleTypeDTO(a sqlc.ArticleType) lookupDTO {
	return lookupDTO{Ucode: uuidString(a.Ucode), Name: a.Name}
}

func toDeviceModelDTO(m sqlc.DeviceModel, brandUcode pgtype.UUID) deviceModelDTO {
	return deviceModelDTO{
		Ucode:      uuidString(m.Ucode),
		BrandUcode: uuidString(brandUcode),
		Name:       m.Name,
	}
}
