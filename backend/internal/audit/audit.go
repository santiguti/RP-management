package audit

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

type Entry struct {
	Action      string
	EntityType  string
	EntityID    *int64
	EntityUcode *pgtype.UUID
	Before      any
	After       any
}

// Record persists an audit entry after a successful mutation. Failures are
// logged and intentionally ignored so auditing never breaks the user request.
func Record(ctx context.Context, q *sqlc.Queries, r *http.Request, e Entry) {
	var actor pgtype.Int8
	if u, ok := middleware.UserFromContext(ctx); ok {
		actor = pgtype.Int8{Int64: u.ID, Valid: true}
	}

	beforeJSON, err := marshalJSON(e.Before)
	if err != nil {
		log.Printf("audit marshal before: %v", err)
		return
	}
	afterJSON, err := marshalJSON(e.After)
	if err != nil {
		log.Printf("audit marshal after: %v", err)
		return
	}

	var entityID pgtype.Int8
	if e.EntityID != nil {
		entityID = pgtype.Int8{Int64: *e.EntityID, Valid: true}
	}
	var entityUcode pgtype.UUID
	if e.EntityUcode != nil && e.EntityUcode.Valid {
		entityUcode = *e.EntityUcode
	}

	if _, err := q.CreateAuditEntry(ctx, sqlc.CreateAuditEntryParams{
		ActorUserID: actor,
		Action:      e.Action,
		EntityType:  e.EntityType,
		EntityID:    entityID,
		EntityUcode: entityUcode,
		BeforeJson:  beforeJSON,
		AfterJson:   afterJSON,
		Ip:          pgtype.Text{String: clientIP(r), Valid: r != nil},
		UserAgent:   pgtype.Text{String: userAgent(r), Valid: r != nil},
	}); err != nil {
		log.Printf("audit insert (%s): %v", e.Action, err)
	}
}

func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
		return v
	}
	return r.RemoteAddr
}

func userAgent(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.UserAgent()
}
