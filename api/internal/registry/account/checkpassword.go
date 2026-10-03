package account

import (
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountgetcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getcredentials"
	accountinsertattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertattempt"
	accountremoveattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeattempt"
	accountremoveoldattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeoldattempts"
	accountcheckpasswordproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/checkpassword"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewCheckPassword builds the check-password use case, limited like a login (ADR-0066).
func NewCheckPassword(deps registry.Deps, auth tx.Auth) accountcheckpasswordproc.Processor {
	return accountcheckpasswordproc.New(auth, accountcheckpasswordproc.Ports{
		GetCredentials:    accountgetcredentialsport.NewPG(),
		CountAttempts:     accountcountattemptsport.NewPG(),
		InsertAttempt:     accountinsertattemptport.NewPG(),
		RemoveAttempt:     accountremoveattemptport.NewPG(),
		RemoveOldAttempts: accountremoveoldattemptsport.NewPG(),
	}, hasher(), deps.Clock)
}
