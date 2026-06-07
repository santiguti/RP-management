package parts

import (
	"errors"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNonPositiveQuantity = errors.New("quantity must be non-zero")
	ErrInsufficientStock   = errors.New("insufficient stock")
)

type MovementType string

const (
	MovementPurchase   MovementType = "purchase"
	MovementUse        MovementType = "use"
	MovementAdjustment MovementType = "adjustment"
	MovementReturn     MovementType = "return"
)

func SignedDelta(t MovementType, absQty *big.Rat, adjustmentOut bool) (*big.Rat, error) {
	if absQty.Sign() == 0 {
		return nil, ErrNonPositiveQuantity
	}
	out := new(big.Rat).Abs(absQty)
	switch t {
	case MovementPurchase, MovementReturn:
		return out, nil
	case MovementUse:
		return out.Neg(out), nil
	case MovementAdjustment:
		if adjustmentOut {
			return out.Neg(out), nil
		}
		return out, nil
	default:
		return nil, errors.New("unknown movement type")
	}
}

func CheckStockSufficient(currentStock *big.Rat, delta *big.Rat) error {
	next := new(big.Rat).Add(currentStock, delta)
	if next.Sign() < 0 {
		return ErrInsufficientStock
	}
	return nil
}

func NumericToRat(n pgtype.Numeric) *big.Rat {
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
