package account

import (
	accountaddoperatororch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/addoperator"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewAddOperator builds the operator command's use case (ADR-0035), without HTTP: app.RunOperator
// calls it.
func NewAddOperator(deps registry.Deps, auth tx.Auth) accountaddoperatororch.Orchestrator {
	return accountaddoperatororch.New(NewCreateAccount(deps, auth), NewPromote(deps, auth), NewMakeLink(deps, auth),
		deps.Mailer, deps.Config.WebBaseURL)
}
