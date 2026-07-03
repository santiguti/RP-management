package importer

import (
	"errors"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	clientdomain "github.com/santiguti/rp-management/backend/internal/domain/clients"
	"github.com/santiguti/rp-management/backend/internal/domain/money"
)

var errInvalidDecimal = errors.New("invalid decimal")

type ParsedTransaction struct {
	Row              int     `json:"-"`
	TransactionType  string  `json:"transaction_type"`
	Amount           string  `json:"amount"`
	Currency         string  `json:"currency"`
	TransactionDate  string  `json:"transaction_date"`
	PaymentMethod    string  `json:"payment_method"`
	Category         string  `json:"category"`
	CounterpartyType string  `json:"counterparty_type"`
	ClientPhone      *string `json:"client_phone,omitempty"`
	SupplierName     *string `json:"supplier_name,omitempty"`
	WONumber         *string `json:"wo_number,omitempty"`
	Description      *string `json:"description,omitempty"`
}

func parseTransactions(f *excelize.File) (Result, error) {
	rows, err := firstSheetRows(f)
	if err != nil {
		return Result{}, err
	}
	if len(rows) < 1 {
		return Result{Kind: KindTransactions}, nil
	}

	hm := headerMap(rows[0])
	res := Result{Kind: KindTransactions}
	for ri := 1; ri < len(rows); ri++ {
		row := rows[ri]
		if isBlank(row) {
			continue
		}
		res.TotalRows++
		rowNum := ri + 1

		tx := ParsedTransaction{
			Row:              rowNum,
			TransactionType:  cellAt(row, hm, "transaction_type"),
			Amount:           cellAt(row, hm, "amount"),
			Currency:         strings.ToUpper(cellAt(row, hm, "currency")),
			TransactionDate:  cellAt(row, hm, "transaction_date"),
			PaymentMethod:    cellAt(row, hm, "payment_method"),
			Category:         cellAt(row, hm, "category"),
			CounterpartyType: cellAt(row, hm, "counterparty_type"),
			ClientPhone:      strPtrOrNil(cellAt(row, hm, "client_phone")),
			SupplierName:     strPtrOrNil(cellAt(row, hm, "supplier_name")),
			WONumber:         strPtrOrNil(cellAt(row, hm, "wo_number")),
			Description:      strPtrOrNil(cellAt(row, hm, "description")),
		}
		if tx.Currency == "" {
			tx.Currency = "ARS"
		}
		if tx.TransactionDate == "" {
			tx.TransactionDate = time.Now().UTC().Format("2006-01-02")
		}

		errs := validateTransactionRow(rowNum, &tx)
		if len(errs) > 0 {
			res.Errors = append(res.Errors, errs...)
			res.InvalidCount++
			continue
		}
		res.ValidRows = append(res.ValidRows, tx)
		res.ValidCount++
	}
	res.Preview = previewSlice(res.ValidRows, 20)
	return res, nil
}

func validateTransactionRow(row int, tx *ParsedTransaction) []RowError {
	var errs []RowError
	if !isKnownTransactionType(tx.TransactionType) {
		errs = append(errs, rowError(row, "transaction_type", "Tipo de transacción inválido"))
	}
	if err := validatePositiveDecimal(tx.Amount); err != nil {
		errs = append(errs, rowError(row, "amount", "Monto inválido"))
	}
	if len(tx.Currency) != 3 {
		errs = append(errs, rowError(row, "currency", "Moneda inválida"))
	}
	if _, err := time.Parse("2006-01-02", tx.TransactionDate); err != nil {
		errs = append(errs, rowError(row, "transaction_date", "Fecha inválida"))
	}
	if !isKnownPaymentMethod(tx.PaymentMethod) {
		errs = append(errs, rowError(row, "payment_method", "Medio de pago inválido"))
	}
	if !isKnownTransactionCategory(tx.Category) {
		errs = append(errs, rowError(row, "category", "Categoría inválida"))
	}
	if !isKnownCounterpartyType(tx.CounterpartyType) {
		errs = append(errs, rowError(row, "counterparty_type", "Tipo de contraparte inválido"))
	}

	if tx.ClientPhone != nil {
		normalized, err := clientdomain.NormalizeE164(*tx.ClientPhone)
		if err != nil {
			errs = append(errs, rowError(row, "client_phone", "Teléfono de cliente inválido"))
		} else {
			tx.ClientPhone = &normalized
		}
	}
	errs = append(errs, validateCounterpartyShape(row, tx)...)
	if tx.Description != nil && len(*tx.Description) > 2000 {
		errs = append(errs, rowError(row, "description", "Descripción demasiado larga"))
	}
	return errs
}

func validatePositiveDecimal(raw string) error {
	n, err := money.StringToNumeric(raw)
	if err != nil || numericSign(n) <= 0 {
		return errInvalidDecimal
	}
	return nil
}

func validateCounterpartyShape(row int, tx *ParsedTransaction) []RowError {
	hasClient := tx.ClientPhone != nil
	hasSupplier := tx.SupplierName != nil
	switch tx.CounterpartyType {
	case "client":
		if !hasClient {
			return []RowError{rowError(row, "client_phone", "Cliente requerido")}
		}
		if hasSupplier {
			return []RowError{rowError(row, "supplier_name", "Proveedor no corresponde")}
		}
	case "supplier":
		if !hasSupplier {
			return []RowError{rowError(row, "supplier_name", "Proveedor requerido")}
		}
		if hasClient {
			return []RowError{rowError(row, "client_phone", "Cliente no corresponde")}
		}
	case "none":
		if hasClient || hasSupplier {
			return []RowError{rowError(row, "counterparty_type", "Contraparte no corresponde")}
		}
	}
	return nil
}

func isKnownTransactionType(value string) bool {
	return value == "income" || value == "expense"
}

func isKnownPaymentMethod(value string) bool {
	switch value {
	case "cash", "transfer", "card", "mercadopago", "other":
		return true
	default:
		return false
	}
}

func isKnownTransactionCategory(value string) bool {
	switch value {
	case "wo_payment", "wo_deposit", "part_purchase", "supplies", "rent", "utilities", "salary", "taxes", "food", "transport", "other_income", "other_expense":
		return true
	default:
		return false
	}
}

func isKnownCounterpartyType(value string) bool {
	switch value {
	case "client", "supplier", "none":
		return true
	default:
		return false
	}
}
