package login

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountfindcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findcredentials"
	accountinsertattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertattempt"
	accountinsertsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertsession"
	accountremoveattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeattempts"
	accountremoveoldattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeoldattempts"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries login uses.
type Ports struct {
	CountAttempts     accountcountattemptsport.Port
	FindCredentials   accountfindcredentialsport.Port
	InsertAttempt     accountinsertattemptport.Port
	RemoveAttempts    accountremoveattemptsport.Port
	RemoveOldAttempts accountremoveoldattemptsport.Port
	InsertSession     accountinsertsessionport.Port
}

type processor struct {
	auth   tx.Auth
	ports  Ports
	hasher *domain.Hasher
	now    func() time.Time
}

// New returns the login use case.
func New(auth tx.Auth, ports Ports, hasher *domain.Hasher, now func() time.Time) Processor {
	return &processor{auth: auth, ports: ports, hasher: hasher, now: now}
}

// Execute runs two auth transactions with the password check between them, so no connection is
// held while hashing (ADR-0066):
//
//  1. count the failures in the window (at the limit: 429, nothing recorded), find the account,
//     and record this attempt as a failure before the password is checked, so parallel guesses
//     are all counted and an abandoned request still counts;
//  2. on failure, delete attempts older than a day and answer 401 after the commit (ADR-0032:
//     the stored rejection is not an error inside the transaction); on success, delete the
//     email's attempts (this one included) and create the session.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	email, err := domain.NormalizeEmail(req.Email)
	if err != nil {
		return Response{}, err
	}
	ip, err := netip.ParseAddr(req.IP)
	if err != nil {
		return Response{}, apperr.Wrap(apperr.Internal, "", errors.New("login: the client address is not an IP address"))
	}
	ip = ip.Unmap()
	now := p.now()

	var (
		limited bool
		found   bool
		cred    accountfindcredentialsport.Credentials
	)
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		counts, err := p.ports.CountAttempts.Count(ctx, email, ip, now.Add(-domain.LoginWindow))
		if err != nil {
			return err
		}
		if domain.Limited(counts.Email, counts.IP) {
			limited = true
			return nil
		}
		if cred, found, err = p.ports.FindCredentials.Find(ctx, email); err != nil {
			return err
		}
		_, err = p.ports.InsertAttempt.Insert(ctx, email, ip, now)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("login: %w", err)
	}
	if limited {
		return Response{}, apperr.New(apperr.RateLimited, domain.RateLimitedMessage)
	}

	// The same hashing work whether or not there is a real hash (unknown email, invite not
	// accepted): VerifyOrDummy checks against a dummy hash then.
	match, err := p.hasher.VerifyOrDummy(cred.PasswordHash, req.Password)
	if err != nil {
		return Response{}, fmt.Errorf("login: %w", err)
	}

	if !found || !match || cred.Disabled {
		err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
			return p.ports.RemoveOldAttempts.Remove(ctx, now.Add(-domain.AttemptRetention))
		})
		if err != nil {
			return Response{}, fmt.Errorf("login: %w", err)
		}
		return Response{}, apperr.New(apperr.Unauthenticated, domain.LoginFailedMessage)
	}

	token := domain.NewToken()
	expiresAt := now.Add(domain.SessionLifetime)
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		if err := p.ports.RemoveAttempts.Remove(ctx, email); err != nil {
			return err
		}
		_, err := p.ports.InsertSession.Insert(ctx, accountinsertsessionport.NewSession{
			UserID: cred.UserID, TokenHash: token.Hash, Now: now, ExpiresAt: expiresAt,
		})
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("login: %w", err)
	}
	return Response{Token: token.Plain, ExpiresAt: expiresAt}, nil
}
