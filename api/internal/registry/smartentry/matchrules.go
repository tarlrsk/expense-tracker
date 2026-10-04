package smartentry

import (
	categorizationfindport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	smartentrymatchrulesproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/matchrules"
)

// NewMatchRules builds the match-merchant-rules use case.
func NewMatchRules(deps registry.Deps) smartentrymatchrulesproc.Processor {
	return smartentrymatchrulesproc.New(deps.UserTx, smartentrymatchrulesproc.Ports{
		Rule: categorizationfindport.NewPG(),
	})
}
