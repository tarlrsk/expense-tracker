package updateprofile

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
	accountupdatedisplaynameport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatedisplayname"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries updateprofile uses.
type Ports struct {
	UpdateDisplayName accountupdatedisplaynameport.Port
	GetProfile        accountgetprofileport.Port
}

type processor struct {
	user  tx.User
	auth  tx.Auth
	ports Ports
}

// New returns the update-profile use case.
func New(user tx.User, auth tx.Auth, ports Ports) Processor {
	return &processor{user: user, auth: auth, ports: ports}
}

// Execute checks the display name, writes it as app_user for the caller (row-level security and
// the column grant apply), then reads the profile back as app_auth (ADR-0068). The two
// transactions run one after the other, never nested.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	if req.DisplayName != nil {
		name, err := domain.NormalizeDisplayName(*req.DisplayName)
		if err != nil {
			return Response{}, err
		}
		updated := false
		err = p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
			var err error
			updated, err = p.ports.UpdateDisplayName.Update(ctx, req.UserID, name)
			return err
		})
		if err != nil {
			return Response{}, fmt.Errorf("update profile: %w", err)
		}
		if !updated {
			return Response{}, unauthenticated()
		}
	}

	var (
		prof  accountgetprofileport.Profile
		found bool
	)
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		var err error
		prof, found, err = p.ports.GetProfile.Get(ctx, req.UserID)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("update profile: %w", err)
	}
	if !found {
		return Response{}, unauthenticated()
	}
	return Response{Profile: prof}, nil
}

func unauthenticated() error {
	return apperr.New(apperr.Unauthenticated, domain.UnauthenticatedMessage)
}
