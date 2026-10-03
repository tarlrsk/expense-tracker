package account

import (
	accountremovesessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removesession"
	accountlogoutproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/logout"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewLogout builds the logout use case.
func NewLogout(_ registry.Deps, auth tx.Auth) accountlogoutproc.Processor {
	return accountlogoutproc.New(auth, accountremovesessionport.NewPG())
}
