package money

import (
	"math/big"
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

// NumericToString renders n as a plain decimal string with two fractional digits.
func NumericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return ""
	}
	return numericToRat(n).FloatString(2)
}

func NumericToStringPtr(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	raw, err := n.MarshalJSON()
	if err != nil || string(raw) == "null" {
		return nil
	}
	out := strings.Trim(string(raw), `"`)
	return &out
}

func numericToRat(n pgtype.Numeric) *big.Rat {
	r := new(big.Rat)
	raw, err := n.MarshalJSON()
	if err != nil {
		return r
	}
	s := string(raw)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	if _, ok := r.SetString(s); ok {
		return r
	}
	return new(big.Rat)
}
