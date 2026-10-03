// Package transactions wires the transactions module's use cases and routes. Every use case runs
// as app_user through deps.UserTx; the module never gets the auth transactor (ADR-0032,
// ADR-0034).
package transactions

import (
	transactionshandler "github.com/tarlrsk/expense-tracker/api/internal/handler/transactions"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the transactions module's routes (ADR-0071).
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Authed.GET("/transactions", transactionshandler.List(NewList(deps)))
	routes.Authed.POST("/transactions", transactionshandler.Create(NewCreate(deps)))
	routes.Authed.PATCH("/transactions/:id", transactionshandler.Update(NewUpdate(deps)))
	routes.Authed.DELETE("/transactions/:id", transactionshandler.Remove(NewRemove(deps)))
}
