package account

import (
	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
	accountupdatedisplaynameport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatedisplayname"
	accountupdateprofileproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/updateprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewUpdateProfile builds the update-profile use case: the write runs as app_user (deps.UserTx),
// the read-back as app_auth (ADR-0068).
func NewUpdateProfile(deps registry.Deps, auth tx.Auth) accountupdateprofileproc.Processor {
	return accountupdateprofileproc.New(deps.UserTx, auth, accountupdateprofileproc.Ports{
		UpdateDisplayName: accountupdatedisplaynameport.NewPG(),
		GetProfile:        accountgetprofileport.NewPG(),
	})
}
