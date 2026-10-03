package account

import (
	accountdeletemeorch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/deleteme"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewDeleteMe builds the delete-my-account use case.
func NewDeleteMe(deps registry.Deps, auth tx.Auth) accountdeletemeorch.Orchestrator {
	return accountdeletemeorch.New(NewCheckPassword(deps, auth), NewRemoveAccount(deps, auth))
}
