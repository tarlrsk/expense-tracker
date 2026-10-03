package domain

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

var cheap = HashParams{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32}

func TestHasherDefaultFormat(t *testing.T) {
	h := NewHasher(DefaultHashParams)
	encoded := h.Hash("correct horse battery")
	format := regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)
	if !format.MatchString(encoded) {
		t.Fatalf("hash %q is not the standard argon2id string with the default parameters", encoded)
	}
	if ok, err := h.Verify(encoded, "correct horse battery"); !ok || err != nil {
		t.Errorf("Verify(right) = %v, %v", ok, err)
	}
}

func TestHasher(t *testing.T) {
	h := NewHasher(cheap)
	encoded := h.Hash("correct horse")
	if again := h.Hash("correct horse"); again == encoded {
		t.Error("two hashes of one password are equal: no fresh salt")
	}

	// Parameters are read from the stored string: a hasher with other parameters verifies it.
	other := NewHasher(HashParams{Memory: 128, Time: 2, Threads: 2, SaltLength: 8, KeyLength: 16})
	tests := []struct {
		name    string
		encoded string
		pw      string
		want    bool
		wantErr error
	}{
		{name: "right", encoded: encoded, pw: "correct horse", want: true},
		{name: "wrong", encoded: encoded, pw: "correct horsE"},
		{name: "empty password", encoded: encoded, pw: ""},
		{name: "other parameters", encoded: other.Hash("x y z"), pw: "x y z", want: true},
		{name: "empty hash", encoded: "", pw: "x", wantErr: ErrBadHash},
		{name: "bcrypt", encoded: "$2a$10$abcdefghijklmnopqrstuv", pw: "x", wantErr: ErrBadHash},
		{name: "argon2i", encoded: strings.Replace(encoded, "argon2id", "argon2i", 1), pw: "x", wantErr: ErrBadHash},
		{name: "other version", encoded: strings.Replace(encoded, "v=19", "v=16", 1), pw: "x", wantErr: ErrBadHash},
		{name: "zero memory", encoded: strings.Replace(encoded, "m=64", "m=0", 1), pw: "x", wantErr: ErrBadHash},
		{name: "huge memory", encoded: strings.Replace(encoded, "m=64", "m=99999999", 1), pw: "x", wantErr: ErrBadHash},
		{name: "bad salt", encoded: strings.Replace(encoded, "$", "$!", 4), pw: "x", wantErr: ErrBadHash},
		{name: "missing key", encoded: encoded[:strings.LastIndex(encoded, "$")], pw: "x", wantErr: ErrBadHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.Verify(tt.encoded, tt.pw)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Errorf("Verify = %v, %v; want %v, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}

	// No real hash: checked against the dummy, never a match.
	if ok, err := h.VerifyOrDummy("", "anything"); ok || err != nil {
		t.Errorf("VerifyOrDummy(\"\") = %v, %v; want false, nil", ok, err)
	}
	if ok, err := h.VerifyOrDummy(encoded, "correct horse"); !ok || err != nil {
		t.Errorf("VerifyOrDummy(real) = %v, %v; want true, nil", ok, err)
	}
}

func TestCheckPasswordRule(t *testing.T) {
	tests := []struct {
		pw   string
		want bool
	}{
		{pw: "", want: false},
		{pw: "123456789", want: false},
		{pw: "1234567890", want: true},
		{pw: strings.Repeat("a", 128), want: true},
		{pw: strings.Repeat("a", 129), want: false},
		{pw: strings.Repeat("ข", 10), want: true},   // 10 characters, 30 bytes
		{pw: strings.Repeat("ข", 128), want: true},  // 384 bytes
		{pw: strings.Repeat("ข", 129), want: false}, // counted in characters
		{pw: "          ", want: true},              // no composition rules
	}
	for _, tt := range tests {
		err := CheckPasswordRule(tt.pw)
		if (err == nil) != tt.want {
			t.Errorf("CheckPasswordRule(%d bytes) = %v, want ok %v", len(tt.pw), err, tt.want)
		}
		if err != nil && apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("kind = %s, want invalid_input", apperr.KindOf(err))
		}
	}
}

func TestToken(t *testing.T) {
	a, b := NewToken(), NewToken()
	if len(a.Plain) != 43 || a.Plain == b.Plain || bytes.Equal(a.Hash, b.Hash) || len(a.Hash) != 32 {
		t.Fatalf("tokens %q %q: want two different 43-character tokens with 32-byte hashes", a.Plain, b.Plain)
	}
	if strings.ContainsAny(a.Plain, "+/=") {
		t.Errorf("token %q is not unpadded base64url", a.Plain)
	}
	hash, ok := HashToken(a.Plain)
	if !ok || !bytes.Equal(hash, a.Hash) {
		t.Errorf("HashToken(plain) = %x, %v; want the stored hash", hash, ok)
	}
	for _, bad := range []string{"", "abc", a.Plain[:42], a.Plain + "A", a.Plain[:42] + "=", a.Plain[:42] + "+", a.Plain[:42] + " "} {
		if _, ok := HashToken(bad); ok {
			t.Errorf("HashToken(%q) ok, want not ok", bad)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{in: "  Person@Example.com \t", want: "Person@Example.com", ok: true},
		{in: "", ok: false},
		{in: "   ", ok: false},
		{in: strings.Repeat("a", 242) + "@example.com", want: strings.Repeat("a", 242) + "@example.com", ok: true}, // 254
		{in: strings.Repeat("a", 243) + "@example.com", ok: false},                                                 // 255
	}
	for _, tt := range tests {
		got, err := NormalizeEmail(tt.in)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("NormalizeEmail(%d chars) = %q, %v; want %q, ok %v", len(tt.in), got, err, tt.want, tt.ok)
		}
	}
}

func TestLimited(t *testing.T) {
	tests := []struct {
		email, ip int
		want      bool
	}{
		{0, 0, false}, {4, 19, false}, {5, 0, true}, {0, 20, true}, {6, 25, true},
	}
	for _, tt := range tests {
		if got := Limited(tt.email, tt.ip); got != tt.want {
			t.Errorf("Limited(%d, %d) = %v, want %v", tt.email, tt.ip, got, tt.want)
		}
	}
}
