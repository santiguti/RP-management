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
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type deviceDTO struct {
	Ucode            string  `json:"ucode"`
	ClientUcode      string  `json:"client_ucode"`
	BrandUcode       string  `json:"brand_ucode"`
	ModelUcode       *string `json:"model_ucode,omitempty"`
	ArticleTypeUcode string  `json:"article_type_ucode"`
	SerialNumber     *string `json:"serial_number,omitempty"`
	Color            *string `json:"color,omitempty"`
	Description      *string `json:"description,omitempty"`
	CreatedTs        string  `json:"created_ts"`
}

type createDeviceReq struct {
	ClientUcode      string  `json:"client_ucode" validate:"required"`
	BrandUcode       string  `json:"brand_ucode" validate:"required"`
	ModelUcode       *string `json:"model_ucode" validate:"omitempty"`
	ArticleTypeUcode string  `json:"article_type_ucode" validate:"required"`
	SerialNumber     *string `json:"serial_number" validate:"omitempty,max=120"`
	Color            *string `json:"color" validate:"omitempty,max=80"`
	Description      *string `json:"description" validate:"omitempty,max=2000"`
}

type updateDeviceReq struct {
	ClientUcode      *string `json:"client_ucode" validate:"omitempty"`
	BrandUcode       *string `json:"brand_ucode" validate:"omitempty"`
	ModelUcode       *string `json:"model_ucode" validate:"omitempty"`
	ArticleTypeUcode *string `json:"article_type_ucode" validate:"omitempty"`
	SerialNumber     *string `json:"serial_number" validate:"omitempty,max=120"`
	Color            *string `json:"color" validate:"omitempty,max=80"`
	Description      *string `json:"description" validate:"omitempty,max=2000"`
}

type Devices struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewDevices(q *sqlc.Queries) *Devices {
	return &Devices{queries: q, val: validator.New()}
}

