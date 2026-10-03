package reorder

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categorieslistport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/list"
	categorieslockport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/lock"
	categoriessetorderport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/setorder"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries reorder uses.
type Ports struct {
	Lock     categorieslockport.Port
	List     categorieslistport.Port
	SetOrder categoriessetorderport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the reorder use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute checks the list, then in one user transaction locks the caller's categories, compares
// the list with the current non-archived set and, only when they match, numbers the listed
// categories 1 to n in the given order. Archived categories keep their numbers. A list that does
// not match (a category added, archived or unarchived since the client loaded it, or an id that
// is not the caller's) is a conflict and writes nothing (ADR-0069).
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	if req.IDs == nil {
		return Response{}, apperr.New(apperr.InvalidInput, domain.IDsRequiredMessage)
	}
	if repeats(req.IDs) {
		return Response{}, apperr.New(apperr.InvalidInput, domain.RepeatedIDMessage)
	}

	var (
		resp    Response
		changed bool
	)
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		if err := p.ports.Lock.Lock(ctx, req.UserID); err != nil {
			return err
		}
		current, err := p.ports.List.List(ctx, req.UserID)
		if err != nil {
			return err
		}
		if !sameActiveSet(current, req.IDs) {
			changed = true
			return nil
		}
		if err := p.ports.SetOrder.SetOrder(ctx, req.UserID, req.IDs); err != nil {
			return err
		}
		resp.Categories, err = p.ports.List.List(ctx, req.UserID)
		return err
	})
	switch {
	case err != nil:
		return Response{}, fmt.Errorf("reorder categories: %w", err)
	case changed:
		return Response{}, apperr.New(apperr.Conflict, domain.OrderChangedMessage)
	}
	return resp, nil
}

// repeats reports whether an id appears more than once.
func repeats(ids []uuid.UUID) bool {
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

// sameActiveSet reports whether ids, which has no repeats, are exactly the non-archived
// categories of current.
func sameActiveSet(current []domain.Category, ids []uuid.UUID) bool {
	active := make(map[uuid.UUID]bool, len(current))
	for _, c := range current {
		if !c.Archived {
			active[c.ID] = true
		}
	}
	if len(active) != len(ids) {
		return false
	}
	for _, id := range ids {
		if !active[id] {
			return false
		}
	}
	return true
}
