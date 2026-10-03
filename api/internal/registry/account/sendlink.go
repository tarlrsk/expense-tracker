package account

import (
	accountsendlinkorch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/sendlink"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewSendLink builds the send-a-new-link use case.
func NewSendLink(deps registry.Deps, auth tx.Auth) accountsendlinkorch.Orchestrator {
	return accountsendlinkorch.New(NewMakeLink(deps, auth), deps.Mailer, deps.Config.WebBaseURL, deps.Logger)
}
