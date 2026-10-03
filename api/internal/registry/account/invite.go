package account

import (
	accountinviteorch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/invite"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewInvite builds the invite use case.
func NewInvite(deps registry.Deps, auth tx.Auth) accountinviteorch.Orchestrator {
	return accountinviteorch.New(auth, NewCreateAccount(deps, auth), NewMakeLink(deps, auth),
		deps.Mailer, deps.Config.WebBaseURL, deps.Logger)
}
