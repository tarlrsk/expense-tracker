package transactions

import (
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	transactionslockport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/lock"
	transactionsremoveport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/remove"
	transactionsremoveproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/remove"
)

// NewRemove builds the delete-transaction use case.
func NewRemove(deps registry.Deps) transactionsremoveproc.Processor {
	return transactionsremoveproc.New(deps.UserTx, transactionsremoveproc.Ports{
		Lock:   transactionslockport.NewPG(),
		Remove: transactionsremoveport.NewPG(),
	})
}
