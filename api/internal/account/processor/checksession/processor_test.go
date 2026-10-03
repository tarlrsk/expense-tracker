package checksession

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountfindsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findsession"
	accounttouchsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/touchsession"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type findFn func(ctx context.Context, hash []byte, now time.Time) (accountfindsessionport.Session, bool, error)

func (f findFn) Find(ctx context.Context, h []byte, n time.Time) (accountfindsessionport.Session, bool, error) {
	return f(ctx, h, n)
}

type touchFn func(ctx context.Context, t accounttouchsessionport.Touch) error

func (f touchFn) Touch(ctx context.Context, t accounttouchsessionport.Touch) error { return f(ctx, t) }

func TestExecute(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	token := domain.NewToken()
	session := accountfindsessionport.Session{ID: uuid.New(), UserID: uuid.New(), Role: domain.RoleOperator}
	tests := []struct {
		name      string
		token     string
		lastUsed  time.Time
		found     bool
		wantKind  apperr.Kind
		wantTouch bool
		wantTx    int
	}{
		{name: "used 2 hours ago: touched", token: token.Plain, lastUsed: now.Add(-2 * time.Hour), found: true, wantTouch: true, wantTx: 1},
		{name: "used 61 minutes ago: touched", token: token.Plain, lastUsed: now.Add(-61 * time.Minute), found: true, wantTouch: true, wantTx: 1},
		{name: "used exactly an hour ago: not touched", token: token.Plain, lastUsed: now.Add(-time.Hour), found: true, wantTx: 1},
		{name: "used 5 minutes ago: not touched", token: token.Plain, lastUsed: now.Add(-5 * time.Minute), found: true, wantTx: 1},
		{name: "unknown or expired", token: token.Plain, wantKind: apperr.Unauthenticated, wantTx: 1},
		{name: "malformed: no transaction", token: "abc", wantKind: apperr.Unauthenticated},
		{name: "empty: no transaction", token: "", wantKind: apperr.Unauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := txtest.New()
			var touched *accounttouchsessionport.Touch
			p := New(auth,
				findFn(func(_ context.Context, hash []byte, at time.Time) (accountfindsessionport.Session, bool, error) {
					if string(hash) != string(token.Hash) || !at.Equal(now) {
						t.Errorf("Find(%x, %s)", hash, at)
					}
					s := session
					s.LastUsedAt = tt.lastUsed
					return s, tt.found, nil
				}),
				touchFn(func(_ context.Context, tc accounttouchsessionport.Touch) error {
					touched = &tc
					return nil
				}),
				func() time.Time { return now })
			resp, err := p.Execute(t.Context(), Request{Token: tt.token})
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if n := len(auth.Records()); n != tt.wantTx {
				t.Errorf("transactions = %d, want %d", n, tt.wantTx)
			}
			if (touched != nil) != tt.wantTouch {
				t.Fatalf("touched = %v, want %v", touched != nil, tt.wantTouch)
			}
			if touched != nil {
				want := accounttouchsessionport.Touch{
					SessionID: session.ID, Now: now, ExpiresAt: now.Add(30 * 24 * time.Hour), StaleBefore: now.Add(-time.Hour),
				}
				if *touched != want {
					t.Errorf("touch = %+v, want %+v", *touched, want)
				}
			}
			if err == nil && resp != (Response{UserID: session.UserID, SessionID: session.ID, Role: domain.RoleOperator}) {
				t.Errorf("response = %+v", resp)
			}
		})
	}
}
