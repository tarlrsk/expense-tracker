package domain

import (
	"strings"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{name: "plain", in: "Food", want: "Food"},
		{name: "one character", in: "x", want: "x"},
		{name: "trimmed", in: "  Food \t", want: "Food"},
		{name: "inner runs collapsed", in: "Bills \t\n &   Utilities", want: "Bills & Utilities"},
		{name: "unicode spaces", in: " Pet 　food ", want: "Pet food"},
		{name: "thai", in: " ทำบุญ  ", want: "ทำบุญ"},
		{name: "50 characters", in: strings.Repeat("ก", 50), want: strings.Repeat("ก", 50)},
		{name: "50 after collapsing", in: strings.Repeat("a", 24) + "     " + strings.Repeat("b", 25), want: strings.Repeat("a", 24) + " " + strings.Repeat("b", 25)},
		{name: "emoji with joiner", in: "👨‍👩‍👧 Family", want: "👨‍👩‍👧 Family"},
		{name: "empty", in: "", wantErr: true},
		{name: "only spaces", in: " \t \n ", wantErr: true},
		{name: "51 characters", in: strings.Repeat("ก", 51), wantErr: true},
		{name: "control character", in: "Fo\x00od", wantErr: true},
		{name: "escape", in: "Food\x1b[31m", wantErr: true},
		{name: "delete", in: "Food\x7f", wantErr: true},
		{name: "c1 control", in: "Food\u0080", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeName(tt.in)
			if tt.wantErr {
				if apperr.KindOf(err) != apperr.InvalidInput {
					t.Errorf("NormalizeName(%q) = %q, %v; want invalid_input", tt.in, got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("NormalizeName(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestNormalizeIcon(t *testing.T) {
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{name: "empty clears", in: "", want: ""},
		{name: "spaces clear", in: "   ", want: ""},
		{name: "trimmed", in: " 🍜 ", want: "🍜"},
		{name: "32 characters", in: strings.Repeat("a", 32), want: strings.Repeat("a", 32)},
		{name: "33 characters", in: strings.Repeat("a", 33), wantErr: true},
		{name: "control character", in: "a\nb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeIcon(tt.in)
			if tt.wantErr {
				if apperr.KindOf(err) != apperr.InvalidInput {
					t.Errorf("NormalizeIcon(%q) = %q, %v; want invalid_input", tt.in, got, err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("NormalizeIcon(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestParseKind(t *testing.T) {
	for _, in := range []string{"expense", "income"} {
		if k, err := ParseKind(in); err != nil || string(k) != in {
			t.Errorf("ParseKind(%q) = %q, %v", in, k, err)
		}
	}
	for _, in := range []string{"", "Expense", "INCOME", "transfer", " expense"} {
		if _, err := ParseKind(in); apperr.KindOf(err) != apperr.InvalidInput {
			t.Errorf("ParseKind(%q) error = %v, want invalid_input", in, err)
		}
	}
}
