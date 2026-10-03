package transactions

import (
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	transactionslistport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/list"
	transactionslistproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/list"
)

// NewList builds the list-transactions use case.
func NewList(deps registry.Deps) transactionslistproc.Processor {
	return transactionslistproc.New(deps.UserTx, transactionslistproc.Ports{
		List: transactionslistport.NewPG(),
	})
}
