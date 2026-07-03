package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/santiguti/rp-management/backend/internal/audit"
	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/domain/money"
	"github.com/santiguti/rp-management/backend/internal/http/middleware"
	"github.com/santiguti/rp-management/backend/internal/importer"
)

const importMaxFormBytes = 15 << 20

type Import struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewImport(pool *pgxpool.Pool, q *sqlc.Queries) *Import {
	return &Import{pool: pool, queries: q}
}

func (i *Import) Excel(w http.ResponseWriter, r *http.Request) {
	kind, ok := parseImportKind(w, r.URL.Query().Get("kind"))
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, importMaxFormBytes)
	if err := r.ParseMultipartForm(importMaxFormBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_file"})
		return
	}
	defer file.Close()

	result, err := importer.Parse(kind, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_file"})
		return
	}

	confirm := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("confirm")), "true")
	if !confirm || result.ValidCount == 0 {
		writeJSON(w, http.StatusOK, importDryRunResponse{Result: result, Committed: false})
		return
	}

	inserted, commitErrors, err := i.commit(r, result)
	if err != nil {
		log.Printf("commit import %s: %v", kind, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if len(commitErrors) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  "commit_failed",
			"errors": commitErrors,
		})
		return
	}

	audit.Record(r.Context(), i.queries, r, audit.Entry{
		Action:     "import.commit",
		EntityType: "import",
		After: map[string]any{
			"kind":           kind,
			"valid":          result.ValidCount,
			"inserted_ucode": inserted,
		},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"committed":       true,
		"kind":            kind,
		"valid":           result.ValidCount,
		"inserted_ucodes": inserted,
	})
}

type importDryRunResponse struct {
	importer.Result
	Committed bool `json:"committed"`
}

func parseImportKind(w http.ResponseWriter, raw string) (importer.Kind, bool) {
	switch importer.Kind(strings.TrimSpace(raw)) {
	case importer.KindClients:
		return importer.KindClients, true
	case importer.KindParts:
		return importer.KindParts, true
	case importer.KindTransactions:
		return importer.KindTransactions, true
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_kind"})
		return "", false
	}
}

func (i *Import) commit(r *http.Request, result importer.Result) ([]string, []importer.RowError, error) {
	tx, err := i.pool.Begin(r.Context())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	qtx := i.queries.WithTx(tx)
	var inserted []string
	var rowErrs []importer.RowError
	switch result.Kind {
	case importer.KindClients:
		inserted, rowErrs = i.commitClients(r, qtx, result.ValidRows)
	case importer.KindParts:
		inserted, rowErrs = i.commitParts(r, qtx, result.ValidRows)
	case importer.KindTransactions:
		inserted, rowErrs = i.commitTransactions(r, qtx, result.ValidRows)
	default:
		rowErrs = []importer.RowError{{Row: 0, Column: "kind", Message: "Tipo de importación inválido"}}
	}
	if len(rowErrs) > 0 {
		return nil, rowErrs, nil
	}
	if err := tx.Commit(r.Context()); err != nil {
		return nil, nil, err
	}
	return inserted, nil, nil
}

func (i *Import) commitClients(r *http.Request, q *sqlc.Queries, rows []any) ([]string, []importer.RowError) {
	user, _ := middleware.UserFromContext(r.Context())
	inserted := make([]string, 0, len(rows))
	for _, raw := range rows {
		row := raw.(importer.ParsedClient)
		client, err := q.CreateClient(r.Context(), sqlc.CreateClientParams{
			Name:            row.Name,
			Phone:           textFromPtr(row.Phone),
			Email:           textFromPtr(row.Email),
			DniCuit:         textFromPtr(row.DniCuit),
			Address:         textFromPtr(row.Address),
			Notes:           textFromPtr(row.Notes),
			ClientType:      row.ClientType,
			CreatedByUserID: pgtype.Int8{Int64: user.ID, Valid: true},
		})
		if isUniqueViolation(err) {
			return nil, []importer.RowError{{Row: row.Row, Column: "phone", Message: "Teléfono ya existe"}}
		}
		if err != nil {
			return nil, []importer.RowError{{Row: row.Row, Column: "row", Message: "No se pudo importar el cliente"}}
		}
		inserted = append(inserted, uuidString(client.Ucode))
	}
	return inserted, nil
}

func (i *Import) commitParts(r *http.Request, q *sqlc.Queries, rows []any) ([]string, []importer.RowError) {
	user, _ := middleware.UserFromContext(r.Context())
	inserted := make([]string, 0, len(rows))
	for _, raw := range rows {
		row := raw.(importer.ParsedPart)
		reorderLevel, ok := numericFromImporterString(row.ReorderLevel)
		if !ok {
			return nil, []importer.RowError{{Row: row.Row, Column: "reorder_level", Message: "Valor numérico inválido"}}
		}
		defaultCost, ok := numericFromImporterString(row.DefaultCost)
		if !ok {
			return nil, []importer.RowError{{Row: row.Row, Column: "default_cost", Message: "Valor numérico inválido"}}
		}
		defaultSalePrice, ok := numericFromImporterString(row.DefaultSalePrice)
		if !ok {
			return nil, []importer.RowError{{Row: row.Row, Column: "default_sale_price", Message: "Valor numérico inválido"}}
		}
		part, err := q.CreatePart(r.Context(), sqlc.CreatePartParams{
			Sku:              textFromPtr(row.Sku),
			Name:             row.Name,
			Description:      pgtype.Text{},
			Unit:             row.Unit,
			ReorderLevel:     reorderLevel,
			DefaultCost:      defaultCost,
			DefaultSalePrice: defaultSalePrice,
			CreatedByUserID:  pgtype.Int8{Int64: user.ID, Valid: true},
		})
		if isUniqueViolation(err) {
			return nil, []importer.RowError{{Row: row.Row, Column: "sku", Message: "SKU ya existe"}}
		}
		if err != nil {
			return nil, []importer.RowError{{Row: row.Row, Column: "row", Message: "No se pudo importar el repuesto"}}
		}
		inserted = append(inserted, uuidString(part.Ucode))
	}
	return inserted, nil
}

