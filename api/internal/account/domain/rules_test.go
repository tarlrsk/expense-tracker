package domain

import (
	"strings"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestCheckNewEmail(t *testing.T) {
	long := strings.Repeat("a", MaxEmailLength-len("@example.test")) + "@example.test" // exactly 254
	tests := []struct {
		in, want string
		ok       bool
	}{
		{in: "ann@example.test", want: "ann@example.test", ok: true},
		{in: "  Ann.Lee+tag@Example.TEST \n", want: "Ann.Lee+tag@Example.TEST", ok: true},
		{in: long, want: long, ok: true},
		{in: "x" + long},
		{in: ""},
		{in: "   "},
		{in: "plain"},
		{in: "@example.test"},
		{in: "ann@"},
		{in: "ann@@example.test"},
		{in: "Ann <ann@example.test>"},
		{in: "<ann@example.test>"},
		{in: `"Ann" <ann@example.test>`},
		{in: "ann@example.test, bob@example.test"},
		{in: "ann@example.test bob@example.test"},
		{in: "ann@example.test (Ann)"},
		{in: "ann @example.test"},
	}
	for _, tt := range tests {
		got, err := CheckNewEmail(tt.in)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Errorf("CheckNewEmail(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
			continue
		}
		if apperr.KindOf(err) != apperr.InvalidInput || !strings.Contains(err.Error(), InviteEmailMessage) {
			t.Errorf("CheckNewEmail(%q) = %q, %v; want invalid_input", tt.in, got, err)
		}
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{in: "", want: "", ok: true},
		{in: "   ", want: "", ok: true},
		{in: "  Ann Lee \t", want: "Ann Lee", ok: true},
		{in: strings.Repeat("ก", 50), want: strings.Repeat("ก", 50), ok: true}, // runes, not bytes
		{in: " " + strings.Repeat("x", 50) + " ", want: strings.Repeat("x", 50), ok: true},
		{in: strings.Repeat("ก", 51)},
		{in: strings.Repeat("x", 51)},
	}
	for _, tt := range tests {
		got, err := NormalizeDisplayName(tt.in)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Errorf("NormalizeDisplayName(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
			continue
		}
		if apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("NormalizeDisplayName(%d runes) = %v; want invalid_input", len([]rune(tt.in)), err)
		}
	}
}

func TestSetPasswordLinkAndText(t *testing.T) {
	tok := NewToken().Plain
	for _, base := range []string{"https://satang.example", "https://satang.example/"} {
		if got, want := SetPasswordLink(base, tok), "https://satang.example/set-password#token="+tok; got != want {
			t.Errorf("SetPasswordLink(%q) = %q, want %q", base, got, want)
		}
	}
	text := SetPasswordText("LINK")
	for _, want := range []string{"\nLINK\n", "7 days", "works once"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

func TestStatusOf(t *testing.T) {
	if StatusOf(false) != StatusInvited || StatusOf(true) != StatusActive {
		t.Errorf("StatusOf(false) = %q, StatusOf(true) = %q", StatusOf(false), StatusOf(true))
	}
}