func (d *Devices) Create(w http.ResponseWriter, r *http.Request) {
	var req createDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimCreateDeviceReq(&req)
	if err := d.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	resolved, ok := d.resolveDeviceRefs(w, r, req.ClientUcode, req.BrandUcode, req.ModelUcode, req.ArticleTypeUcode)
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	out, err := d.queries.CreateDevice(r.Context(), sqlc.CreateDeviceParams{
		ClientID:        resolved.client.ID,
		BrandID:         resolved.brand.ID,
		ModelID:         resolved.modelID,
		ArticleTypeID:   resolved.articleType.ID,
		SerialNumber:    textFromPtr(req.SerialNumber),
		Color:           textFromPtr(req.Color),
		Description:     textFromPtr(req.Description),
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("create device: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	detail, ok := d.deviceDetailByUcode(w, r, out.Ucode)
	if !ok {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]deviceDTO{"device": toDeviceDTOFromGet(detail)})
}

func (d *Devices) Search(w http.ResponseWriter, r *http.Request) {
	clientRaw := strings.TrimSpace(r.URL.Query().Get("client_ucode"))
	serial := strings.TrimSpace(r.URL.Query().Get("serial"))
	if clientRaw == "" && serial == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client_ucode_or_serial_required"})
		return
	}

	params := sqlc.SearchDevicesParams{Serial: serial}
	if clientRaw != "" {
		clientUcode, err := uuidFromString(clientRaw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
			return
		}
		client, err := d.queries.GetClientByUcode(r.Context(), clientUcode)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
			return
		}
		if err != nil {
			log.Printf("resolve search client: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}
		params.HasClient = true
		params.ClientID = client.ID
	}

	rows, err := d.queries.SearchDevices(r.Context(), params)
	if err != nil {
		log.Printf("search devices: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]deviceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDeviceDTO(row.Device, row.ClientUcode, row.BrandUcode, row.ModelUcode, row.ArticleTypeUcode))
	}
	writeJSON(w, http.StatusOK, map[string][]deviceDTO{"devices": out})
}

func (d *Devices) Get(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	detail, ok := d.deviceDetailByUcode(w, r, ucode)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]deviceDTO{"device": toDeviceDTOFromGet(detail)})
}

func (d *Devices) Update(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	detail, ok := d.deviceDetailByUcode(w, r, ucode)
	if !ok {
		return
	}

	var req updateDeviceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	trimUpdateDeviceReq(&req)
	if err := d.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdateDeviceParams{
		ID:            detail.Device.ID,
		ClientID:      detail.Device.ClientID,
		BrandID:       detail.Device.BrandID,
		ModelID:       detail.Device.ModelID,
		ArticleTypeID: detail.Device.ArticleTypeID,
		SerialNumber:  detail.Device.SerialNumber,
		Color:         detail.Device.Color,
		Description:   detail.Device.Description,
	}
	if req.ClientUcode != nil {
		client, ok := d.resolveClient(w, r, *req.ClientUcode)
		if !ok {
			return
		}
		params.ClientID = client.ID
	}
	if req.BrandUcode != nil {
		brand, ok := d.resolveBrand(w, r, *req.BrandUcode)
		if !ok {
			return
		}
		params.BrandID = brand.ID
	}
	if req.ArticleTypeUcode != nil {
		articleType, ok := d.resolveArticleType(w, r, *req.ArticleTypeUcode)
		if !ok {
			return
		}
		params.ArticleTypeID = articleType.ID
	}
	if req.ModelUcode != nil {
		modelID, ok := d.resolveModelID(w, r, req.ModelUcode)
		if !ok {
			return
		}
		params.ModelID = modelID
	}
	if req.SerialNumber != nil {
		params.SerialNumber = textFromPtr(req.SerialNumber)
	}
	if req.Color != nil {
		params.Color = textFromPtr(req.Color)
	}
	if req.Description != nil {
		params.Description = textFromPtr(req.Description)
	}

	out, err := d.queries.UpdateDevice(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update device: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	updated, ok := d.deviceDetailByUcode(w, r, out.Ucode)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]deviceDTO{"device": toDeviceDTOFromGet(updated)})
}

func (d *Devices) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	detail, err := d.queries.GetDeviceByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("get device for delete: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := d.queries.SoftDeleteDevice(r.Context(), sqlc.SoftDeleteDeviceParams{
		ID:             detail.Device.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete device: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type resolvedDeviceRefs struct {
	client      sqlc.Client
	brand       sqlc.Brand
	modelID     pgtype.Int8
	articleType sqlc.ArticleType
}

func (d *Devices) resolveDeviceRefs(w http.ResponseWriter, r *http.Request, clientUcode, brandUcode string, modelUcode *string, articleTypeUcode string) (resolvedDeviceRefs, bool) {
	client, ok := d.resolveClient(w, r, clientUcode)
	if !ok {
		return resolvedDeviceRefs{}, false
	}
	brand, ok := d.resolveBrand(w, r, brandUcode)
	if !ok {
		return resolvedDeviceRefs{}, false
	}
	modelID, ok := d.resolveModelID(w, r, modelUcode)
	if !ok {
		return resolvedDeviceRefs{}, false
	}
	articleType, ok := d.resolveArticleType(w, r, articleTypeUcode)
	if !ok {
		return resolvedDeviceRefs{}, false
	}
	return resolvedDeviceRefs{client: client, brand: brand, modelID: modelID, articleType: articleType}, true
}

func (d *Devices) resolveClient(w http.ResponseWriter, r *http.Request, raw string) (sqlc.Client, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
		return sqlc.Client{}, false
	}
	client, err := d.queries.GetClientByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_client"})
		return sqlc.Client{}, false
	}
	if err != nil {
		log.Printf("resolve client: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Client{}, false
	}
	return client, true
}

