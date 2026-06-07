package parts

import (
	"errors"
	"math/big"
	"testing"
)

func TestSignedDelta_Purchase(t *testing.T) {
	got, err := SignedDelta(MovementPurchase, big.NewRat(5, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(big.NewRat(5, 1)) != 0 {
		t.Fatalf("delta = %s, want 5", got)
	}
}

func TestSignedDelta_Use(t *testing.T) {
	got, err := SignedDelta(MovementUse, big.NewRat(5, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(big.NewRat(-5, 1)) != 0 {
		t.Fatalf("delta = %s, want -5", got)
	}
}

func TestSignedDelta_AdjustmentOut(t *testing.T) {
	got, err := SignedDelta(MovementAdjustment, big.NewRat(3, 2), true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(big.NewRat(-3, 2)) != 0 {
		t.Fatalf("delta = %s, want -3/2", got)
	}
}

func TestSignedDelta_AdjustmentIn(t *testing.T) {
	got, err := SignedDelta(MovementAdjustment, big.NewRat(3, 2), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatalf("delta = %s, want 3/2", got)
	}
}

func TestSignedDelta_ZeroQuantity(t *testing.T) {
	_, err := SignedDelta(MovementPurchase, big.NewRat(0, 1), false)
	if !errors.Is(err, ErrNonPositiveQuantity) {
		t.Fatalf("err = %v, want ErrNonPositiveQuantity", err)
	}
}

func TestSignedDelta_UnknownType(t *testing.T) {
	_, err := SignedDelta(MovementType("mystery"), big.NewRat(1, 1), false)
	if err == nil {
		t.Fatal("err = nil, want error")
	}
}

func TestCheckStockSufficient_OK(t *testing.T) {
	if err := CheckStockSufficient(big.NewRat(5, 1), big.NewRat(-3, 1)); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestCheckStockSufficient_Insufficient(t *testing.T) {
	err := CheckStockSufficient(big.NewRat(5, 1), big.NewRat(-10, 1))
	if !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("err = %v, want ErrInsufficientStock", err)
	}
}

func TestCheckStockSufficient_EmptyOK(t *testing.T) {
	if err := CheckStockSufficient(big.NewRat(0, 1), big.NewRat(5, 1)); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}
