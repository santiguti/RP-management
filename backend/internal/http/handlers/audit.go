package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

type auditEntryDTO struct {
	Ucode         string  `json:"ucode"`
	CreatedTs     string  `json:"created_ts"`
	Action        string  `json:"action"`
	EntityType    string  `json:"entity_type"`
	EntityUcode   *string `json:"entity_ucode,omitempty"`
	ActorUsername *string `json:"actor_username,omitempty"`
	ActorFullName *string `json:"actor_full_name,omitempty"`
	Before        any     `json:"before,omitempty"`
	After         any     `json:"after,omitempty"`
	IP            *string `json:"ip,omitempty"`
	UserAgent     *string `json:"user_agent,omitempty"`
}

type Audit struct {
	queries *sqlc.Queries
}

func NewAudit(q *sqlc.Queries) *Audit {
	return &Audit{queries: q}
}

func (a *Audit) List(w http.ResponseWriter, r *http.Request) {
	params, countParams, page, pageSize, ok := a.parseListParams(w, r)
	if !ok {
		return
	}

	total, err := a.queries.CountAuditEntries(r.Context(), countParams)
	if err != nil {
		log.Printf("count audit entries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	rows, err := a.queries.ListAuditEntries(r.Context(), params)
	if err != nil {
		log.Printf("list audit entries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	entries := make([]auditEntryDTO, 0, len(rows))
	for _, row := range rows {
		dto, err := toAuditEntryDTO(row)
		if err != nil {
			log.Printf("decode audit entry: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}
		entries = append(entries, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":   entries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (a *Audit) parseListParams(w http.ResponseWriter, r *http.Request) (sqlc.ListAuditEntriesParams, sqlc.CountAuditEntriesParams, int, int, bool) {
	q := r.URL.Query()
	page := parsePositiveInt(q.Get("page"), 1)
	pageSize := parsePositiveInt(q.Get("page_size"), 25)
	if pageSize > 100 {
		pageSize = 100
	}

	params := sqlc.ListAuditEntriesParams{
		EntityType: strings.TrimSpace(q.Get("entity_type")),
		Action:     strings.TrimSpace(q.Get("action")),
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	}
	if params.EntityType != "" && !isKnownAuditEntityType(params.EntityType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
		return sqlc.ListAuditEntriesParams{}, sqlc.CountAuditEntriesParams{}, 0, 0, false
	}

	if raw := strings.TrimSpace(q.Get("actor")); raw != "" {
		user, err := a.queries.GetUserByUsername(r.Context(), raw)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
			return sqlc.ListAuditEntriesParams{}, sqlc.CountAuditEntriesParams{}, 0, 0, false
		}
		if err != nil {
			log.Printf("resolve audit actor: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return sqlc.ListAuditEntriesParams{}, sqlc.CountAuditEntriesParams{}, 0, 0, false
		}
		params.HasActor = true
		params.ActorUserID = user.ID
	}
	if raw := strings.TrimSpace(q.Get("entity_ucode")); raw != "" {
		ucode, err := uuidFromString(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
			return sqlc.ListAuditEntriesParams{}, sqlc.CountAuditEntriesParams{}, 0, 0, false
		}
		params.HasEntityUcode = true
		params.EntityUcode = ucode
	}
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		from, ok := parseAuditTime(w, raw)
		if !ok {
			return sqlc.ListAuditEntriesParams{}, sqlc.CountAuditEntriesParams{}, 0, 0, false
		}
		params.HasFrom = true
		params.DateFrom = from
	}
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		to, ok := parseAuditTime(w, raw)
		if !ok {
			return sqlc.ListAuditEntriesParams{}, sqlc.CountAuditEntriesParams{}, 0, 0, false
		}
		params.HasTo = true
		params.DateTo = to
	}

	countParams := sqlc.CountAuditEntriesParams{
		HasActor:       params.HasActor,
		ActorUserID:    params.ActorUserID,
		EntityType:     params.EntityType,
		HasEntityUcode: params.HasEntityUcode,
		EntityUcode:    params.EntityUcode,
		Action:         params.Action,
		HasFrom:        params.HasFrom,
		DateFrom:       params.DateFrom,
		HasTo:          params.HasTo,
		DateTo:         params.DateTo,
	}
	return params, countParams, page, pageSize, true
}

func toAuditEntryDTO(row sqlc.ListAuditEntriesRow) (auditEntryDTO, error) {
	entry := row.AuditLog
	before, err := decodeAuditJSON(entry.BeforeJson)
	if err != nil {
		return auditEntryDTO{}, err
	}
	after, err := decodeAuditJSON(entry.AfterJson)
	if err != nil {
		return auditEntryDTO{}, err
	}
	return auditEntryDTO{
		Ucode:         uuidString(entry.Ucode),
		CreatedTs:     entry.CreatedTs.Time.Format(time.RFC3339),
		Action:        entry.Action,
		EntityType:    entry.EntityType,
		EntityUcode:   uuidPtrFromUUID(entry.EntityUcode),
		ActorUsername: stringPtrFromText(row.ActorUsername),
		ActorFullName: stringPtrFromText(row.ActorFullName),
		Before:        before,
		After:         after,
		IP:            stringPtrFromText(entry.Ip),
		UserAgent:     stringPtrFromText(entry.UserAgent),
	}, nil
}

func decodeAuditJSON(raw []byte) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseAuditTime(w http.ResponseWriter, raw string) (pgtype.Timestamptz, bool) {
	layouts := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return pgtype.Timestamptz{Time: parsed, Valid: true}, true
		}
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
	return pgtype.Timestamptz{}, false
}

func isKnownAuditEntityType(value string) bool {
	switch value {
	case "client", "supplier", "work_order", "transaction", "recurring_expense", "part", "part_movement", "attachment", "auth", "user", "work_order_part":
		return true
	default:
		return false
	}
}

func uuidPtrFromUUID(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	out := uuidString(value)
	return &out
}
