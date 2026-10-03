package listusers

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountlistusersport "github.com/tarlrsk/expense-tracker/api/internal/account/port/listusers"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type processor struct {
	auth tx.Auth
	list accountlistusersport.Port
}

// New returns the list-users use case.
func New(auth tx.Auth, list accountlistusersport.Port) Processor {
	return &processor{auth: auth, list: list}
}

func (p *processor) Execute(ctx context.Context, _ Request) (Response, error) {
	var users []domain.Account
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		var err error
		users, err = p.list.List(ctx)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("list users: %w", err)
	}
	return Response{Users: users}, nil
}
