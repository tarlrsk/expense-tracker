package createaccount

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountinsertprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertprofile"
	accountinsertuserport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertuser"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries createaccount uses.
type Ports struct {
	InsertUser    accountinsertuserport.Port
	InsertProfile accountinsertprofileport.Port
}

type processor struct {
	auth  tx.Auth
	ports Ports
}

// New returns the account-creation use case.
func New(auth tx.Auth, ports Ports) Processor {
	return &processor{auth: auth, ports: ports}
}

// Execute checks the email, then inserts the users row and the profile in one auth transaction
// (or in the caller's, which it joins). An email in use, also by an insert racing this one, is a
// conflict: the unique key on users.email decides.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	email, err := domain.CheckNewEmail(req.Email)
	if err != nil {
		return Response{}, err
	}
	var user accountinsertuserport.NewUser
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		var err error
		if user, err = p.ports.InsertUser.Insert(ctx, email); err != nil {
			return err
		}
		return p.ports.InsertProfile.Insert(ctx, user.ID, domain.RoleUser)
	})
	if errors.Is(err, accountinsertuserport.ErrEmailTaken) {
		return Response{}, apperr.Wrap(apperr.Conflict, domain.EmailTakenMessage, err)
	}
	if err != nil {
		return Response{}, fmt.Errorf("create account: %w", err)
	}
	return Response{Account: domain.Account{
		ID: user.ID, Email: email, Role: domain.RoleUser, Status: domain.StatusInvited, CreatedAt: user.CreatedAt,
	}}, nil
}
