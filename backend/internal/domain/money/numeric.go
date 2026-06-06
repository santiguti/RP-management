package money

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// StringToNumeric parses a decimal string into a pgtype.Numeric suitable for
// NUMERIC columns. It returns an error on malformed input.
func StringToNumeric(raw string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(strings.TrimSpace(raw)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}
