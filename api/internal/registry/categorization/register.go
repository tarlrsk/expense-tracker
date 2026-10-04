// Package categorization wires the categorization module's use cases and routes. Every use case
// runs as app_user through deps.UserTx; the module never gets the auth transactor (ADR-0032,
// ADR-0034).
package categorization

import (
	transactionshandler "github.com/tarlrsk/expense-tracker/api/internal/handler/transactions"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the routes whose use cases live in categorization: creating and updating a
// transaction, which learn merchant rules (ADR-0078). They are transactions endpoints with the
// transactions handlers (ADR-0071); only the use case behind them is this module's.
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Authed.POST("/transactions", transactionshandler.Create(NewCreateTransaction(deps)))
	routes.Authed.PATCH("/transactions/:id", transactionshandler.Update(NewUpdateTransaction(deps)))
}
