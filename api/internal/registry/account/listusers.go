package account

import (
	accountlistusersport "github.com/tarlrsk/expense-tracker/api/internal/account/port/listusers"
	accountlistusersproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/listusers"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewListUsers builds the operator's user list.
func NewListUsers(_ registry.Deps, auth tx.Auth) accountlistusersproc.Processor {
	return accountlistusersproc.New(auth, accountlistusersport.NewPG())
}
