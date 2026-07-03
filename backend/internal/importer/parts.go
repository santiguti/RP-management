package importer

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"

	"github.com/santiguti/rp-management/backend/internal/domain/money"
)

type ParsedPart struct {
	Name             string  `json:"name"`
	Sku              *string `json:"sku,omitempty"`
	Unit             string  `json:"unit"`
	ReorderLevel     *string `json:"reorder_level,omitempty"`
	DefaultCost      *string `json:"default_cost,omitempty"`
	DefaultSalePrice *string `json:"default_sale_price,omitempty"`
}

func parseParts(f *excelize.File) (Result, error) {
	rows, err := firstSheetRows(f)
	if err != nil {
		return Result{}, err
	}
	if len(rows) < 1 {
		return Result{Kind: KindParts}, nil
	}

	hm := headerMap(rows[0])
	res := Result{Kind: KindParts}
	for ri := 1; ri < len(rows); ri++ {
		row := rows[ri]
		if isBlank(row) {
			continue
		}
		res.TotalRows++
		rowNum := ri + 1

		part := ParsedPart{
			Name:             cellAt(row, hm, "name"),
			Sku:              strPtrOrNil(cellAt(row, hm, "sku")),
			Unit:             cellAt(row, hm, "unit"),
			ReorderLevel:     strPtrOrNil(cellAt(row, hm, "reorder_level")),
			DefaultCost:      strPtrOrNil(cellAt(row, hm, "default_cost")),
			DefaultSalePrice: strPtrOrNil(cellAt(row, hm, "default_sale_price")),
		}
		if part.Unit == "" {
			part.Unit = "unidad"
		}

		errs := validatePartRow(rowNum, &part)
		if len(errs) > 0 {
			res.Errors = append(res.Errors, errs...)
			res.InvalidCount++
			continue
		}
		res.ValidRows = append(res.ValidRows, part)
		res.ValidCount++
	}
	res.Preview = previewSlice(res.ValidRows, 20)
	return res, nil
}

func validatePartRow(row int, part *ParsedPart) []RowError {
	var errs []RowError
	if part.Name == "" {
		errs = append(errs, rowError(row, "name", "Nombre es obligatorio"))
	} else if len(part.Name) > 200 {
		errs = append(errs, rowError(row, "name", "Nombre demasiado largo"))
	}
	if part.Sku != nil && len(*part.Sku) > 64 {
		errs = append(errs, rowError(row, "sku", "SKU demasiado largo"))
	}
	part.Unit = strings.TrimSpace(part.Unit)
	if part.Unit == "" {
		errs = append(errs, rowError(row, "unit", "Unidad es obligatoria"))
	} else if len(part.Unit) > 32 {
		errs = append(errs, rowError(row, "unit", "Unidad demasiado larga"))
	}

	if err := validateOptionalNonNegativeDecimal(part.ReorderLevel); err != nil {
		errs = append(errs, rowError(row, "reorder_level", "Valor numérico inválido"))
	}
	if err := validateOptionalNonNegativeDecimal(part.DefaultCost); err != nil {
		errs = append(errs, rowError(row, "default_cost", "Valor numérico inválido"))
	}
	if err := validateOptionalNonNegativeDecimal(part.DefaultSalePrice); err != nil {
		errs = append(errs, rowError(row, "default_sale_price", "Valor numérico inválido"))
	}
	return errs
}

func validateOptionalNonNegativeDecimal(raw *string) error {
	if raw == nil {
		return nil
	}
	n, err := money.StringToNumeric(*raw)
	if err != nil || numericSign(n) < 0 {
		return errInvalidDecimal
	}
	return nil
}

func numericSign(n pgtype.Numeric) int {
	if !n.Valid || n.Int == nil {
		return -1
	}
	return n.Int.Sign()
}
