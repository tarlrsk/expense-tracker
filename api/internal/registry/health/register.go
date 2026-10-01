package health

import (
	healthhandler "github.com/tarlrsk/expense-tracker/api/internal/handler/health"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the health module's routes.
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Public.GET("/healthz", healthhandler.Check(NewCheck(deps)))
}
