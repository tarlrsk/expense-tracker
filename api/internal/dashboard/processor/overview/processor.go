package overview

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/authz"
	"github.com/tarlrsk/expense-tracker/api/internal/dashboard/domain"
	dashboardcategorybreakdownport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/categorybreakdown"
	dashboardmonthtotalsport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/monthtotals"
	dashboardtopdescriptionsport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/topdescriptions"
	dashboardtrendport "github.com/tarlrsk/expense-tracker/api/internal/dashboard/port/trend"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries overview uses.
type Ports struct {
	Totals    dashboardmonthtotalsport.Port
	Breakdown dashboardcategorybreakdownport.Port
	Trend     dashboardtrendport.Port
	Top       dashboardtopdescriptionsport.Port
}

type processor struct {
	user  tx.User
	now   func() time.Time
	zone  *time.Location
	ports Ports
}

// New returns the dashboard overview use case. now is the clock and zone the app time zone;
// together they say which month is the current one (ADR-0042).
func New(user tx.User, now func() time.Time, zone *time.Location, ports Ports) Processor {
	return &processor{user: user, now: now, zone: zone, ports: ports}
}

// errNotReadable: authz does not let the caller read the totals of their own scope. Today it
// cannot happen (the scope is always the caller's own data); it is a fault, never a filter.
var errNotReadable = errors.New("dashboard overview: the caller may not read the totals of this scope")

// Execute checks the month (the current one when none was sent), then runs the four queries in
// one user transaction for the caller (row-level security applies), so the blocks agree with
// each other. The scope is the caller's own data.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	current := domain.CurrentMonth(p.now(), p.zone)
	month := current
	if req.Month != nil {
		m, err := domain.ParseMonth(*req.Month, current)
		if err != nil {
			return Response{}, err
		}
		month = m
	}
	scope := domain.OwnerScope(req.UserID)
	if !authz.CanReadTotals(req.UserID, scope.OwnerID) {
		return Response{}, errNotReadable
	}

	resp := Response{Month: month}
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		var err error
		if resp.Totals, err = p.ports.Totals.Totals(ctx, scope, month); err != nil {
			return fmt.Errorf("month totals: %w", err)
		}
		if resp.Categories, err = p.ports.Breakdown.Breakdown(ctx, scope, month); err != nil {
			return fmt.Errorf("category breakdown: %w", err)
		}
		if resp.Trend, err = p.ports.Trend.Trend(ctx, scope, month, domain.TrendMonths); err != nil {
			return fmt.Errorf("trend: %w", err)
		}
		if resp.TopDescriptions, err = p.ports.Top.Top(ctx, scope, month, domain.TopDescriptionsLimit); err != nil {
			return fmt.Errorf("top descriptions: %w", err)
		}
		return nil
	})
	if err != nil {
		return Response{}, fmt.Errorf("dashboard overview: %w", err)
	}
	return resp, nil
}
