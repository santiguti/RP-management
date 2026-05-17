package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	enc, err := Hash("hunter2")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$") {
		t.Fatalf("unexpected encoded prefix: %q", enc)
	}
	ok, err := Verify("hunter2", enc)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("expected match")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	enc, err := Hash("hunter2")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, err := Verify("hunter3", enc)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch for wrong password")
	}
}

func TestVerifyMalformed(t *testing.T) {
	cases := []string{
		"",
		"not-argon",
		"$argon2id$",
		"$argon2id$v=19$m=65536,t=3,p=2$not_base64!!$xxxx",
		"$argon2id$v=19$m=65536,t=3,p=2$xxxx$not_base64!!",
		"$bcrypt$v=19$m=65536,t=3,p=2$AAAA$BBBB",
	}
	for _, c := range cases {
		_, err := Verify("anything", c)
		if err == nil || !errors.Is(err, ErrInvalidHash) {
			t.Errorf("expected ErrInvalidHash for %q, got %v", c, err)
		}
	}
}

func TestHashSaltsAreUnique(t *testing.T) {
	a, _ := Hash("same-password")
	b, _ := Hash("same-password")
	if a == b {
		t.Fatal("two hashes of the same password should differ (salt should be random)")
	}
}