func (d *Devices) resolveBrand(w http.ResponseWriter, r *http.Request, raw string) (sqlc.Brand, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_brand"})
		return sqlc.Brand{}, false
	}
	brand, err := d.queries.GetBrandByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_brand"})
		return sqlc.Brand{}, false
	}
	if err != nil {
		log.Printf("resolve brand: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Brand{}, false
	}
	return brand, true
}

func (d *Devices) resolveModelID(w http.ResponseWriter, r *http.Request, raw *string) (pgtype.Int8, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.Int8{}, true
	}
	ucode, err := uuidFromString(*raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_model"})
		return pgtype.Int8{}, false
	}
	row, err := d.queries.GetDeviceModelByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_model"})
		return pgtype.Int8{}, false
	}
	if err != nil {
		log.Printf("resolve model: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return pgtype.Int8{}, false
	}
	return pgtype.Int8{Int64: row.DeviceModel.ID, Valid: true}, true
}

func (d *Devices) resolveArticleType(w http.ResponseWriter, r *http.Request, raw string) (sqlc.ArticleType, bool) {
	ucode, err := uuidFromString(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_article_type"})
		return sqlc.ArticleType{}, false
	}
	articleType, err := d.queries.GetArticleTypeByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_article_type"})
		return sqlc.ArticleType{}, false
	}
	if err != nil {
		log.Printf("resolve article type: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.ArticleType{}, false
	}
	return articleType, true
}

func (d *Devices) deviceDetailByUcode(w http.ResponseWriter, r *http.Request, ucode pgtype.UUID) (sqlc.GetDeviceByUcodeRow, bool) {
	row, err := d.queries.GetDeviceByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.GetDeviceByUcodeRow{}, false
	}
	if err != nil {
		log.Printf("get device: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.GetDeviceByUcodeRow{}, false
	}
	return row, true
}

func toDeviceDTOFromGet(row sqlc.GetDeviceByUcodeRow) deviceDTO {
	return toDeviceDTO(row.Device, row.ClientUcode, row.BrandUcode, row.ModelUcode, row.ArticleTypeUcode)
}

func toDeviceDTO(d sqlc.Device, clientUcode, brandUcode, modelUcode, articleTypeUcode pgtype.UUID) deviceDTO {
	return deviceDTO{
		Ucode:            uuidString(d.Ucode),
		ClientUcode:      uuidString(clientUcode),
		BrandUcode:       uuidString(brandUcode),
		ModelUcode:       stringPtrFromUUID(modelUcode),
		ArticleTypeUcode: uuidString(articleTypeUcode),
		SerialNumber:     stringPtrFromText(d.SerialNumber),
		Color:            stringPtrFromText(d.Color),
		Description:      stringPtrFromText(d.Description),
		CreatedTs:        d.CreatedTs.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func stringPtrFromUUID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	out := uuidString(value)
	return &out
}

func uuidFromString(raw string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(strings.TrimSpace(raw)); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func trimCreateDeviceReq(req *createDeviceReq) {
	req.ClientUcode = strings.TrimSpace(req.ClientUcode)
	req.BrandUcode = strings.TrimSpace(req.BrandUcode)
	req.ArticleTypeUcode = strings.TrimSpace(req.ArticleTypeUcode)
	trimStringPtr(req.ModelUcode)
	trimStringPtr(req.SerialNumber)
	trimStringPtr(req.Color)
	trimStringPtr(req.Description)
}

func trimUpdateDeviceReq(req *updateDeviceReq) {
	trimStringPtr(req.ClientUcode)
	trimStringPtr(req.BrandUcode)
	trimStringPtr(req.ModelUcode)
	trimStringPtr(req.ArticleTypeUcode)
	trimStringPtr(req.SerialNumber)
	trimStringPtr(req.Color)
	trimStringPtr(req.Description)
}

func trimStringPtr(value *string) {
	if value == nil {
		return
	}
	*value = strings.TrimSpace(*value)
}
