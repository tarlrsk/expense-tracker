package account

import (
	accountcountoperatorsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countoperators"
	accountgetroleport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getrole"
	accountlockoperatorsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/lockoperators"
	accountremoveuserport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeuser"
	accountremoveaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/removeaccount"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewRemoveAccount builds the remove-account use case (operator removal and delete my account).
func NewRemoveAccount(_ registry.Deps, auth tx.Auth) accountremoveaccountproc.Processor {
	return accountremoveaccountproc.New(auth, accountremoveaccountproc.Ports{
		LockOperators:  accountlockoperatorsport.NewPG(),
		GetRole:        accountgetroleport.NewPG(),
		CountOperators: accountcountoperatorsport.NewPG(),
		RemoveUser:     accountremoveuserport.NewPG(),
	})
}
