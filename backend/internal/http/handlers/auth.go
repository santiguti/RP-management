package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/auth"
	"github.com/santiguti/rp-management/backend/internal/config"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
)

const sessionMaxAge = 30 * 24 * 60 * 60

type loginReq struct {
	Username string `json:"username" validate:"required,min=1,max=64"`
	Password string `json:"password" validate:"required,min=1,max=128"`
}

type userDTO struct {
	Ucode    string `json:"ucode"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type Auth struct {
	queries *sqlc.Queries
	cfg     config.Config
	val     *validator.Validate
}

func NewAuth(q *sqlc.Queries, cfg config.Config) *Auth {
	return &Auth{
		queries: q,
		cfg:     cfg,
		val:     validator.New(),
	}
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if err := a.val.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	user, err := a.queries.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	}

	ok, err := auth.Verify(req.Password, user.PasswordHash)
	if err != nil || !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	}

	token, err := auth.NewSessionToken()
	if err != nil {
		log.Printf("new session token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	csrf, err := auth.NewCSRFToken()
	if err != nil {
		log.Printf("new csrf token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	var ip *netip.Addr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			ip = &addr
		}
	}

	if err := a.queries.CreateSession(r.Context(), sqlc.CreateSessionParams{
		ID:        auth.HashSessionToken(token),
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(30 * 24 * time.Hour), Valid: true},
		Ip:        ip,
		UserAgent: pgtype.Text{String: r.UserAgent(), Valid: r.UserAgent() != ""},
	}); err != nil {
		log.Printf("create session: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	if err := a.queries.UpdateUserLastLogin(r.Context(), user.ID); err != nil {
		log.Printf("update user last login: %v", err)
	}

	http.SetCookie(w, sessionCookie(a.cfg, token, sessionMaxAge))
	http.SetCookie(w, csrfCookie(a.cfg, csrf, sessionMaxAge))
	writeJSON(w, http.StatusOK, map[string]userDTO{"user": toUserDTO(user)})
}

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("rp_session"); err == nil && cookie.Value != "" {
		if err := a.queries.DeleteSession(r.Context(), auth.HashSessionToken(cookie.Value)); err != nil {
			log.Printf("delete session: %v", err)
		}
	}

	http.SetCookie(w, sessionCookie(a.cfg, "", -1))
	http.SetCookie(w, csrfCookie(a.cfg, "", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]userDTO{"user": toUserDTO(*user)})
}

func toUserDTO(u sqlc.User) userDTO {
	b := u.Ucode.Bytes
	return userDTO{
		Ucode:    fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]),
		Username: u.Username,
		FullName: u.FullName,
		Role:     u.Role,
	}
}

func sessionCookie(cfg config.Config, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "rp_session",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   cfg.AppEnv == "prod",
		SameSite: http.SameSiteLaxMode,
	}
}

func csrfCookie(cfg config.Config, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "rp_csrf",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   cfg.AppEnv == "prod",
		SameSite: http.SameSiteLaxMode,
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
