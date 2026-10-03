package remove

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/authz"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionslockport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/lock"
	transactionsremoveport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/remove"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries remove uses.
type Ports struct {
	Lock   transactionslockport.Port
	Remove transactionsremoveport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the delete-transaction use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute locks the transaction, asks authz whether the caller may change it and deletes it, in
// one user transaction. Another user's id is not found, like an unknown one: row-level security
// hides the row.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	var removed bool
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		t, found, err := p.ports.Lock.Lock(ctx, req.UserID, req.ID)
		if err != nil || !found || !authz.CanChangeTransaction(req.UserID, t.OwnerID) {
			return err
		}
		removed, err = p.ports.Remove.Remove(ctx, req.UserID, req.ID)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("remove transaction: %w", err)
	}
	if !removed {
		return Response{}, apperr.New(apperr.NotFound, domain.NotFoundMessage)
	}
	return Response{}, nil
}
