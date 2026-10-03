package checksession

import (
	"context"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountfindsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findsession"
	accounttouchsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/touchsession"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type processor struct {
	auth  tx.Auth
	find  accountfindsessionport.Port
	touch accounttouchsessionport.Port
	now   func() time.Time
}

// New returns the session check.
func New(auth tx.Auth, find accountfindsessionport.Port, touch accounttouchsessionport.Port, now func() time.Time) Processor {
	return &processor{auth: auth, find: find, touch: touch, now: now}
}

func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	hash, ok := domain.HashToken(req.Token)
	if !ok {
		return Response{}, unauthenticated()
	}
	now := p.now()
	staleBefore := now.Add(-domain.SessionTouchInterval)

	var resp Response
	found := false
	err := p.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		s, ok, err := p.find.Find(ctx, hash, now)
		if err != nil || !ok {
			return err
		}
		found = true
		resp = Response{UserID: s.UserID, SessionID: s.ID, Role: s.Role}
		if !s.LastUsedAt.Before(staleBefore) {
			return nil
		}
		return p.touch.Touch(ctx, accounttouchsessionport.Touch{
			SessionID: s.ID, Now: now, ExpiresAt: now.Add(domain.SessionLifetime), StaleBefore: staleBefore,
		})
	})
	if err != nil {
		return Response{}, fmt.Errorf("check session: %w", err)
	}
	if !found {
		return Response{}, unauthenticated()
	}
	return resp, nil
}

func unauthenticated() error {
	return apperr.New(apperr.Unauthenticated, domain.UnauthenticatedMessage)
}
