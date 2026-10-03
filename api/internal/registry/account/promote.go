package account

import (
	accountfindcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findcredentials"
	accountsetroleport "github.com/tarlrsk/expense-tracker/api/internal/account/port/setrole"
	accountpromoteproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/promote"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewPromote builds the promote use case (operator command only, ADR-0035).
func NewPromote(_ registry.Deps, auth tx.Auth) accountpromoteproc.Processor {
	return accountpromoteproc.New(auth, accountpromoteproc.Ports{
		FindCredentials: accountfindcredentialsport.NewPG(),
		SetRole:         accountsetroleport.NewPG(),
	})
}
