package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	clientdomain "github.com/santiguti/rp-management/backend/internal/domain/clients"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type clientDTO struct {
	Ucode      string  `json:"ucode"`
	Name       string  `json:"name"`
	Phone      *string `json:"phone,omitempty"`
	Email      *string `json:"email,omitempty"`
	DniCuit    *string `json:"dni_cuit,omitempty"`
	Address    *string `json:"address,omitempty"`
	Notes      *string `json:"notes,omitempty"`
	ClientType string  `json:"client_type"`
	CreatedTs  string  `json:"created_ts"`
}

type createClientReq struct {
	Name       string  `json:"name" validate:"required,min=1,max=200"`
	Phone      *string `json:"phone" validate:"omitempty,min=3,max=32"`
	Email      *string `json:"email" validate:"omitempty,email,max=200"`
	DniCuit    *string `json:"dni_cuit" validate:"omitempty,max=32"`
	Address    *string `json:"address" validate:"omitempty,max=400"`
	Notes      *string `json:"notes" validate:"omitempty,max=2000"`
	ClientType *string `json:"client_type" validate:"omitempty,oneof=particular empresa"`
}

type updateClientReq struct {
	Name       *string `json:"name" validate:"omitempty,min=1,max=200"`
	Phone      *string `json:"phone" validate:"omitempty,min=3,max=32"`
	Email      *string `json:"email" validate:"omitempty,email,max=200"`
	DniCuit    *string `json:"dni_cuit" validate:"omitempty,max=32"`
	Address    *string `json:"address" validate:"omitempty,max=400"`
	Notes      *string `json:"notes" validate:"omitempty,max=2000"`
	ClientType *string `json:"client_type" validate:"omitempty,oneof=particular empresa"`
}

type Clients struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewClients(q *sqlc.Queries) *Clients {
	return &Clients{queries: q, val: validator.New()}
}

