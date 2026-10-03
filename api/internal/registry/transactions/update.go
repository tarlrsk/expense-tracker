package transactions

import (
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	transactionslockport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/lock"
	transactionsupdateport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/update"
	transactionsupdateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/update"
)

// NewUpdate builds the update-transaction use case. Today is read from deps.Clock in the app
// time zone (ADR-0042).
func NewUpdate(deps registry.Deps) transactionsupdateproc.Processor {
	return transactionsupdateproc.New(deps.UserTx, deps.Clock, deps.Config.AppTimeZone, transactionsupdateproc.Ports{
		Lock:     transactionslockport.NewPG(),
		Update:   transactionsupdateport.NewPG(),
		Category: categoriesfindport.NewPG(),
	})
}
