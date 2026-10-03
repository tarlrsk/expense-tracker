package account

import (
	accountfindlinkport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findlink"
	accountinsertsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertsession"
	accountremovesessionsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removesessions"
	accountupdatepasswordport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatepassword"
	accountuselinkport "github.com/tarlrsk/expense-tracker/api/internal/account/port/uselink"
	accountsetpasswordproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/setpassword"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewSetPassword builds the set-password (accept invite) use case.
func NewSetPassword(deps registry.Deps, auth tx.Auth) accountsetpasswordproc.Processor {
	return accountsetpasswordproc.New(auth, accountsetpasswordproc.Ports{
		FindLink:       accountfindlinkport.NewPG(),
		UseLink:        accountuselinkport.NewPG(),
		UpdatePassword: accountupdatepasswordport.NewPG(),
		RemoveSessions: accountremovesessionsport.NewPG(),
		InsertSession:  accountinsertsessionport.NewPG(),
	}, hasher(), deps.Clock)
}
