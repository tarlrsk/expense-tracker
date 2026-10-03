package account

import (
	accountfindsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findsession"
	accounttouchsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/touchsession"
	accountchecksessionproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/checksession"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewCheckSession builds the session check, without HTTP: app wraps it in middleware.Session,
// and other callers (the operator command, T6) can use the same wiring.
func NewCheckSession(deps registry.Deps, auth tx.Auth) accountchecksessionproc.Processor {
	return accountchecksessionproc.New(auth, accountfindsessionport.NewPG(), accounttouchsessionport.NewPG(), deps.Clock)
}
