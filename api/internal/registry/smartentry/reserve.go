package smartentry

import (
	categorieslistport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/list"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	smartentryreserveport "github.com/tarlrsk/expense-tracker/api/internal/smartentry/port/reserve"
	smartentryreserveproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/reserve"
)

// NewReserve builds the reserve-an-AI-parse use case with AI_DAILY_PARSE_LIMIT. The day is read
// from deps.Clock in the app time zone (ADR-0042).
func NewReserve(deps registry.Deps) smartentryreserveproc.Processor {
	return smartentryreserveproc.New(deps.UserTx, deps.Clock, deps.Config.AppTimeZone, deps.Config.AI.DailyParseLimit,
		smartentryreserveproc.Ports{
			Usage:      smartentryreserveport.NewPG(),
			Categories: categorieslistport.NewPG(),
		})
}
