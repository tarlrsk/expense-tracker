package categorization

import (
	categorizationcreatetransactionorch "github.com/tarlrsk/expense-tracker/api/internal/categorization/orchestrator/createtransaction"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	transactionsreg "github.com/tarlrsk/expense-tracker/api/internal/registry/transactions"
)

// NewCreateTransaction builds the create-transaction use case with merchant-rule learning: the
// transactions module's create processor (wired by registry/transactions) and learn.
func NewCreateTransaction(deps registry.Deps) categorizationcreatetransactionorch.Orchestrator {
	return categorizationcreatetransactionorch.New(deps.UserTx, transactionsreg.NewCreate(deps), NewLearn(deps))
}
