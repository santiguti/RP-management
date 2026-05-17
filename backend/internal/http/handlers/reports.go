package handlers

import (
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

type balanceReportDTO struct {
	From             string `json:"from"`
	To               string `json:"to"`
	IncomeArs        string `json:"income_ars"`
	ExpenseArs       string `json:"expense_ars"`
	NetArs           string `json:"net_ars"`
	TransactionCount int64  `json:"transaction_count"`
}

type pnlReportDTO struct {
	From    string           `json:"from"`
	To      string           `json:"to"`
	Income  []pnlCategoryDTO `json:"income"`
	Expense []pnlCategoryDTO `json:"expense"`
}

type pnlCategoryDTO struct {
	Category string `json:"category"`
	TotalArs string `json:"total_ars"`
	Count    int64  `json:"count"`
}

type dashboardReportDTO struct {
	Today                  moneySummaryDTO          `json:"today"`
	Month                  moneySummaryDTO          `json:"month"`
	OpenWorkOrdersByStatus map[string]int64         `json:"open_work_orders_by_status"`
	AgingReadyWorkOrders   []agingReadyWorkOrderDTO `json:"aging_ready_work_orders"`
	TopClients90d          []topClientDTO           `json:"top_clients_90d"`
}

type moneySummaryDTO struct {
	IncomeArs  string `json:"income_ars"`
	ExpenseArs string `json:"expense_ars"`
	NetArs     string `json:"net_ars"`
}

type agingReadyWorkOrderDTO struct {
	Ucode      string `json:"ucode"`
	WoNumber   string `json:"wo_number"`
	ReadyTs    string `json:"ready_ts"`
	ClientName string `json:"client_name"`
	DaysReady  int    `json:"days_ready"`
}

type topClientDTO struct {
	Ucode    string `json:"ucode"`
	Name     string `json:"name"`
	TotalArs string `json:"total_ars"`
}

type Reports struct {
	queries *sqlc.Queries
	val     *validator.Validate
}

func NewReports(q *sqlc.Queries) *Reports {
	return &Reports{queries: q, val: validator.New()}
}

func (rp *Reports) Balance(w http.ResponseWriter, r *http.Request) {
	dateFrom, dateTo, fromLabel, toLabel, ok := parseReportDateRange(w, r, false)
	if !ok {
		return
	}
	row, err := rp.queries.ReportBalance(r.Context(), sqlc.ReportBalanceParams{
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		log.Printf("report balance: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	income := reportMoneyString(row.IncomeArs)
	expense := reportMoneyString(row.ExpenseArs)
	writeJSON(w, http.StatusOK, balanceReportDTO{
		From:             fromLabel,
		To:               toLabel,
		IncomeArs:        income,
		ExpenseArs:       expense,
		NetArs:           subtractMoneyStrings(income, expense),
		TransactionCount: row.TransactionCount,
	})
}

func (rp *Reports) PnL(w http.ResponseWriter, r *http.Request) {
	dateFrom, dateTo, fromLabel, toLabel, ok := parseReportDateRange(w, r, true)
	if !ok {
		return
	}
	rows, err := rp.queries.ReportPnLByCategory(r.Context(), sqlc.ReportPnLByCategoryParams{
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		log.Printf("report pnl: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	out := pnlReportDTO{
		From:    fromLabel,
		To:      toLabel,
		Income:  []pnlCategoryDTO{},
		Expense: []pnlCategoryDTO{},
	}
	for _, row := range rows {
		item := pnlCategoryDTO{
			Category: row.Category,
			TotalArs: reportMoneyString(row.TotalArs),
			Count:    row.TransactionCount,
		}
		if row.TransactionType == "income" {
			out.Income = append(out.Income, item)
		} else {
			out.Expense = append(out.Expense, item)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (rp *Reports) Dashboard(w http.ResponseWriter, r *http.Request) {
	counters, err := rp.queries.ReportDashboardCounters(r.Context())
	if err != nil {
		log.Printf("report dashboard counters: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	statusRows, err := rp.queries.ReportWorkOrderCountsByStatus(r.Context())
	if err != nil {
		log.Printf("report work order counts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	agingRows, err := rp.queries.ReportAgingReadyWorkOrders(r.Context())
	if err != nil {
		log.Printf("report aging ready work orders: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	topRows, err := rp.queries.ReportTopClientsByRevenue(r.Context())
	if err != nil {
		log.Printf("report top clients: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	statuses := make(map[string]int64, len(statusRows))
	for _, row := range statusRows {
		statuses[row.Status] = row.Count
	}
	aging := make([]agingReadyWorkOrderDTO, 0, len(agingRows))
	for _, row := range agingRows {
		daysReady := 0
		if row.ReadyTs.Valid {
			daysReady = int(time.Since(row.ReadyTs.Time) / (24 * time.Hour))
		}
		aging = append(aging, agingReadyWorkOrderDTO{
			Ucode:      uuidString(row.Ucode),
			WoNumber:   row.WoNumber,
			ReadyTs:    timeString(row.ReadyTs),
			ClientName: row.ClientName,
			DaysReady:  daysReady,
		})
	}
	topClients := make([]topClientDTO, 0, len(topRows))
	for _, row := range topRows {
		topClients = append(topClients, topClientDTO{
			Ucode:    uuidString(row.Ucode),
			Name:     row.Name,
			TotalArs: reportMoneyString(row.TotalArs),
		})
	}

	todayIncome := reportMoneyString(counters.IncomeTodayArs)
	todayExpense := reportMoneyString(counters.ExpenseTodayArs)
	monthIncome := reportMoneyString(counters.IncomeMonthArs)
	monthExpense := reportMoneyString(counters.ExpenseMonthArs)
	writeJSON(w, http.StatusOK, dashboardReportDTO{
		Today: moneySummaryDTO{
			IncomeArs:  todayIncome,
			ExpenseArs: todayExpense,
			NetArs:     subtractMoneyStrings(todayIncome, todayExpense),
		},
		Month: moneySummaryDTO{
			IncomeArs:  monthIncome,
			ExpenseArs: monthExpense,
			NetArs:     subtractMoneyStrings(monthIncome, monthExpense),
		},
		OpenWorkOrdersByStatus: statuses,
		AgingReadyWorkOrders:   aging,
		TopClients90d:          topClients,
	})
}

func parseReportDateRange(w http.ResponseWriter, r *http.Request, allowPeriod bool) (pgtype.Date, pgtype.Date, string, string, bool) {
	query := r.URL.Query()
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if allowPeriod {
		switch strings.TrimSpace(query.Get("period")) {
		case "":
		case "month":
			from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		case "year":
			from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
			to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter"})
			return pgtype.Date{}, pgtype.Date{}, "", "", false
		}
	}

	if raw := strings.TrimSpace(query.Get("from")); raw != "" {
		parsed, ok := parseReportDate(w, raw)
		if !ok {
			return pgtype.Date{}, pgtype.Date{}, "", "", false
		}
		from = parsed
	}
	if raw := strings.TrimSpace(query.Get("to")); raw != "" {
		parsed, ok := parseReportDate(w, raw)
		if !ok {
			return pgtype.Date{}, pgtype.Date{}, "", "", false
		}
		to = parsed
	}
	return pgtype.Date{Time: from, Valid: true}, pgtype.Date{Time: to, Valid: true}, from.Format("2006-01-02"), to.Format("2006-01-02"), true
}

func parseReportDate(w http.ResponseWriter, raw string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_date"})
		return time.Time{}, false
	}
	return parsed, true
}

func subtractMoneyStrings(left, right string) string {
	l := decimalRat(left)
	r := decimalRat(right)
	l.Sub(l, r)
	return l.FloatString(2)
}

func reportMoneyString(n pgtype.Numeric) string {
	return decimalRat(numericToString(n)).FloatString(2)
}

func decimalRat(raw string) *big.Rat {
	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return new(big.Rat)
	}
	return value
}
