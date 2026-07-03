package importer

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/xuri/excelize/v2"

	clientdomain "github.com/santiguti/rp-management/backend/internal/domain/clients"
)

type ParsedClient struct {
	Row        int     `json:"-"`
	Name       string  `json:"name"`
	Phone      *string `json:"phone,omitempty"`
	Email      *string `json:"email,omitempty"`
	DniCuit    *string `json:"dni_cuit,omitempty"`
	Address    *string `json:"address,omitempty"`
	Notes      *string `json:"notes,omitempty"`
	ClientType string  `json:"client_type"`
}

func parseClients(f *excelize.File) (Result, error) {
	rows, err := firstSheetRows(f)
	if err != nil {
		return Result{}, err
	}
	if len(rows) < 1 {
		return Result{Kind: KindClients}, nil
	}

	hm := headerMap(rows[0])
	res := Result{Kind: KindClients}
	for ri := 1; ri < len(rows); ri++ {
		row := rows[ri]
		if isBlank(row) {
			continue
		}
		res.TotalRows++
		rowNum := ri + 1

		client := ParsedClient{
			Row:        rowNum,
			Name:       cellAt(row, hm, "name"),
			Phone:      strPtrOrNil(cellAt(row, hm, "phone")),
			Email:      strPtrOrNil(cellAt(row, hm, "email")),
			DniCuit:    strPtrOrNil(cellAt(row, hm, "dni_cuit")),
			Address:    strPtrOrNil(cellAt(row, hm, "address")),
			Notes:      strPtrOrNil(cellAt(row, hm, "notes")),
			ClientType: cellAt(row, hm, "client_type"),
		}
		if client.ClientType == "" {
			client.ClientType = "particular"
		}

		errs := validateClientRow(rowNum, &client)
		if len(errs) > 0 {
			res.Errors = append(res.Errors, errs...)
			res.InvalidCount++
			continue
		}
		res.ValidRows = append(res.ValidRows, client)
		res.ValidCount++
	}
	res.Preview = previewSlice(res.ValidRows, 20)
	return res, nil
}

func validateClientRow(row int, client *ParsedClient) []RowError {
	var errs []RowError
	if client.Name == "" {
		errs = append(errs, rowError(row, "name", "Nombre es obligatorio"))
	} else if len(client.Name) > 200 {
		errs = append(errs, rowError(row, "name", "Nombre demasiado largo"))
	}

	if client.Phone != nil {
		normalized, err := clientdomain.NormalizeE164(*client.Phone)
		if err != nil && !errors.Is(err, clientdomain.ErrPhoneEmpty) {
			errs = append(errs, rowError(row, "phone", "Teléfono inválido"))
		} else if err == nil {
			client.Phone = &normalized
		}
	}

	if client.Email != nil {
		if err := validator.New().Var(*client.Email, "email,max=200"); err != nil {
			errs = append(errs, rowError(row, "email", "Email inválido"))
		}
	}
	if client.DniCuit != nil && len(*client.DniCuit) > 32 {
		errs = append(errs, rowError(row, "dni_cuit", "DNI/CUIT demasiado largo"))
	}
	if client.Address != nil && len(*client.Address) > 400 {
		errs = append(errs, rowError(row, "address", "Dirección demasiado larga"))
	}
	if client.Notes != nil && len(*client.Notes) > 2000 {
		errs = append(errs, rowError(row, "notes", "Notas demasiado largas"))
	}

	client.ClientType = strings.TrimSpace(client.ClientType)
	switch client.ClientType {
	case "particular", "empresa":
	default:
		errs = append(errs, rowError(row, "client_type", "Tipo de cliente inválido"))
	}
	return errs
}

func rowError(row int, column, message string) RowError {
	return RowError{Row: row, Column: column, Message: message}
}