func (c *Clients) Create(w http.ResponseWriter, r *http.Request) {
	var req createClientReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := c.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	phone, ok := normalizePhone(w, req.Phone)
	if !ok {
		return
	}
	if phone.Valid && c.phoneBelongsToOtherClient(r, phone, 0) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "phone_already_exists"})
		return
	}

	clientType := "particular"
	if req.ClientType != nil {
		clientType = strings.TrimSpace(*req.ClientType)
	}
	user, _ := middleware.UserFromContext(r.Context())

	out, err := c.queries.CreateClient(r.Context(), sqlc.CreateClientParams{
		Name:            req.Name,
		Phone:           phone,
		Email:           textFromPtr(req.Email),
		DniCuit:         textFromPtr(req.DniCuit),
		Address:         textFromPtr(req.Address),
		Notes:           textFromPtr(req.Notes),
		ClientType:      clientType,
		CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("create client: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]clientDTO{"client": toClientDTO(out)})
}

func (c *Clients) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	total, err := c.queries.CountClients(r.Context(), q)
	if err != nil {
		log.Printf("count clients: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	rows, err := c.queries.SearchClients(r.Context(), sqlc.SearchClientsParams{
		Q:          q,
		PageSize:   int32(pageSize),
		PageOffset: int32(offset),
	})
	if err != nil {
		log.Printf("search clients: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	out := make([]clientDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toClientDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"clients":   out,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (c *Clients) Get(w http.ResponseWriter, r *http.Request) {
	client, ok := c.clientByUcode(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]clientDTO{"client": toClientDTO(client)})
}

func (c *Clients) Update(w http.ResponseWriter, r *http.Request) {
	client, ok := c.clientByUcode(w, r)
	if !ok {
		return
	}

	var req updateClientReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
	}
	if err := c.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	params := sqlc.UpdateClientParams{
		ID:         client.ID,
		Name:       client.Name,
		Phone:      client.Phone,
		Email:      client.Email,
		DniCuit:    client.DniCuit,
		Address:    client.Address,
		Notes:      client.Notes,
		ClientType: client.ClientType,
	}
	if req.Name != nil {
		params.Name = *req.Name
	}
	if req.Phone != nil {
		phone, ok := normalizePhone(w, req.Phone)
		if !ok {
			return
		}
		if phone.Valid && c.phoneBelongsToOtherClient(r, phone, client.ID) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "phone_already_exists"})
			return
		}
		params.Phone = phone
	}
	if req.Email != nil {
		params.Email = textFromPtr(req.Email)
	}
	if req.DniCuit != nil {
		params.DniCuit = textFromPtr(req.DniCuit)
	}
	if req.Address != nil {
		params.Address = textFromPtr(req.Address)
	}
	if req.Notes != nil {
		params.Notes = textFromPtr(req.Notes)
	}
	if req.ClientType != nil {
		params.ClientType = strings.TrimSpace(*req.ClientType)
	}

	out, err := c.queries.UpdateClient(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err != nil {
		log.Printf("update client: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]clientDTO{"client": toClientDTO(out)})
}

func (c *Clients) Delete(w http.ResponseWriter, r *http.Request) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return
	}
	client, err := c.queries.GetClientByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		log.Printf("get client for delete: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	user, _ := middleware.UserFromContext(r.Context())
	if err := c.queries.SoftDeleteClient(r.Context(), sqlc.SoftDeleteClientParams{
		ID:             client.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete client: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Clients) ListDevices(w http.ResponseWriter, r *http.Request) {
	client, ok := c.clientByUcode(w, r)
	if !ok {
		return
	}
	rows, err := c.queries.ListClientDevices(r.Context(), client.ID)
	if err != nil {
		log.Printf("list client devices: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]deviceDTO, 0, len(rows))
	for _, row := range rows {
		detail, err := c.queries.GetDeviceByUcode(r.Context(), row.Ucode)
		if err != nil {
			log.Printf("get client device detail: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}
		out = append(out, toDeviceDTO(detail.Device, detail.ClientUcode, detail.BrandUcode, detail.ModelUcode, detail.ArticleTypeUcode))
	}
	writeJSON(w, http.StatusOK, map[string][]deviceDTO{"devices": out})
}

func (c *Clients) clientByUcode(w http.ResponseWriter, r *http.Request) (sqlc.Client, bool) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return sqlc.Client{}, false
	}
	client, err := c.queries.GetClientByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.Client{}, false
	}
	if err != nil {
		log.Printf("get client: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Client{}, false
	}
	return client, true
}

func (c *Clients) phoneBelongsToOtherClient(r *http.Request, phone pgtype.Text, clientID int64) bool {
	existing, err := c.queries.GetClientByPhone(r.Context(), phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		log.Printf("get client by phone: %v", err)
		return false
	}
	return existing.ID != clientID
}

func normalizePhone(w http.ResponseWriter, raw *string) (pgtype.Text, bool) {
	if raw == nil {
		return pgtype.Text{}, true
	}
	normalized, err := clientdomain.NormalizeE164(*raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_phone"})
		return pgtype.Text{}, false
	}
	return pgtype.Text{String: normalized, Valid: true}, true
}

func parseUcode(w http.ResponseWriter, raw string) (pgtype.UUID, bool) {
	var id pgtype.UUID
	if err := id.Scan(raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_ucode"})
		return pgtype.UUID{}, false
	}
	return id, true
}

func toClientDTO(c sqlc.Client) clientDTO {
	return clientDTO{
		Ucode:      uuidString(c.Ucode),
		Name:       c.Name,
		Phone:      stringPtrFromText(c.Phone),
		Email:      stringPtrFromText(c.Email),
		DniCuit:    stringPtrFromText(c.DniCuit),
		Address:    stringPtrFromText(c.Address),
		Notes:      stringPtrFromText(c.Notes),
		ClientType: c.ClientType,
		CreatedTs:  c.CreatedTs.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func uuidString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func textFromPtr(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	trimmed := strings.TrimSpace(*value)
	return pgtype.Text{String: trimmed, Valid: trimmed != ""}
}

func stringPtrFromText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
