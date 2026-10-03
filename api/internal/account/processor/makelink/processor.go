package makelink

import (
	"context"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountgetcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getcredentials"
	accountinsertlinkport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertlink"
	accountremovelinksport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removelinks"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries makelink uses.
type Ports struct {
	GetCredentials accountgetcredentialsport.Port
	RemoveLinks    accountremovelinksport.Port
	InsertLink     accountinsertlinkport.Port
}

type processor struct {
	auth  tx.Auth
	ports Ports
	now   func() time.Time
}

// New returns the make-link use case.
func New(auth tx.Auth, ports Ports, now func() time.Time) Processor {
	return &processor{auth: auth, ports: ports, now: now}
}

// Execute reads the user's email, deletes their earlier links and stores the new one, valid for
// domain.LinkLifetime, in one auth transaction (or the caller's, which it joins).
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	token := domain.NewToken()
	now := p.now()
	var email string
	found := false
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		cred, ok, err := p.ports.GetCredentials.Get(ctx, req.UserID)
		if err != nil || !ok {
			return err
		}
		found, email = true, cred.Email
		if err := p.ports.RemoveLinks.Remove(ctx, req.UserID); err != nil {
			return err
		}
		return p.ports.InsertLink.Insert(ctx, accountinsertlinkport.NewLink{
			UserID: req.UserID, TokenHash: token.Hash, Now: now, ExpiresAt: now.Add(domain.LinkLifetime),
		})
	})
	if err != nil {
		return Response{}, fmt.Errorf("make link: %w", err)
	}
	if !found {
		return Response{}, apperr.New(apperr.NotFound, domain.NoSuchUserMessage)
	}
	return Response{Email: email, Token: token.Plain}, nil
}
