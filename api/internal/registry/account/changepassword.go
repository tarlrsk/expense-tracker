package account

import (
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountgetcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getcredentials"
	accountinsertattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertattempt"
	accountremoveattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeattempt"
	accountremoveoldattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeoldattempts"
	accountremovesessionsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removesessions"
	accountupdatepasswordport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatepassword"
	accountchangepasswordproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/changepassword"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewChangePassword builds the change-password use case.
func NewChangePassword(deps registry.Deps, auth tx.Auth) accountchangepasswordproc.Processor {
	return accountchangepasswordproc.New(auth, accountchangepasswordproc.Ports{
		GetCredentials:    accountgetcredentialsport.NewPG(),
		CountAttempts:     accountcountattemptsport.NewPG(),
		InsertAttempt:     accountinsertattemptport.NewPG(),
		RemoveAttempt:     accountremoveattemptport.NewPG(),
		RemoveOldAttempts: accountremoveoldattemptsport.NewPG(),
		UpdatePassword:    accountupdatepasswordport.NewPG(),
		RemoveSessions:    accountremovesessionsport.NewPG(),
	}, hasher(), deps.Clock)
}
