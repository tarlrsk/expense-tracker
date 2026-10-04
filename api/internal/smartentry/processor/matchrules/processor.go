package matchrules

import (
	"context"
	"fmt"

	categorizationdomain "github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	categorizationfindport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries matchrules uses. Rule is the categorization module's read port (ADR-0032
// lets a processor use read ports of earlier modules).
type Ports struct {
	Rule categorizationfindport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the match-rules use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute groups the merchants by key and looks each key up once.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	resp := Response{Matches: make([]Match, len(req.Merchants))}
	keys := map[string][]int{} // merchant key -> the merchants that have it
	var order []string
	for i, m := range req.Merchants {
		key := categorizationdomain.MerchantKey(m)
		if key == "" {
			continue
		}
		if _, seen := keys[key]; !seen {
			order = append(order, key)
		}
		keys[key] = append(keys[key], i)
	}
	if len(order) == 0 {
		return resp, nil
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
				resp.Matches[i] = Match{Found: true, Rule: rule}
			}
		}
		return nil
	})
	if err != nil {
		return Response{}, fmt.Errorf("match merchant rules: %w", err)
	}
	return resp, nil
}
