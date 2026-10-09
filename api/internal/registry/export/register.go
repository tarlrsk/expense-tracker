// Package export wires the export module's use cases and routes. Every use case runs as app_user
// through deps.UserTx; the module never gets the auth transactor (ADR-0032, ADR-0034).
package export

import (
	exporthandler "github.com/tarlrsk/expense-tracker/api/internal/handler/export"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the export module's routes (ADR-0087, ADR-0092).
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Authed.GET("/export.csv", exporthandler.TransactionsCSV(NewTransactionsCSV(deps)))
}
