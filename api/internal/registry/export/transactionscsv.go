package export

import (
	"github.com/tarlrsk/expense-tracker/api/internal/export/domain"
	exportlistport "github.com/tarlrsk/expense-tracker/api/internal/export/port/list"
	exporttransactionscsvproc "github.com/tarlrsk/expense-tracker/api/internal/export/processor/transactionscsv"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewTransactionsCSV builds the CSV export use case, reading domain.PageSize rows per user
// transaction.
func NewTransactionsCSV(deps registry.Deps) exporttransactionscsvproc.Processor {
	return exporttransactionscsvproc.New(deps.UserTx, domain.PageSize, exporttransactionscsvproc.Ports{
		List: exportlistport.NewPG(),
	})
}
