package app

import (
	"log/slog"
	"testing"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Deps.UserTx opens user transactions only: it cannot be turned into the auth transactor, which
// app hands to the account module alone (ADR-0032, ADR-0034).
func TestDepsUserTxIsNotAuth(t *testing.T) {
	deps := newDeps(config.Config{}, slog.New(slog.DiscardHandler), &db.DB{})
	if deps.UserTx == nil {
		t.Fatal("Deps.UserTx is nil")
	}
	if _, ok := deps.UserTx.(tx.Auth); ok {
		t.Error("Deps.UserTx implements tx.Auth")
	}
}
