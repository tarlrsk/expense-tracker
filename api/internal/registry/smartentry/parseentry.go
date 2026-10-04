package smartentry

import (
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	smartentryparseorch "github.com/tarlrsk/expense-tracker/api/internal/smartentry/orchestrator/parse"
)

// NewParseEntry builds quick entry: the local parse, the reservation, the AI (deps.AIParse) and
// the rule match.
func NewParseEntry(deps registry.Deps) smartentryparseorch.Orchestrator {
	return smartentryparseorch.New(NewParse(deps), NewReserve(deps), NewMatchRules(deps), deps.AIParse, deps.Logger)
}
