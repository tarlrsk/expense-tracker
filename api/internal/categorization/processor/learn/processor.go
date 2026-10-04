package learn

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	categorizationupsertport "github.com/tarlrsk/expense-tracker/api/internal/categorization/port/upsert"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries learn uses. Category is the categories module's read port (ADR-0032
// lets a processor use read ports of earlier modules).
type Ports struct {
	Upsert   categorizationupsertport.Port
	Category categoriesfindport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the learn-merchant-rule use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute computes the merchant key and, when there is one, in one user transaction (joining the
// caller's open one) checks the category is one of the caller's non-archived ones and upserts the
// rule. Without a key it opens no transaction.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	merchant := strings.TrimSpace(req.Merchant)
	key := domain.MerchantKey(merchant)
	if key == "" || utf8.RuneCountInString(merchant) > domain.MaxLength {
		return Response{}, nil
	}

	var learnt bool
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		cat, found, err := p.ports.Category.Find(ctx, req.UserID, req.CategoryID)
		if err != nil || !found || cat.Archived {
			return err
		}
		_, err = p.ports.Upsert.Upsert(ctx, categorizationupsertport.NewRule{
			OwnerID: req.UserID, MerchantKey: key, Merchant: merchant, CategoryID: req.CategoryID,
		})
		learnt = err == nil
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("learn merchant rule: %w", err)
	}
	return Response{Learnt: learnt}, nil
}
