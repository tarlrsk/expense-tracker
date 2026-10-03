package changepassword

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
	accountremovesessionsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/removesessions"
	accountupdatepasswordport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatepassword"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries changepassword uses.
type Ports struct {
	GetCredentials    accountgetcredentialsport.Port
	CountAttempts     accountcountattemptsport.Port
	InsertAttempt     accountinsertattemptport.Port
	RemoveAttempt     accountremoveattemptport.Port
	RemoveOldAttempts accountremoveoldattemptsport.Port
	UpdatePassword    accountupdatepasswordport.Port
	RemoveSessions    accountremovesessionsport.Port
}

type processor struct {
	auth   tx.Auth
	ports  Ports
	hasher *domain.Hasher
	now    func() time.Time
}

// New returns the change-password use case.
func New(auth tx.Auth, ports Ports, hasher *domain.Hasher, now func() time.Time) Processor {
	return &processor{auth: auth, ports: ports, hasher: hasher, now: now}
}

// errChanged: the stored hash changed between the check and the write (a concurrent change).
var errChanged = errors.New("change password: the password changed meanwhile")

// Execute checks the new password's rule before any database work, then runs like a login: the
// first transaction counts failures for the user's email and the address (429 at the limit) and
// records this attempt as a failure; the current password is checked and the new one hashed
// outside any transaction; the second transaction either cleans old attempts (wrong password:
// 400 after the commit) or removes this attempt, writes the hash only if it is still the one
// checked, and ends the user's other sessions.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	if err := domain.CheckPasswordRule(req.NewPassword); err != nil {
		return Response{}, err
	}
	ip, err := netip.ParseAddr(req.IP)
	if err != nil {
		return Response{}, apperr.Wrap(apperr.Internal, "", errors.New("change password: the client address is not an IP address"))
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
		return Response{}, fmt.Errorf("change password: %w", err)
	case !found:
		return Response{}, apperr.New(apperr.Unauthenticated, domain.UnauthenticatedMessage)
	case limited:
		return Response{}, apperr.New(apperr.RateLimited, domain.RateLimitedMessage)
	}

	match, err := p.hasher.VerifyOrDummy(cred.PasswordHash, req.CurrentPassword)
	if err != nil {
		return Response{}, fmt.Errorf("change password: %w", err)
	}
	if !match {
		err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
			return p.ports.RemoveOldAttempts.Remove(ctx, now.Add(-domain.AttemptRetention))
		})
		if err != nil {
			return Response{}, fmt.Errorf("change password: %w", err)
		}
		return Response{}, wrongCurrent()
	}

	newHash := p.hasher.Hash(req.NewPassword)
	checked := cred.PasswordHash
	err = p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		if err := p.ports.RemoveAttempt.Remove(ctx, attemptID); err != nil {
			return err
		}
		updated, err := p.ports.UpdatePassword.Update(ctx, accountupdatepasswordport.Update{
			UserID: req.UserID, Hash: newHash, IfHash: &checked,
		})
		if err != nil {
			return err
		}
		if !updated {
			return errChanged // rolls back: this attempt stays counted
		}
		return p.ports.RemoveSessions.Remove(ctx, req.UserID, req.SessionID)
	})
	if errors.Is(err, errChanged) {
		return Response{}, wrongCurrent()
	}
	if err != nil {
		return Response{}, fmt.Errorf("change password: %w", err)
	}
	return Response{}, nil
}

func wrongCurrent() error {
	return apperr.New(apperr.InvalidInput, domain.WrongCurrentPasswordMessage)
}
