package reserve

import (
	"context"
	"fmt"
	"time"

	categorieslistport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/list"
	smartentryreserveport "github.com/tarlrsk/expense-tracker/api/internal/smartentry/port/reserve"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries reserve uses. Categories is the categories module's read port (ADR-0032
// lets a processor use read ports of earlier modules).
type Ports struct {
	Usage      smartentryreserveport.Port
	Categories categorieslistport.Port
}

type processor struct {
	user  tx.User
	now   func() time.Time
	zone  *time.Location
	limit int
	ports Ports
}

// New returns the reserve use case. now is the clock and zone the app time zone; together they
// say which day is counted (ADR-0042). limit is AI_DAILY_PARSE_LIMIT.
func New(user tx.User, now func() time.Time, zone *time.Location, limit int, ports Ports) Processor {
	return &processor{user: user, now: now, zone: zone, limit: limit, ports: ports}
}

// Execute reserves first, so a request at the limit reads nothing more. A failure after the
// reservation rolls it back with the rest of the transaction.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	resp := Response{Day: transactionsdomain.Today(p.now(), p.zone)}
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		_, reserved, err := p.ports.Usage.Reserve(ctx, req.UserID, resp.Day, p.limit)
		if err != nil || !reserved {
			return err
		}
		all, err := p.ports.Categories.List(ctx, req.UserID)
		if err != nil {
			return err
		}
		resp.Reserved = true
		for _, c := range all {
			if !c.Archived {
				resp.Categories = append(resp.Categories, c)
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, fmt.Errorf("reserve an AI parse: %w", err)
	}
	return resp, nil
}
