package removeaccount

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcountoperatorsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countoperators"
	accountgetroleport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getrole"
	accountlockoperatorsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/lockoperators"
	accountremoveuserport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeuser"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries removeaccount uses.
type Ports struct {
	LockOperators  accountlockoperatorsport.Port
	GetRole        accountgetroleport.Port
	CountOperators accountcountoperatorsport.Port
	RemoveUser     accountremoveuserport.Port
}

type processor struct {
	auth  tx.Auth
	ports Ports
}

// New returns the remove-account use case.
func New(auth tx.Auth, ports Ports) Processor {
	return &processor{auth: auth, ports: ports}
}

// errVanished: the users row read under the lock was gone at the delete (cannot happen while
// every removal takes the lock; checked so a removal never reports success for nothing).
var errVanished = errors.New("remove account: the user disappeared during the removal")

// Execute runs the check and the delete in one auth transaction that first takes a
// transaction-level lock shared by every removal, so two removals at the same moment run one
// after the other and the second sees the first's result: they cannot leave no operator.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	if req.OperatorID != uuid.Nil && req.OperatorID == req.UserID {
		return Response{}, apperr.New(apperr.Conflict, domain.RemoveSelfMessage)
	}
	var found, last bool
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		if err := p.ports.LockOperators.Lock(ctx); err != nil {
			return err
		}
		role, ok, err := p.ports.GetRole.Get(ctx, req.UserID)
		if err != nil || !ok {
			return err
		}
		found = true
		if role == domain.RoleOperator {
			n, err := p.ports.CountOperators.Count(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				last = true
				return nil
			}
		}
		removed, err := p.ports.RemoveUser.Remove(ctx, req.UserID)
		if err != nil {
			return err
		}
		if !removed {
			return errVanished
		}
		return nil
	})
	switch {
	case err != nil:
		return Response{}, fmt.Errorf("remove account: %w", err)
	case !found:
		return Response{}, apperr.New(apperr.NotFound, domain.NoSuchUserMessage)
	case last:
		return Response{}, apperr.New(apperr.Conflict, domain.LastOperatorMessage)
	}
	return Response{}, nil
}
