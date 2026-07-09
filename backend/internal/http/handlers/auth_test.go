package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/auth"
	"github.com/santiguti/rp-management/backend/internal/config"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
	"github.com/santiguti/rp-management/backend/internal/storage"
)

var testPool *pgxpool.Pool
var testAttachmentStore *storage.FileStore
var (
	testHashOnce sync.Once
	testHash     string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("RP_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Println("skipping: no DATABASE_URL")
		os.Exit(0)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		panic(err)
	}
	testPool = pool
	attachmentsDir, err := os.MkdirTemp("", "rp-management-attachments-test-*")
	if err != nil {
		pool.Close()
		panic(err)
	}
	testAttachmentStore, err = storage.New(attachmentsDir)
	if err != nil {
		pool.Close()
		_ = os.RemoveAll(attachmentsDir)
		panic(err)
	}

	// Serialize against other test packages that share this database
	// (cmd/jobs): both reset + reseed the same tables, and `go test ./...`
	// runs packages in parallel. Advisory lock 421338 must match everywhere.
	lockConn, err := pool.Acquire(context.Background())
	if err != nil {
		pool.Close()
		_ = os.RemoveAll(attachmentsDir)
		panic(err)
	}
	if _, err := lockConn.Exec(context.Background(), `SELECT pg_advisory_lock(421338)`); err != nil {
		lockConn.Release()
		pool.Close()
		_ = os.RemoveAll(attachmentsDir)
		panic(err)
	}

	code := m.Run()

	_, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock(421338)`)
	lockConn.Release()
	pool.Close()
	_ = os.RemoveAll(attachmentsDir)
	os.Exit(code)
}

func newPoolQueries(t *testing.T) (*sqlc.Queries, func()) {
	t.Helper()

	resetTestDB(t)
	return sqlc.New(testPool), func() {}
}

func TestLogin_OK(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	user := seedOwner(t, q)

	ts := httptest.NewServer(testRouter(q))
	defer ts.Close()

	res := postJSON(t, ts.Client(), ts.URL+"/api/v1/auth/login", map[string]string{
		"username": user.Username,
		"password": "pw",
	}, "")
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	var body struct {
		User userDTO `json:"user"`
	}
	decodeJSON(t, res.Body, &body)
	if body.User.Ucode == "" {
		t.Fatal("user.ucode is empty")
	}
	if body.User.Role != "owner" {
		t.Fatalf("user.role = %q, want owner", body.User.Role)
	}

	sessionCookie := responseCookie(res, "rp_session")
	if sessionCookie == nil {
		t.Fatal("missing rp_session cookie")
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("rp_session cookie is not HttpOnly")
	}

	csrfCookie := responseCookie(res, "rp_csrf")
	if csrfCookie == nil {
		t.Fatal("missing rp_csrf cookie")
	}
	if csrfCookie.HttpOnly {
		t.Fatal("rp_csrf cookie should not be HttpOnly")
	}
}

func TestLogin_BadPassword(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	user := seedOwner(t, q)

	ts := httptest.NewServer(testRouter(q))
	defer ts.Close()

	res := postJSON(t, ts.Client(), ts.URL+"/api/v1/auth/login", map[string]string{
		"username": user.Username,
		"password": "wrong",
	}, "")
	defer res.Body.Close()

	assertError(t, res, http.StatusUnauthorized, "invalid_credentials")
	if got := res.Cookies(); len(got) != 0 {
		t.Fatalf("cookies = %v, want none", got)
	}
}

func TestLogin_MissingUser(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	_ = seedOwner(t, q)

	ts := httptest.NewServer(testRouter(q))
	defer ts.Close()

	res := postJSON(t, ts.Client(), ts.URL+"/api/v1/auth/login", map[string]string{
		"username": uniqueUsername(t) + "_missing",
		"password": "pw",
	}, "")
	defer res.Body.Close()

	assertError(t, res, http.StatusUnauthorized, "invalid_credentials")
	if got := res.Cookies(); len(got) != 0 {
		t.Fatalf("cookies = %v, want none", got)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()

	ts := httptest.NewServer(testRouter(q))
	defer ts.Close()

	res, err := ts.Client().Get(ts.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	assertError(t, res, http.StatusUnauthorized, "unauthenticated")
}

func TestMe_OK(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	user := seedOwner(t, q)

	ts, client := newCookieServer(t, q)
	defer ts.Close()

	httpLogin(t, client, ts.URL, user.Username)
	res, err := client.Get(ts.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	var body struct {
		User userDTO `json:"user"`
	}
	decodeJSON(t, res.Body, &body)
	if body.User.Username != user.Username {
		t.Fatalf("user.username = %q, want %q", body.User.Username, user.Username)
	}
}

func TestLogout_OK(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	user := seedOwner(t, q)

	ts, client := newCookieServer(t, q)
	defer ts.Close()

	csrf := httpLogin(t, client, ts.URL, user.Username)
	oldSession := jarCookie(t, client, ts.URL, "rp_session")
	res := postJSON(t, client, ts.URL+"/api/v1/auth/logout", nil, csrf)
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, http.StatusNoContent, readBody(t, res))
	}

	replayCookie(t, client, ts.URL, oldSession)
	me, err := client.Get(ts.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer me.Body.Close()

	assertError(t, me, http.StatusUnauthorized, "unauthenticated")
}

func TestCSRF_RejectsMissingHeader(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	user := seedOwner(t, q)

	ts, client := newCookieServer(t, q)
	defer ts.Close()

	httpLogin(t, client, ts.URL, user.Username)
	res := postJSON(t, client, ts.URL+"/api/v1/auth/logout", nil, "")
	defer res.Body.Close()

	assertError(t, res, http.StatusForbidden, "csrf_invalid")
}

func TestCSRF_RejectsWrongHeader(t *testing.T) {
	q, cleanup := newPoolQueries(t)
	defer cleanup()
	user := seedOwner(t, q)

	ts, client := newCookieServer(t, q)
	defer ts.Close()

	httpLogin(t, client, ts.URL, user.Username)
	res := postJSON(t, client, ts.URL+"/api/v1/auth/logout", nil, "not-the-cookie")
	defer res.Body.Close()

	assertError(t, res, http.StatusForbidden, "csrf_invalid")
}

func testRouter(q *sqlc.Queries) http.Handler {
	authH := NewAuth(q, config.Config{AppEnv: "dev"})
	clientsH := NewClients(q)
	devicesH := NewDevices(q)
	workOrdersH := NewWorkOrders(q, testPool)
	suppliersH := NewSuppliers(q)
	transactionsH := NewTransactions(q)
	recurringH := NewRecurringExpenses(q)
	reportsH := NewReports(q)
	exportsH := NewExports(q)
	partsH := NewParts(q, testPool)
	attachmentsH := NewAttachments(q, testAttachmentStore)
	auditH := NewAudit(q)
	importH := NewImport(testPool, q)
	brandsH := NewBrands(q)
	modelsH := NewDeviceModels(q)
	typesH := NewArticleTypes(q)
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", authH.Login)
		api.Group(func(pr chi.Router) {
			pr.Use(middleware.RequireSession(q))
			pr.Use(middleware.CSRF)
			pr.Post("/auth/logout", authH.Logout)
			pr.Get("/auth/me", authH.Me)
			pr.Route("/brands", func(br chi.Router) {
				br.Get("/", brandsH.List)
				br.Get("/{ucode}/models", modelsH.ListByBrand)
				br.Group(func(o chi.Router) {
					o.Use(middleware.RequireRole("owner"))
					o.Post("/", brandsH.Create)
					o.Patch("/{ucode}", brandsH.Update)
					o.Delete("/{ucode}", brandsH.Delete)
				})
			})
			pr.Route("/clients", func(cr chi.Router) {
				cr.Post("/", clientsH.Create)
				cr.Get("/", clientsH.Search)
				cr.Get("/{ucode}", clientsH.Get)
				cr.Patch("/{ucode}", clientsH.Update)
				cr.Delete("/{ucode}", clientsH.Delete)
				cr.Get("/{ucode}/devices", clientsH.ListDevices)
			})
			pr.Route("/devices", func(dr chi.Router) {
				dr.Post("/", devicesH.Create)
				dr.Get("/", devicesH.Search)
				dr.Get("/{ucode}", devicesH.Get)
				dr.Patch("/{ucode}", devicesH.Update)
				dr.Delete("/{ucode}", devicesH.Delete)
			})
			pr.Route("/work-orders", func(wr chi.Router) {
				wr.Post("/", workOrdersH.Intake)
				wr.Get("/", workOrdersH.Search)
				wr.Get("/{ucode}", workOrdersH.Get)
				wr.Get("/{ucode}/transactions", workOrdersH.ListTransactions)
				wr.Get("/{ucode}/parts", workOrdersH.ListParts)
				wr.Post("/{ucode}/parts", workOrdersH.AddPart)
				wr.Delete("/{ucode}/parts/{id}", workOrdersH.RemovePart)
				wr.Get("/{ucode}/attachments", attachmentsH.List)
				wr.Post("/{ucode}/attachments", attachmentsH.Upload)
				wr.Get("/{ucode}/attachments/{att_ucode}", attachmentsH.Download)
				wr.Group(func(o chi.Router) {
					o.Use(middleware.RequireRole("owner"))
					o.Delete("/{ucode}/attachments/{att_ucode}", attachmentsH.Delete)
				})
				wr.Patch("/{ucode}", workOrdersH.Update)
				wr.Post("/{ucode}/transitions/{event}", workOrdersH.Transition)
			})
			pr.Route("/transactions", func(tr chi.Router) {
				tr.Get("/", transactionsH.Search)
				tr.Post("/", transactionsH.Create)
				tr.Get("/{ucode}", transactionsH.Get)
				tr.Patch("/{ucode}", transactionsH.Update)
				tr.Delete("/{ucode}", transactionsH.Delete)
			})
			pr.Route("/recurring-expenses", func(rr chi.Router) {
				rr.Get("/", recurringH.List)
				rr.Get("/{ucode}", recurringH.Get)
				rr.Group(func(o chi.Router) {
					o.Use(middleware.RequireRole("owner"))
					o.Post("/", recurringH.Create)
					o.Patch("/{ucode}", recurringH.Update)
					o.Delete("/{ucode}", recurringH.Delete)
					o.Post("/{ucode}/run-now", recurringH.RunNow)
				})
			})
			pr.Route("/reports", func(rr chi.Router) {
				rr.Get("/balance", reportsH.Balance)
				rr.Get("/pnl", reportsH.PnL)
				rr.Get("/dashboard", reportsH.Dashboard)
				rr.Get("/low-stock", reportsH.LowStock)
			})
			pr.Get("/transactions.csv", exportsH.Transactions)
			pr.Get("/clients.csv", exportsH.Clients)
			pr.Get("/parts.csv", exportsH.Parts)
			pr.Get("/work-orders.csv", exportsH.WorkOrders)
			pr.Group(func(ar chi.Router) {
				ar.Use(middleware.RequireRole("owner"))
				ar.Get("/audit-log", auditH.List)
				ar.Get("/audit-log/", auditH.List)
			})
			pr.Route("/import", func(ir chi.Router) {
				ir.Use(middleware.RequireRole("owner"))
				ir.Post("/excel", importH.Excel)
			})
			pr.Route("/suppliers", func(sr chi.Router) {
				sr.Get("/", suppliersH.List)
				sr.Post("/", suppliersH.Create)
				sr.Get("/{ucode}", suppliersH.Get)
				sr.Patch("/{ucode}", suppliersH.Update)
				sr.Delete("/{ucode}", suppliersH.Delete)
			})
			pr.Route("/parts", func(pr2 chi.Router) {
				pr2.Get("/", partsH.Search)
				pr2.Get("/{ucode}", partsH.Get)
				pr2.Get("/{ucode}/movements", partsH.ListMovements)
				pr2.Post("/{ucode}/movements", partsH.CreateMovement)
				pr2.Group(func(o chi.Router) {
					o.Use(middleware.RequireRole("owner"))
					o.Post("/", partsH.Create)
					o.Patch("/{ucode}", partsH.Update)
					o.Delete("/{ucode}", partsH.Delete)
				})
			})
			pr.Route("/device-models", func(mr chi.Router) {
				mr.Use(middleware.RequireRole("owner"))
				mr.Post("/", modelsH.Create)
				mr.Patch("/{ucode}", modelsH.Update)
				mr.Delete("/{ucode}", modelsH.Delete)
			})
			pr.Route("/article-types", func(tr chi.Router) {
				tr.Get("/", typesH.List)
				tr.Group(func(o chi.Router) {
					o.Use(middleware.RequireRole("owner"))
					o.Post("/", typesH.Create)
					o.Patch("/{ucode}", typesH.Update)
					o.Delete("/{ucode}", typesH.Delete)
				})
			})
		})
	})
	return r
}

func seedOwner(t *testing.T, q *sqlc.Queries) sqlc.User {
	t.Helper()

	user, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{
		Username:        uniqueUsername(t),
		PasswordHash:    testPasswordHash(t),
		FullName:        "Test Owner",
		Role:            "owner",
		CreatedByUserID: pgtype.Int8{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM rp.users WHERE id = $1`, user.ID)
	})
	return user
}

