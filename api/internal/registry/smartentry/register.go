package smartentry

import (
	smartentryhandler "github.com/tarlrsk/expense-tracker/api/internal/handler/smartentry"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the smart entry module's routes (PLAN-0003 T5).
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Authed.POST("/entry/parse", smartentryhandler.Parse(NewParseEntry(deps)))
}
