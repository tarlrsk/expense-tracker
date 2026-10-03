package account

import (
	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
	accountgetprofileproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/getprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewGetProfile builds the get-profile use case.
func NewGetProfile(_ registry.Deps, auth tx.Auth) accountgetprofileproc.Processor {
	return accountgetprofileproc.New(auth, accountgetprofileport.NewPG())
}
