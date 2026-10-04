package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

func TestCheckText(t *testing.T) {
	tests := []struct {
		name, in, want string
		ok             bool
	}{
		{"trimmed", "  grab 145\n", "grab 145", true},
		{"one character", "x", "x", true},
		{"1,000 Thai characters", strings.Repeat("ก", 1000), strings.Repeat("ก", 1000), true},
		{"1,000 characters once trimmed", " \n" + strings.Repeat("a", 1000) + "\t ", strings.Repeat("a", 1000), true},
		{"line breaks kept inside", "coffee 60\ngrab 145", "coffee 60\ngrab 145", true},
		{"empty", "", "", false},
		{"blank", " \n\t ", "", false},
		{"1,001 characters", strings.Repeat("ก", 1001), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckText(tt.in)
			if tt.ok != (err == nil) || got != tt.want {
				t.Fatalf("CheckText = %q, %v", got, err)
			}
			var ae *apperr.Error
			if !tt.ok && (!errors.As(err, &ae) || ae.Kind != apperr.InvalidInput || ae.Message != TextRuleMessage) {
				t.Errorf("error = %v, want invalid_input with the text message", err)
			}
		})
	}
}
