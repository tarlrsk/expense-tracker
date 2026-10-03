package getprofile

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type processor struct {
	auth tx.Auth
	get  accountgetprofileport.Port
}

// New returns the get-profile use case.
func New(auth tx.Auth, get accountgetprofileport.Port) Processor {
	return &processor{auth: auth, get: get}
}

// Execute reads the profile as app_auth: it includes users.email, which app_user cannot read
// (ADR-0068). The id comes from the session check, never from the client.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	var (
		prof  accountgetprofileport.Profile
		found bool
	)
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		var err error
		prof, found, err = p.get.Get(ctx, req.UserID)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("get profile: %w", err)
	}
	if !found {
		return Response{}, apperr.New(apperr.Unauthenticated, domain.UnauthenticatedMessage)
	}
	return Response{Profile: prof}, nil
}
