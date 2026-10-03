package update

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	categorieslockport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/lock"
	categoriesupdateport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/update"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries update uses.
type Ports struct {
	Lock   categorieslockport.Port
	Find   categoriesfindport.Port
	Update categoriesupdateport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the update-category use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute checks the input, then in one user transaction locks the caller's categories, reads
// the category and writes only what differs from it (ADR-0069):
//   - archiving keeps the category's sort_order;
//   - unarchiving moves it to the end of the list;
//   - archived set to the value it has, or a name or icon equal to the current one, writes nothing.
//
// A taken name on a rename or an unarchive, also by a write racing this one, is a conflict: the
// unique name index decides, and it covers only non-archived categories, so renaming an archived
// category is never refused. Another user's id is not found, like an unknown one: row-level
// security hides the row.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	if req.Name == nil && req.Icon == nil && req.Archived == nil {
		return Response{}, apperr.New(apperr.InvalidInput, domain.NoChangeMessage)
	}
	var name, icon *string
	if req.Name != nil {
		n, err := domain.NormalizeName(*req.Name)
		if err != nil {
			return Response{}, err
		}
		name = &n
	}
	if req.Icon != nil {
		i, err := domain.NormalizeIcon(*req.Icon)
		if err != nil {
			return Response{}, err
		}
		icon = &i
	}

	var (
		cat   domain.Category
		found bool
	)
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		if err := p.ports.Lock.Lock(ctx, req.UserID); err != nil {
			return err
		}
		var err error
		cat, found, err = p.ports.Find.Find(ctx, req.UserID, req.ID)
		if err != nil || !found {
			return err
		}
		ch, changed := changes(cat, name, icon, req.Archived)
		if !changed {
			return nil
		}
		cat, found, err = p.ports.Update.Update(ctx, req.UserID, req.ID, ch)
		return err
	})
	switch {
	case errors.Is(err, categoriesupdateport.ErrNameTaken):
		return Response{}, apperr.Wrap(apperr.Conflict, domain.NameTakenMessage, err)
	case err != nil:
		return Response{}, fmt.Errorf("update category: %w", err)
	case !found:
		return Response{}, apperr.New(apperr.NotFound, domain.NotFoundMessage)
	}
	return Response{Category: cat}, nil
}

// changes returns what differs between cur and the requested values, and whether anything does.
func changes(cur domain.Category, name, icon *string, archived *bool) (categoriesupdateport.Changes, bool) {
	var ch categoriesupdateport.Changes
	if name != nil && *name != cur.Name {
		ch.Name = name
	}
	if icon != nil && *icon != cur.Icon {
		ch.Icon = icon
	}
	if archived != nil && *archived != cur.Archived {
		ch.Archived = archived
		ch.ToEnd = !*archived
	}
	return ch, ch.Name != nil || ch.Icon != nil || ch.Archived != nil
}