func (i *Import) commitTransactions(r *http.Request, q *sqlc.Queries, rows []any) ([]string, []importer.RowError) {
	user, _ := middleware.UserFromContext(r.Context())
	inserted := make([]string, 0, len(rows))
	for _, raw := range rows {
		row := raw.(importer.ParsedTransaction)
		amount, err := money.StringToNumeric(row.Amount)
		if err != nil {
			return nil, []importer.RowError{{Row: row.Row, Column: "amount", Message: "Monto inválido"}}
		}
		date, err := time.Parse("2006-01-02", row.TransactionDate)
		if err != nil {
			return nil, []importer.RowError{{Row: row.Row, Column: "transaction_date", Message: "Fecha inválida"}}
		}

		clientID, supplierID, rowErr := i.resolveImportCounterparty(r, q, row)
		if rowErr != nil {
			return nil, []importer.RowError{*rowErr}
		}
		workOrderID, rowErr := i.resolveImportWorkOrder(r, q, row)
		if rowErr != nil {
			return nil, []importer.RowError{*rowErr}
		}

		transaction, err := q.CreateTransaction(r.Context(), sqlc.CreateTransactionParams{
			TransactionType:  row.TransactionType,
			Amount:           amount,
			Currency:         row.Currency,
			FxRateToArs:      numericOne(),
			TransactionDate:  pgtype.Date{Time: date, Valid: true},
			PaymentMethod:    row.PaymentMethod,
			Category:         row.Category,
			CounterpartyType: row.CounterpartyType,
			ClientID:         clientID,
			SupplierID:       supplierID,
			WorkOrderID:      workOrderID,
			Description:      textFromPtr(row.Description),
			CreatedByUserID:  pgtype.Int8{Int64: user.ID, Valid: true},
		})
		if err != nil {
			return nil, []importer.RowError{{Row: row.Row, Column: "row", Message: "No se pudo importar la transacción"}}
		}
		inserted = append(inserted, uuidString(transaction.Ucode))
	}
	return inserted, nil
}

func (i *Import) resolveImportCounterparty(r *http.Request, q *sqlc.Queries, row importer.ParsedTransaction) (pgtype.Int8, pgtype.Int8, *importer.RowError) {
	switch row.CounterpartyType {
	case "client":
		client, err := q.GetClientByPhone(r.Context(), pgtype.Text{String: *row.ClientPhone, Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.Int8{}, pgtype.Int8{}, &importer.RowError{Row: row.Row, Column: "client_phone", Message: "Cliente no encontrado por teléfono"}
		}
		if err != nil {
			return pgtype.Int8{}, pgtype.Int8{}, &importer.RowError{Row: row.Row, Column: "client_phone", Message: "No se pudo resolver el cliente"}
		}
		return pgtype.Int8{Int64: client.ID, Valid: true}, pgtype.Int8{}, nil
	case "supplier":
		supplier, err := q.GetSupplierByName(r.Context(), *row.SupplierName)
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.Int8{}, pgtype.Int8{}, &importer.RowError{Row: row.Row, Column: "supplier_name", Message: "Proveedor no encontrado"}
		}
		if err != nil {
			return pgtype.Int8{}, pgtype.Int8{}, &importer.RowError{Row: row.Row, Column: "supplier_name", Message: "No se pudo resolver el proveedor"}
		}
		return pgtype.Int8{}, pgtype.Int8{Int64: supplier.ID, Valid: true}, nil
	default:
		return pgtype.Int8{}, pgtype.Int8{}, nil
	}
}

func (i *Import) resolveImportWorkOrder(r *http.Request, q *sqlc.Queries, row importer.ParsedTransaction) (pgtype.Int8, *importer.RowError) {
	if row.WONumber == nil {
		return pgtype.Int8{}, nil
	}
	workOrder, err := q.GetWorkOrderByNumber(r.Context(), *row.WONumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.Int8{}, &importer.RowError{Row: row.Row, Column: "wo_number", Message: "Orden de trabajo no encontrada"}
	}
	if err != nil {
		return pgtype.Int8{}, &importer.RowError{Row: row.Row, Column: "wo_number", Message: "No se pudo resolver la orden de trabajo"}
	}
	return pgtype.Int8{Int64: workOrder.ID, Valid: true}, nil
}

func numericFromImporterString(raw *string) (pgtype.Numeric, bool) {
	if raw == nil {
		return pgtype.Numeric{}, true
	}
	n, err := money.StringToNumeric(*raw)
	return n, err == nil
}

func numericOne() pgtype.Numeric {
	n, _ := money.StringToNumeric("1")
	return n
}
