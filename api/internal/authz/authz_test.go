package authz

import (
	"testing"

	"github.com/google/uuid"
)

func TestOwnerRules(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	tests := []struct {
		name          string
		caller, owner uuid.UUID
		want          bool
	}{
		{name: "own data", caller: a, owner: a, want: true},
		{name: "another user's data", caller: b, owner: a, want: false},
		{name: "no caller", caller: uuid.Nil, owner: a, want: false},
		{name: "no owner", caller: a, owner: uuid.Nil, want: false},
		{name: "neither", caller: uuid.Nil, owner: uuid.Nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanReadTransaction(tt.caller, tt.owner); got != tt.want {
				t.Errorf("CanReadTransaction = %v, want %v", got, tt.want)
			}
			if got := CanChangeTransaction(tt.caller, tt.owner); got != tt.want {
				t.Errorf("CanChangeTransaction = %v, want %v", got, tt.want)
			}
			if got := CanReadTotals(tt.caller, tt.owner); got != tt.want {
				t.Errorf("CanReadTotals = %v, want %v", got, tt.want)
			}
		})
	}
}
