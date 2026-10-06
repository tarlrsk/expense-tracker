// Package dashboard wires the dashboard module's use cases and routes. Every use case runs as
// app_user through deps.UserTx; the module never gets the auth transactor (ADR-0032,
// ADR-0034).
package dashboard

import (
	dashboardhandler "github.com/tarlrsk/expense-tracker/api/internal/handler/dashboard"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the dashboard module's routes (ADR-0088, ADR-0089).
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Authed.GET("/dashboard", dashboardhandler.Overview(NewOverview(deps)))
}
