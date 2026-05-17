package clients

import (
	"errors"
	"testing"
)

func TestNormalizeE164(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr error
	}{
		{
			name: "argentine local format with implicit country",
			raw:  "11 1234-5678",
			want: "+541112345678",
		},
		{
			name: "argentine international spaced format",
			raw:  "+54 9 11 1234 5678",
			want: "+5491112345678",
		},
		{
			name: "already normalized argentine number",
			raw:  "+5491112345678",
			want: "+5491112345678",
		},
		{
			name:    "whitespace is empty",
			raw:     "   ",
			wantErr: ErrPhoneEmpty,
		},
		{
			name:    "empty string",
			raw:     "",
			wantErr: ErrPhoneEmpty,
		},
		{
			name:    "letters only",
			raw:     "abc",
			wantErr: ErrPhoneInvalid,
		},
		{
			name:    "invalid mixed us number",
			raw:     "+1 555 INVALID",
			wantErr: ErrPhoneInvalid,
		},
		{
			name: "valid us number",
			raw:  "+1 415 555 2671",
			want: "+14155552671",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeE164(tt.raw)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NormalizeE164(%q) error = %v, want %v", tt.raw, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeE164(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeE164(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
