// Package health wires the health module's use cases and routes.
package health

import (
	healthpingport "github.com/tarlrsk/expense-tracker/api/internal/health/port/ping"
	healthcheckproc "github.com/tarlrsk/expense-tracker/api/internal/health/processor/check"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewCheck builds the liveness use case.
func NewCheck(_ registry.Deps) healthcheckproc.Processor {
	return healthcheckproc.New(healthpingport.NewStatic())
}
