package account

import (
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountfindcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findcredentials"
	accountinsertattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertattempt"
	accountinsertsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertsession"
	accountremoveattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeattempts"
	accountremoveoldattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeoldattempts"
	accountloginproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/login"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewLogin builds the login use case.
func NewLogin(deps registry.Deps, auth tx.Auth) accountloginproc.Processor {
	return accountloginproc.New(auth, accountloginproc.Ports{
		CountAttempts:     accountcountattemptsport.NewPG(),
		FindCredentials:   accountfindcredentialsport.NewPG(),
		InsertAttempt:     accountinsertattemptport.NewPG(),
		RemoveAttempts:    accountremoveattemptsport.NewPG(),
		RemoveOldAttempts: accountremoveoldattemptsport.NewPG(),
		InsertSession:     accountinsertsessionport.NewPG(),
	}, hasher(), deps.Clock)
}
