package promote

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountfindcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findcredentials"
	accountsetroleport "github.com/tarlrsk/expense-tracker/api/internal/account/port/setrole"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries promote uses.
type Ports struct {
	FindCredentials accountfindcredentialsport.Port
	SetRole         accountsetroleport.Port
}

type processor struct {
	auth  tx.Auth
	ports Ports
}

// New returns the promote use case.
func New(auth tx.Auth, ports Ports) Processor {
	return &processor{auth: auth, ports: ports}
}

// Execute finds the account and sets its role in one auth transaction.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	email, err := domain.NormalizeEmail(req.Email)
	if err != nil {
		return Response{}, err
	}
	var resp Response
	found := false
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		cred, ok, err := p.ports.FindCredentials.Find(ctx, email)
		if err != nil || !ok {
			return err
		}
		found = true
		resp.UserID, resp.HasPassword = cred.UserID, cred.PasswordHash != ""
		resp.Changed, err = p.ports.SetRole.Set(ctx, cred.UserID, domain.RoleOperator)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("promote: %w", err)
	}
	if !found {
		return Response{}, apperr.New(apperr.NotFound, domain.NoSuchUserMessage)
	}
	return resp, nil
}
