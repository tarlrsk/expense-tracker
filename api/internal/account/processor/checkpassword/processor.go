package checkpassword

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountgetcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getcredentials"
	accountinsertattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertattempt"
	accountremoveattemptport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeattempt"
	accountremoveoldattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removeoldattempts"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries checkpassword uses: the same as change password's (ADR-0066).
type Ports struct {
	GetCredentials    accountgetcredentialsport.Port
	CountAttempts     accountcountattemptsport.Port
	InsertAttempt     accountinsertattemptport.Port
	RemoveAttempt     accountremoveattemptport.Port
	RemoveOldAttempts accountremoveoldattemptsport.Port
}

type processor struct {
	auth   tx.Auth
	ports  Ports
	hasher *domain.Hasher
	now    func() time.Time
}

// New returns the check-password use case.
func New(auth tx.Auth, ports Ports, hasher *domain.Hasher, now func() time.Time) Processor {
	return &processor{auth: auth, ports: ports, hasher: hasher, now: now}
}

// Execute runs like the first half of a password change: the first transaction counts failures
// for the user's email and the address (429 at the limit) and records this attempt as a failure;
// the password is checked outside any transaction; the second transaction either cleans old
// attempts (wrong password: 400 after the commit) or removes this attempt again.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	ip, err := netip.ParseAddr(req.IP)
	if err != nil {
		return Response{}, apperr.Wrap(apperr.Internal, "", errors.New("check password: the client address is not an IP address"))
	}
	ip = ip.Unmap()
	now := p.now()

	var (
		found     bool
		limited   bool
		cred      accountgetcredentialsport.Credentials
		attemptID uuid.UUID
	)
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		var err error
		if cred, found, err = p.ports.GetCredentials.Get(ctx, req.UserID); err != nil || !found {
			return err
		}
		counts, err := p.ports.CountAttempts.Count(ctx, cred.Email, ip, now.Add(-domain.LoginWindow))
		if err != nil {
			return err
		}
		if domain.Limited(counts.Email, counts.IP) {
			limited = true
			return nil
		}
		attemptID, err = p.ports.InsertAttempt.Insert(ctx, cred.Email, ip, now)
		return err
	})
	switch {
	case err != nil:
		return Response{}, fmt.Errorf("check password: %w", err)
	case !found:
		return Response{}, apperr.New(apperr.Unauthenticated, domain.UnauthenticatedMessage)
	case limited:
		return Response{}, apperr.New(apperr.RateLimited, domain.RateLimitedMessage)
	}

	match, err := p.hasher.VerifyOrDummy(cred.PasswordHash, req.Password)
	if err != nil {
		return Response{}, fmt.Errorf("check password: %w", err)
	}
	if !match {
		err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
			return p.ports.RemoveOldAttempts.Remove(ctx, now.Add(-domain.AttemptRetention))
		})
		if err != nil {
			return Response{}, fmt.Errorf("check password: %w", err)
		}
		return Response{}, apperr.New(apperr.InvalidInput, domain.WrongPasswordMessage)
	}

	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		return p.ports.RemoveAttempt.Remove(ctx, attemptID)
	})
	if err != nil {
		return Response{}, fmt.Errorf("check password: %w", err)
	}
	return Response{}, nil
}
