package transactions

import (
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	transactionsfindport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/find"
	transactionsinsertport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/insert"
	transactionscreateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/create"
)

// NewCreate builds the create-transaction use case. Today is read from deps.Clock in the app
// time zone (ADR-0042).
func NewCreate(deps registry.Deps) transactionscreateproc.Processor {
	return transactionscreateproc.New(deps.UserTx, deps.Clock, deps.Config.AppTimeZone, transactionscreateproc.Ports{
		Find:     transactionsfindport.NewPG(),
		Insert:   transactionsinsertport.NewPG(),
		Category: categoriesfindport.NewPG(),
	})
}
