package dashboard

import (
	dashboardcategorybreakdownport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/categorybreakdown"
	dashboardmonthtotalsport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/monthtotals"
	dashboardtopdescriptionsport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/topdescriptions"
	dashboardtrendport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/trend"
	dashboardoverviewproc "github.com/tarlrsk/expense-tracker/api/internal/dashboard/processor/overview"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// NewOverview builds the dashboard overview use case. The current month is read from deps.Clock
// in the app time zone (ADR-0042).
func NewOverview(deps registry.Deps) dashboardoverviewproc.Processor {
	return dashboardoverviewproc.New(deps.UserTx, deps.Clock, deps.Config.AppTimeZone, dashboardoverviewproc.Ports{
		Totals:    dashboardmonthtotalsport.NewPG(),
		Breakdown: dashboardcategorybreakdownport.NewPG(),
		Trend:     dashboardtrendport.NewPG(),
		Top:       dashboardtopdescriptionsport.NewPG(),
	})
}
