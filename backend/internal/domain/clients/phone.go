package clients

import (
	"errors"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// DefaultRegion is Argentina. Allows callers to pass numbers like "11 1234-5678"
// without a country prefix.
const DefaultRegion = "AR"

var (
	ErrPhoneEmpty   = errors.New("phone is empty")
	ErrPhoneInvalid = errors.New("phone is not a valid number")
)

// NormalizeE164 returns the input phone in canonical +E.164 form (e.g. "+5491112345678").
// Empty input returns ErrPhoneEmpty so callers can distinguish "not provided" from "bad".
func NormalizeE164(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrPhoneEmpty
	}
	num, err := phonenumbers.Parse(raw, DefaultRegion)
	if err != nil {
		return "", ErrPhoneInvalid
	}
	if !phonenumbers.IsValidNumber(num) {
		return "", ErrPhoneInvalid
	}
	return phonenumbers.Format(num, phonenumbers.E164), nil
}
