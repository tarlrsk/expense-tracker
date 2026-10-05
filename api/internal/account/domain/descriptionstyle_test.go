package domain

import (
	"errors"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestParseDescriptionStyle(t *testing.T) {
	tests := []struct {
		in   string
		want DescriptionStyle
		ok   bool
	}{
		{in: "as_typed", want: DescriptionStyleAsTyped, ok: true},
		{in: "light_tidy", want: DescriptionStyleLightTidy, ok: true},
		{in: "clean_name", want: DescriptionStyleCleanName, ok: true},
		{in: ""},
		{in: " as_typed"},
		{in: "As_Typed"},
		{in: "as typed"},
		{in: "fancy"},
	}
	for _, tt := range tests {
		got, err := ParseDescriptionStyle(tt.in)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Errorf("ParseDescriptionStyle(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
			continue
		}
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind != apperr.InvalidInput || ae.Message != DescriptionStyleRuleMessage || got != "" {
			t.Errorf("ParseDescriptionStyle(%q) = %q, %v; want invalid_input", tt.in, got, err)
		}
	}
	if DefaultDescriptionStyle != DescriptionStyleAsTyped {
		t.Errorf("default = %q, want as_typed", DefaultDescriptionStyle)
	}
}
