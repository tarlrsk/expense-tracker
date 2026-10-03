package create

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categoriescountport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/count"
	categoriesinsertport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/insert"
	categorieslockport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/lock"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries create uses.
type Ports struct {
	Lock   categorieslockport.Port
	Count  categoriescountport.Port
	Insert categoriesinsertport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the create-category use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute checks the input, then in one user transaction locks the caller's categories, checks
// the limit and inserts the category at the end. A taken name, also by an insert racing this
// one, is a conflict: the unique name index decides (ADR-0069).
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	name, err := domain.NormalizeName(req.Name)
	if err != nil {
		return Response{}, err
	}
	kind, err := domain.ParseKind(req.Kind)
	if err != nil {
		return Response{}, err
	}
	icon, err := domain.NormalizeIcon(req.Icon)
	if err != nil {
		return Response{}, err
	}

	var (
		created domain.Category
		full    bool
	)
	err = p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		if err := p.ports.Lock.Lock(ctx, req.UserID); err != nil {
			return err
		}
		n, err := p.ports.Count.Count(ctx, req.UserID)
		if err != nil {
			return err
		}
		if n >= domain.MaxCategories {
			full = true
			return nil
		}
		created, err = p.ports.Insert.Insert(ctx, categoriesinsertport.NewCategory{
			OwnerID: req.UserID, Name: name, Icon: icon, Kind: kind,
		})
		return err
	})
	switch {
	case errors.Is(err, categoriesinsertport.ErrNameTaken):
		return Response{}, apperr.Wrap(apperr.Conflict, domain.NameTakenMessage, err)
	case err != nil:
		return Response{}, fmt.Errorf("create category: %w", err)
	case full:
		return Response{}, apperr.New(apperr.Conflict, domain.LimitMessage)
	}
	return Response{Category: created}, nil
}
