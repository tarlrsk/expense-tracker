package setpassword

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountfindlinkport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findlink"
	accountinsertsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertsession"
	accountremovesessionsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removesessions"
	accountupdatepasswordport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatepassword"
	accountuselinkport "github.com/tarlrsk/expense-tracker/api/internal/account/port/uselink"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries setpassword uses.
type Ports struct {
	FindLink       accountfindlinkport.Port
	UseLink        accountuselinkport.Port
	UpdatePassword accountupdatepasswordport.Port
	RemoveSessions accountremovesessionsport.Port
	InsertSession  accountinsertsessionport.Port
}

type processor struct {
	auth   tx.Auth
	ports  Ports
	hasher *domain.Hasher
	now    func() time.Time
}

// New returns the set-password use case.
func New(auth tx.Auth, ports Ports, hasher *domain.Hasher, now func() time.Time) Processor {
	return &processor{auth: auth, ports: ports, hasher: hasher, now: now}
}

// errNoUser: the link's user has no users row to update (cannot happen while the foreign key
// cascades; checked so the transaction never commits half a change).
var errNoUser = errors.New("set password: the link's user does not exist")

// Execute checks the password rule first, then the link cheaply (no password is hashed for a bad
// link), hashes outside any transaction, and in a second transaction uses the link up under its
// row lock, writes the hash, ends all of the user's sessions and creates the new one. Two
// concurrent uses of one link give one success.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	if err := domain.CheckPasswordRule(req.Password); err != nil {
		return Response{}, err
	}
	linkHash, ok := domain.HashToken(req.Token)
	if !ok {
		return Response{}, invalidLink()
	}

	usable := false
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		var err error
		usable, err = p.ports.FindLink.Usable(ctx, linkHash, p.now())
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("set password: %w", err)
	}
	if !usable {
		return Response{}, invalidLink()
	}

	passwordHash := p.hasher.Hash(req.Password)
	session := domain.NewToken()
	now := p.now()
	expiresAt := now.Add(domain.SessionLifetime)

	used := false
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		userID, ok, err := p.ports.UseLink.Use(ctx, linkHash, now)
		if err != nil || !ok {
			return err // !ok: used, expired or disabled meanwhile; nothing was written
		}
		used = true
		updated, err := p.ports.UpdatePassword.Update(ctx, accountupdatepasswordport.Update{UserID: userID, Hash: passwordHash})
		if err != nil {
			return err
		}
		if !updated {
			return errNoUser
		}
		if err := p.ports.RemoveSessions.Remove(ctx, userID, uuid.Nil); err != nil {
			return err
		}
		_, err = p.ports.InsertSession.Insert(ctx, accountinsertsessionport.NewSession{
			UserID: userID, TokenHash: session.Hash, Now: now, ExpiresAt: expiresAt,
		})
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("set password: %w", err)
	}
	if !used {
		return Response{}, invalidLink()
	}
	return Response{Token: session.Plain, ExpiresAt: expiresAt}, nil
}

func invalidLink() error {
	return apperr.New(apperr.InvalidInput, domain.LinkInvalidMessage)
}
