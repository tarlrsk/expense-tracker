package account

import (
	accountgetcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getcredentials"
	accountinsertlinkport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertlink"
	accountremovelinksport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removelinks"
	accountmakelinkproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/makelink"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewMakeLink builds the one path that makes a set-password link (ADR-0037, ADR-0068).
func NewMakeLink(deps registry.Deps, auth tx.Auth) accountmakelinkproc.Processor {
	return accountmakelinkproc.New(auth, accountmakelinkproc.Ports{
		GetCredentials: accountgetcredentialsport.NewPG(),
		RemoveLinks:    accountremovelinksport.NewPG(),
		InsertLink:     accountinsertlinkport.NewPG(),
	}, deps.Clock)
}
