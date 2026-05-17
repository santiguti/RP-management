package recurring

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/santiguti/rp-management/backend/internal/db/sqlc"
)

const AlreadyGeneratedReason = "already_generated_for_due_date"

type ProcessResult struct {
	Transaction *sqlc.Transaction
	DueDate     time.Time
	Generated   bool
	Reason      string
}

// DueDate returns the most recent on-or-before today calendar date when a rule
// with the given day_of_month was supposed to fire.
func DueDate(today time.Time, dayOfMonth int) time.Time {
	today = today.UTC()
	base := time.Date(today.Year(), today.Month(), dayOfMonth, 0, 0, 0, 0, time.UTC)
	if today.Day() >= dayOfMonth {
		return base
	}
	return base.AddDate(0, -1, 0)
}

func ShouldGenerate(due time.Time, lastGenerated *time.Time) bool {
	if lastGenerated == nil {
		return true
	}
	return lastGenerated.Before(due)
}

func ProcessOne(ctx context.Context, q *sqlc.Queries, rule sqlc.RecurringExpense, today time.Time) (ProcessResult, error) {
	due := DueDate(today, int(rule.DayOfMonth))
	if !ShouldGenerate(due, datePtr(rule.LastGeneratedDate)) {
		return ProcessResult{
			DueDate: due,
			Reason:  AlreadyGeneratedReason,
		}, nil
	}

	fxRate, err := numericFromString("1")
	if err != nil {
		return ProcessResult{}, err
	}
	counterpartyType := "none"
	if rule.SupplierID.Valid {
		counterpartyType = "supplier"
	}
	description := rule.Description
	if !description.Valid || strings.TrimSpace(description.String) == "" {
		description = pgtype.Text{String: rule.Name, Valid: true}
	}

	tx, err := q.CreateTransaction(ctx, sqlc.CreateTransactionParams{
		TransactionType:    "expense",
		Amount:             rule.Amount,
		Currency:           rule.Currency,
		FxRateToArs:        fxRate,
		TransactionDate:    pgtype.Date{Time: due, Valid: true},
		PaymentMethod:      rule.PaymentMethod,
		Category:           rule.Category,
		CounterpartyType:   counterpartyType,
		SupplierID:         rule.SupplierID,
		Description:        description,
		RecurringExpenseID: pgtype.Int8{Int64: rule.ID, Valid: true},
	})
	if err != nil {
		return ProcessResult{}, err
	}
	if err := q.MarkRecurringExpenseGenerated(ctx, sqlc.MarkRecurringExpenseGeneratedParams{
		ID:                rule.ID,
		LastGeneratedDate: pgtype.Date{Time: due, Valid: true},
	}); err != nil {
		return ProcessResult{}, err
	}
	return ProcessResult{
		Transaction: &tx,
		DueDate:     due,
		Generated:   true,
	}, nil
}

func datePtr(date pgtype.Date) *time.Time {
	if !date.Valid {
		return nil
	}
	return &date.Time
}

func numericFromString(raw string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(raw); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}
