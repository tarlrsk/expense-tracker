package categorization

import (
	categorizationupdatetransactionorch "github.com/tarlrsk/expense-tracker/api/internal/categorization/orchestrator/updatetransaction"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	transactionsreg "github.com/tarlrsk/expense-tracker/api/internal/registry/transactions"
)

// NewUpdateTransaction builds the update-transaction use case with merchant-rule learning: the
// transactions module's update processor (wired by registry/transactions) and learn.
func NewUpdateTransaction(deps registry.Deps) categorizationupdatetransactionorch.Orchestrator {
	return categorizationupdatetransactionorch.New(deps.UserTx, transactionsreg.NewUpdate(deps), NewLearn(deps))
}
