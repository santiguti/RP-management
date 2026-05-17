package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/santiguti/rp-management/backend/internal/auth"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

type ctxKey int

const userKey ctxKey = iota

func WithUser(ctx context.Context, u *sqlc.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

func UserFromContext(ctx context.Context) (*sqlc.User, bool) {
	u, ok := ctx.Value(userKey).(*sqlc.User)
	return u, ok
}

func RequireSession(q *sqlc.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("rp_session")
			if err != nil || cookie.Value == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
				return
			}

			hash := auth.HashSessionToken(cookie.Value)
			row, err := q.GetSessionWithUser(r.Context(), hash)
			if errors.Is(err, pgx.ErrNoRows) || err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
				return
			}

			if err := q.TouchSession(context.Background(), hash); err != nil {
				log.Printf("touch session: %v", err)
			}

			ctx := WithUser(r.Context(), &row.User)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || user.Role != role {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("rp_csrf")
		header := r.Header.Get("X-CSRF-Token")
		if err != nil || cookie.Value == "" || header == "" ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "csrf_invalid"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
