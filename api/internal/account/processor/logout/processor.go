package logout

import (
	"context"
	"fmt"

	accountremovesessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removesession"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type processor struct {
	auth   tx.Auth
	remove accountremovesessionport.Port
}

// New returns the logout use case.
func New(auth tx.Auth, remove accountremovesessionport.Port) Processor {
	return &processor{auth: auth, remove: remove}
}

func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		return p.remove.Remove(ctx, req.SessionID)
	})
	if err != nil {
		return Response{}, fmt.Errorf("logout: %w", err)
	}
	return Response{}, nil
}
