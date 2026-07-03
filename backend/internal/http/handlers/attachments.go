package handlers

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/audit"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
	"github.com/santiguti/rp-management/backend/internal/storage"
)

type attachmentDTO struct {
	Ucode            string `json:"ucode"`
	Phase            string `json:"phase"`
	OriginalFilename string `json:"original_filename"`
	MimeType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes"`
	Width            *int32 `json:"width,omitempty"`
	Height           *int32 `json:"height,omitempty"`
	CreatedTs        string `json:"created_ts"`
}

type Attachments struct {
	queries *sqlc.Queries
	store   *storage.FileStore
}

func NewAttachments(q *sqlc.Queries, store *storage.FileStore) *Attachments {
	return &Attachments{queries: q, store: store}
}

func (a *Attachments) Upload(w http.ResponseWriter, r *http.Request) {
	workOrder, ok := a.workOrderByUcode(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, storage.MaxUploadBytes+(2<<20))
	if err := r.ParseMultipartForm(storage.MaxUploadBytes + (2 << 20)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	defer r.MultipartForm.RemoveAll()
	phase := strings.TrimSpace(r.FormValue("phase"))
	if !isAttachmentPhase(phase) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_phase"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	defer file.Close()
	filename := filepath.Base(strings.TrimSpace(header.Filename))
	if filename == "" || filename == "." {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	relPath, mimeType, size, err := a.store.Save("wo-"+uuidString(workOrder.WorkOrder.Ucode), file)
	if errors.Is(err, storage.ErrUnsupportedMime) {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "unsupported_mime"})
		return
	}
	if errors.Is(err, storage.ErrTooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file_too_large"})
		return
	}
	if err != nil {
		log.Printf("save attachment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	width, height := a.imageDimensions(relPath)
	user, _ := middleware.UserFromContext(r.Context())
	attachment, err := a.queries.CreateAttachment(r.Context(), sqlc.CreateAttachmentParams{
		WorkOrderID:      workOrder.WorkOrder.ID,
		Phase:            phase,
		OriginalFilename: filename,
		MimeType:         mimeType,
		SizeBytes:        size,
		StoragePath:      relPath,
		Width:            width,
		Height:           height,
		UploadedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("create attachment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	dto := toAttachmentDTO(attachment)
	audit.Record(r.Context(), a.queries, r, audit.Entry{
		Action:      "attachment.upload",
		EntityType:  "attachment",
		EntityID:    &attachment.ID,
		EntityUcode: &attachment.Ucode,
		After:       dto,
	})
	writeJSON(w, http.StatusCreated, map[string]attachmentDTO{"attachment": dto})
}

func (a *Attachments) List(w http.ResponseWriter, r *http.Request) {
	workOrder, ok := a.workOrderByUcode(w, r)
	if !ok {
		return
	}
	rows, err := a.queries.ListWorkOrderAttachments(r.Context(), workOrder.WorkOrder.ID)
	if err != nil {
		log.Printf("list attachments: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]attachmentDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toAttachmentDTO(row))
	}
	writeJSON(w, http.StatusOK, map[string][]attachmentDTO{"attachments": out})
}

func (a *Attachments) Download(w http.ResponseWriter, r *http.Request) {
	workOrder, ok := a.workOrderByUcode(w, r)
	if !ok {
		return
	}
	attachment, ok := a.attachmentForWorkOrder(w, r, workOrder.WorkOrder.ID)
	if !ok {
		return
	}
	f, err := a.store.Open(attachment.StoragePath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(w, f); err != nil {
		log.Printf("stream attachment: %v", err)
	}
}

func (a *Attachments) Delete(w http.ResponseWriter, r *http.Request) {
	workOrder, ok := a.workOrderByUcode(w, r)
	if !ok {
		return
	}
	attachment, ok := a.attachmentForWorkOrder(w, r, workOrder.WorkOrder.ID)
	if !ok {
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := a.queries.SoftDeleteAttachment(r.Context(), sqlc.SoftDeleteAttachmentParams{
		ID:             attachment.ID,
		VoidedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
	}); err != nil {
		log.Printf("delete attachment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	audit.Record(r.Context(), a.queries, r, audit.Entry{
		Action:      "attachment.delete",
		EntityType:  "attachment",
		EntityID:    &attachment.ID,
		EntityUcode: &attachment.Ucode,
		Before:      toAttachmentDTO(attachment),
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Attachments) workOrderByUcode(w http.ResponseWriter, r *http.Request) (sqlc.GetWorkOrderByUcodeRow, bool) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "ucode"))
	if !ok {
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	workOrder, err := a.queries.GetWorkOrderByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	if err != nil {
		log.Printf("get work order for attachment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.GetWorkOrderByUcodeRow{}, false
	}
	return workOrder, true
}

func (a *Attachments) attachmentForWorkOrder(w http.ResponseWriter, r *http.Request, workOrderID int64) (sqlc.Attachment, bool) {
	ucode, ok := parseUcode(w, chi.URLParam(r, "att_ucode"))
	if !ok {
		return sqlc.Attachment{}, false
	}
	attachment, err := a.queries.GetAttachmentByUcode(r.Context(), ucode)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && attachment.WorkOrderID != workOrderID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return sqlc.Attachment{}, false
	}
	if err != nil {
		log.Printf("get attachment: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return sqlc.Attachment{}, false
	}
	return attachment, true
}

func (a *Attachments) imageDimensions(relPath string) (pgtype.Int4, pgtype.Int4) {
	f, err := a.store.Open(relPath)
	if err != nil {
		return pgtype.Int4{}, pgtype.Int4{}
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return pgtype.Int4{}, pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(config.Width), Valid: true}, pgtype.Int4{Int32: int32(config.Height), Valid: true}
}

func toAttachmentDTO(attachment sqlc.Attachment) attachmentDTO {
	return attachmentDTO{
		Ucode:            uuidString(attachment.Ucode),
		Phase:            attachment.Phase,
		OriginalFilename: attachment.OriginalFilename,
		MimeType:         attachment.MimeType,
		SizeBytes:        attachment.SizeBytes,
		Width:            int32Ptr(attachment.Width),
		Height:           int32Ptr(attachment.Height),
		CreatedTs:        timeString(attachment.CreatedTs),
	}
}

func int32Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
}

func isAttachmentPhase(phase string) bool {
	switch phase {
	case "intake", "diagnosis", "during_repair", "delivery":
		return true
	default:
		return false
	}
}
