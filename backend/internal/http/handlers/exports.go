package handlers

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
	"github.com/santiguti/rp-management/backend/internal/i18n"
)

type Exports struct {
	queries *sqlc.Queries
}

func NewExports(q *sqlc.Queries) *Exports {
	return &Exports{queries: q}
}

func (e *Exports) Transactions(w http.ResponseWriter, r *http.Request) {
	params, ok := e.transactionExportParams(w, r)
	if !ok {
		return
	}
	rows, err := e.queries.ListTransactions(r.Context(), params)
	if err != nil {
		log.Printf("export transactions: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeCSV(w, "movimientos-"+time.Now().UTC().Format("2006-01-02")+".csv", []string{
		"Fecha", "Tipo", "Categoría", "Método", "Contraparte", "Orden", "Monto", "Moneda", "Descripción",
	}, func(cw *csv.Writer) error {
		for _, row := range rows {
			dto := toTransactionDTO(enrichedFromListTransaction(row))
			if err := cw.Write([]string{
				dto.TransactionDate,
				i18n.Lookup(i18n.TransactionType, dto.TransactionType),
				i18n.Lookup(i18n.Category, dto.Category),
				i18n.Lookup(i18n.PaymentMethod, dto.PaymentMethod),
				transactionCounterpartyLabel(dto),
				transactionWorkOrderLabel(dto),
				dto.Amount,
				dto.Currency,
				stringValue(dto.Description),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Exports) Clients(w http.ResponseWriter, r *http.Request) {
	rows, err := e.queries.SearchClients(r.Context(), sqlc.SearchClientsParams{
		PageSize:   10000,
		PageOffset: 0,
	})
	if err != nil {
		log.Printf("export clients: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeCSV(w, "clientes-"+time.Now().UTC().Format("2006-01-02")+".csv", []string{
		"Nombre", "Tipo", "Teléfono", "Email", "DNI/CUIT", "Dirección", "Notas", "Creado",
	}, func(cw *csv.Writer) error {
		for _, row := range rows {
			dto := toClientDTO(row)
			if err := cw.Write([]string{
				dto.Name,
				dto.ClientType,
				stringValue(dto.Phone),
				stringValue(dto.Email),
				stringValue(dto.DniCuit),
				stringValue(dto.Address),
				stringValue(dto.Notes),
				dto.CreatedTs,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Exports) Parts(w http.ResponseWriter, r *http.Request) {
	rows, err := e.queries.SearchParts(r.Context(), sqlc.SearchPartsParams{
		PageSize:   10000,
		PageOffset: 0,
	})
	if err != nil {
		log.Printf("export parts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeCSV(w, "repuestos-"+time.Now().UTC().Format("2006-01-02")+".csv", []string{
		"Nombre", "SKU", "Unidad", "Stock actual", "Punto de reposición", "Costo", "Precio venta", "Creado",
	}, func(cw *csv.Writer) error {
		for _, row := range rows {
			dto := toPartDTO(row)
			if err := cw.Write([]string{
				dto.Name,
				stringValue(dto.Sku),
				dto.Unit,
				dto.CurrentStock,
				stringValue(dto.ReorderLevel),
				stringValue(dto.DefaultCost),
				stringValue(dto.DefaultSalePrice),
				dto.CreatedTs,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Exports) WorkOrders(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !isKnownWorkOrderStatus(status) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_status"})
		return
	}
	rows, err := e.queries.ListWorkOrders(r.Context(), sqlc.ListWorkOrdersParams{
		Status:     status,
		Q:          strings.TrimSpace(r.URL.Query().Get("q")),
		PageSize:   10000,
		PageOffset: 0,
	})
	if err != nil {
		log.Printf("export work orders: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeCSV(w, "ordenes-"+time.Now().UTC().Format("2006-01-02")+".csv", []string{
		"Número", "Estado", "Tipo de servicio", "Cliente", "Dispositivo", "Recibida", "Total mano de obra", "Total repuestos", "Total final",
	}, func(cw *csv.Writer) error {
		for _, row := range rows {
			dto := toWorkOrderDTOFromList(row)
			if err := cw.Write([]string{
				dto.WoNumber,
				i18n.Lookup(i18n.WorkOrderStatus, dto.Status),
				i18n.Lookup(i18n.ServiceType, dto.ServiceType),
				dto.Client.Name,
				workOrderDeviceLabel(dto.Device),
				dto.ReceivedTs,
				stringValue(dto.LaborAmount),
				stringValue(dto.PartsAmount),
				stringValue(dto.FinalAmount),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Exports) transactionExportParams(w http.ResponseWriter, r *http.Request) (sqlc.ListTransactionsParams, bool) {
	q := r.URL.Query()
	transactionType := strings.TrimSpace(q.Get("type"))
	category := strings.TrimSpace(q.Get("category"))
	if transactionType != "" && !isKnownTransactionType(transactionType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
		return sqlc.ListTransactionsParams{}, false
	}
	if category != "" && !isKnownTransactionCategory(category) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
		return sqlc.ListTransactionsParams{}, false
	}
	params := sqlc.ListTransactionsParams{
		TransactionType: transactionType,
		Category:        category,
		PageSize:        10000,
		PageOffset:      0,
	}
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		date, ok := parseTransactionDateForExport(w, raw)
		if !ok {
			return sqlc.ListTransactionsParams{}, false
		}
		params.HasFrom = true
		params.DateFrom = date
	}
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		date, ok := parseTransactionDateForExport(w, raw)
		if !ok {
			return sqlc.ListTransactionsParams{}, false
		}
		params.HasTo = true
		params.DateTo = date
	}
	if raw := strings.TrimSpace(q.Get("work_order_ucode")); raw != "" {
		ucode, ok := parseUcode(w, raw)
		if !ok {
			return sqlc.ListTransactionsParams{}, false
		}
		wo, err := e.queries.GetWorkOrderByUcode(r.Context(), ucode)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown_work_order"})
			return sqlc.ListTransactionsParams{}, false
		}
		params.HasWorkOrder = true
		params.WorkOrderID = wo.WorkOrder.ID
	}
	return params, true
}

func writeCSV(w http.ResponseWriter, filename string, headers []string, writeRows func(*csv.Writer) error) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	_ = cw.Write(headers)
	if err := writeRows(cw); err != nil {
		log.Printf("write csv %s: %v", filename, err)
	}
	cw.Flush()
}

func parseTransactionDateForExport(w http.ResponseWriter, raw string) (pgtype.Date, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_date"})
		return pgtype.Date{}, false
	}
	return pgtype.Date{Time: parsed, Valid: true}, true
}

func transactionCounterpartyLabel(dto transactionDTO) string {
	if dto.Client != nil {
		return dto.Client.Name
	}
	if dto.Supplier != nil {
		return dto.Supplier.Name
	}
	if dto.RecurringExpense != nil {
		return dto.RecurringExpense.Name
	}
	return i18n.Lookup(i18n.CounterpartyType, dto.CounterpartyType)
}

func transactionWorkOrderLabel(dto transactionDTO) string {
	if dto.WorkOrder == nil {
		return ""
	}
	return dto.WorkOrder.WoNumber
}

func workOrderDeviceLabel(device wODeviceDTO) string {
	parts := []string{device.BrandName}
	if device.ModelName != nil {
		parts = append(parts, *device.ModelName)
	}
	parts = append(parts, device.ArticleTypeName)
	return strings.Join(parts, " ")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