func testPasswordHash(t *testing.T) string {
	t.Helper()

	testHashOnce.Do(func() {
		hash, err := auth.Hash("pw")
		if err != nil {
			t.Fatal(err)
		}
		testHash = hash
	})
	return testHash
}

func newCookieServer(t *testing.T, q *sqlc.Queries) (*httptest.Server, *http.Client) {
	t.Helper()

	ts := httptest.NewServer(testRouter(q))
	jar, err := cookiejar.New(nil)
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	client := ts.Client()
	client.Jar = jar
	return ts, client
}

func seedSession(t *testing.T, q *sqlc.Queries, client *http.Client, baseURL string, user sqlc.User) string {
	t.Helper()

	if client.Jar == nil {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatal(err)
		}
		client.Jar = jar
	}

	token, err := auth.NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := auth.NewCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := q.CreateSession(context.Background(), sqlc.CreateSessionParams{
		ID:        auth.HashSessionToken(token),
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(30 * 24 * time.Hour), Valid: true},
		UserAgent: pgtype.Text{String: "handler-test", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar.SetCookies(u, []*http.Cookie{
		sessionCookie(config.Config{AppEnv: "dev"}, token, sessionMaxAge),
		csrfCookie(config.Config{AppEnv: "dev"}, csrf, sessionMaxAge),
	})
	return csrf
}

func login(t *testing.T, client *http.Client, baseURL, username string) string {
	t.Helper()

	q := sqlc.New(testPool)
	user, err := q.GetUserByUsername(context.Background(), username)
	if err != nil {
		t.Fatal(err)
	}
	return seedSession(t, q, client, baseURL, user)
}

func httpLogin(t *testing.T, client *http.Client, baseURL, username string) string {
	t.Helper()

	res := postJSON(t, client, baseURL+"/api/v1/auth/login", map[string]string{
		"username": username,
		"password": "pw",
	}, "")
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", res.StatusCode, http.StatusOK, readBody(t, res))
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.Jar.Cookies(u) {
		if cookie.Name == "rp_csrf" {
			return cookie.Value
		}
	}
	t.Fatal("missing rp_csrf cookie")
	return ""
}

func postJSON(t *testing.T, client *http.Client, url string, payload any, csrf string) *http.Response {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	} else {
		body = http.NoBody
	}

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func assertError(t *testing.T, res *http.Response, status int, code string) {
	t.Helper()

	if res.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", res.StatusCode, status, readBody(t, res))
	}

	var body map[string]string
	decodeJSON(t, res.Body, &body)
	if body["error"] != code {
		t.Fatalf("error = %q, want %q", body["error"], code)
	}
}

func responseCookie(res *http.Response, name string) *http.Cookie {
	for _, cookie := range res.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func jarCookie(t *testing.T, client *http.Client, baseURL, name string) *http.Cookie {
	t.Helper()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}

	for _, cookie := range client.Jar.Cookies(u) {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie", name)
	return nil
}

func replayCookie(t *testing.T, client *http.Client, baseURL string, cookie *http.Cookie) {
	t.Helper()

	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar.SetCookies(u, []*http.Cookie{cookie})
}

func decodeJSON(t *testing.T, r io.Reader, dst any) {
	t.Helper()

	if err := json.NewDecoder(r).Decode(dst); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func uniqueUsername(t *testing.T) string {
	t.Helper()

	name := strings.NewReplacer("/", "_", " ", "_").Replace(strings.ToLower(t.Name()))
	if len(name) > 40 {
		name = name[:40]
	}
	return fmt.Sprintf("%s_%d", name, time.Now().UnixNano())
}
