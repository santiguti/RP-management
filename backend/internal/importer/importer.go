package importer

import (
	"errors"
	"io"

	"github.com/xuri/excelize/v2"
)

type Kind string

const (
	KindClients      Kind = "clients"
	KindParts        Kind = "parts"
	KindTransactions Kind = "transactions"
)

var ErrUnknownKind = errors.New("unknown import kind")

type RowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

type Result struct {
	Kind         Kind       `json:"kind"`
	TotalRows    int        `json:"total_rows"`
	ValidRows    []any      `json:"-"`
	Preview      []any      `json:"preview"`
	Errors       []RowError `json:"errors"`
	ValidCount   int        `json:"valid"`
	InvalidCount int        `json:"invalid"`
}

func Parse(kind Kind, r io.Reader) (Result, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = f.Close() }()

	switch kind {
	case KindClients:
		return parseClients(f)
	case KindParts:
		return parseParts(f)
	case KindTransactions:
		return parseTransactions(f)
	default:
		return Result{}, ErrUnknownKind
	}
}

func previewSlice(rows []any, max int) []any {
	if len(rows) <= max {
		return rows
	}
	return rows[:max]
}
