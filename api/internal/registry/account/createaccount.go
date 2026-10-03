package account

import (
	accountinsertprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertprofile"
	accountinsertuserport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertuser"
	accountcreateaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/createaccount"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewCreateAccount builds the one account-creation use case (ADR-0036); the invite and the
// operator command both use it.
func NewCreateAccount(_ registry.Deps, auth tx.Auth) accountcreateaccountproc.Processor {
	return accountcreateaccountproc.New(auth, accountcreateaccountproc.Ports{
		InsertUser:    accountinsertuserport.NewPG(),
		InsertProfile: accountinsertprofileport.NewPG(),
	})
}
