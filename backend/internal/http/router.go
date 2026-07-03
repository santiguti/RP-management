package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/config"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/http/handlers"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
	"github.com/santiguti/rp-management/backend/internal/storage"
)

func New(cfg config.Config, pool *pgxpool.Pool) http.Handler {
	queries := sqlc.New(pool)
	store, err := storage.New(cfg.AttachmentsDir)
	if err != nil {
		panic(fmt.Errorf("attachments dir: %w", err))
	}
	authH := handlers.NewAuth(queries, cfg)
	clientsH := handlers.NewClients(queries)
	devicesH := handlers.NewDevices(queries)
	workOrdersH := handlers.NewWorkOrders(queries, pool)
	suppliersH := handlers.NewSuppliers(queries)
	transactionsH := handlers.NewTransactions(queries)
	recurringH := handlers.NewRecurringExpenses(queries)
	reportsH := handlers.NewReports(queries)
	partsH := handlers.NewParts(queries, pool)
	attachmentsH := handlers.NewAttachments(queries, store)
	auditH := handlers.NewAudit(queries)
	importH := handlers.NewImport(pool, queries)
	brandsH := handlers.NewBrands(queries)
	modelsH := handlers.NewDeviceModels(queries)
	typesH := handlers.NewArticleTypes(queries)

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	r.Get("/healthz", healthz(pool))

	r.Route("/api/v1", func(api chi.Router) {
		api.Group(func(pub chi.Router) {
			pub.Use(httprate.LimitByIP(5, time.Minute))
			pub.Post("/auth/login", authH.Login)
		})

		api.Group(func(pr chi.Router) {
			pr.Use(middleware.RequireSession(queries))
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
				wr.Delete("/{ucode}/attachments/{att_ucode}", attachmentsH.Delete)
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
			})
			pr.Group(func(ar chi.Router) {
				ar.Use(middleware.RequireRole("owner"))
				ar.Get("/audit-log", auditH.List)
				ar.Get("/audit-log/", auditH.List)
			})
			pr.Route("/import", func(ir chi.Router) {
				ir.Use(middleware.RequireRole("owner"))
				ir.Use(httprate.LimitByIP(5, time.Minute))
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
				pr2.Post("/", partsH.Create)
				pr2.Get("/{ucode}", partsH.Get)
				pr2.Patch("/{ucode}", partsH.Update)
				pr2.Delete("/{ucode}", partsH.Delete)
				pr2.Get("/{ucode}/movements", partsH.ListMovements)
				pr2.Post("/{ucode}/movements", partsH.CreateMovement)
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

func healthz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"status": "db_unreachable",
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
