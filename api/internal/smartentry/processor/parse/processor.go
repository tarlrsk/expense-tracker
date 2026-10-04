package parse

import (
	"context"
	"fmt"
	"time"

	categorizationdomain "github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	categorizationfindport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/smartentry/domain"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries parse uses. Rule is the categorization module's read port (ADR-0032 lets
// a processor use read ports of earlier modules).
type Ports struct {
	Rule categorizationfindport.Port
}

type processor struct {
	user  tx.User
	now   func() time.Time
	zone  *time.Location
	ports Ports
}

// New returns the local-parse use case. now is the clock and zone the app time zone; together
// they say which day is today (ADR-0042).
func New(user tx.User, now func() time.Time, zone *time.Location, ports Ports) Processor {
	return &processor{user: user, now: now, zone: zone, ports: ports}
}

// Execute parses the text, then, when a complete item's merchant has a key, looks the keys up in
// one user transaction (each key once).
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	parsed := domain.Parse(req.Text, transactionsdomain.Today(p.now(), p.zone))
	items := make([]Item, len(parsed))
	keys := map[string][]int{} // merchant key -> the items that have it
	var order []string
	for i, it := range parsed {
		items[i] = Item{Text: it.Text, Amount: it.Amount, HasAmount: it.HasAmount, OccurredOn: it.OccurredOn, Merchant: it.Merchant}
		if !it.Complete {
			continue
		}
		key := categorizationdomain.MerchantKey(it.Merchant)
		if key == "" {
			continue
		}
		if _, seen := keys[key]; !seen {
			order = append(order, key)
		}
		keys[key] = append(keys[key], i)
	}
	if len(order) == 0 {
		return Response{Items: items}, nil
	}

	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		for _, key := range order {
			rule, found, err := p.ports.Rule.Find(ctx, req.UserID, key)
			if err != nil {
				return err
			}
			if !found {
				continue
			}
			for _, i := range keys[key] {
				items[i].Merchant, items[i].CategoryID, items[i].Resolved = rule.Merchant, rule.CategoryID, true
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, fmt.Errorf("parse quick entry: %w", err)
	}
	return Response{Items: items}, nil
}
