// Package smartentry wires the smart entry module's use cases and routes. Every use case runs as
// app_user through deps.UserTx; the module never gets the auth transactor (ADR-0032, ADR-0034).
package smartentry

import (
	categorizationfindport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	smartentryparseproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/parse"
)

// NewParse builds the local-parse use case. Today is read from deps.Clock in the app time zone
// (ADR-0042).
func NewParse(deps registry.Deps) smartentryparseproc.Processor {
	return smartentryparseproc.New(deps.UserTx, deps.Clock, deps.Config.AppTimeZone, smartentryparseproc.Ports{
		Rule: categorizationfindport.NewPG(),
	})
}
