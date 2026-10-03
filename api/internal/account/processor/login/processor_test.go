package login

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountfindcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/findcredentials"
	accountinsertsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertsession"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

var (
	hasher = domain.NewHasher(domain.HashParams{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32})
	now    = time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	errDB  = errors.New("database down")
)

// fakes records every port call with the number of the transaction it ran in.
type fakes struct {
	t       *testing.T
	auth    *txtest.Fake
	calls   []string
	counts  accountcountattemptsport.Counts
	cred    accountfindcredentialsport.Credentials
	found   bool
	failOn  string
	session accountinsertsessionport.NewSession
}

func (f *fakes) call(ctx context.Context, name string) error {
	f.t.Helper()
	if _, err := tx.Require(ctx, tx.RoleAuth); err != nil {
		f.t.Errorf("%s ran outside an auth transaction: %v", name, err)
	}
	f.calls = append(f.calls, name+"@"+strconv.Itoa(len(f.auth.Records())))
	if name == f.failOn {
		return errDB
	}
	return nil
}

type (
	countFn      func(ctx context.Context, email string, ip netip.Addr, since time.Time) (accountcountattemptsport.Counts, error)
	findFn       func(ctx context.Context, email string) (accountfindcredentialsport.Credentials, bool, error)
	attemptFn    func(ctx context.Context, email string, ip netip.Addr, at time.Time) (uuid.UUID, error)
	removeFn     func(ctx context.Context, email string) error
	removeOldFn  func(ctx context.Context, before time.Time) error
	insertSessFn func(ctx context.Context, s accountinsertsessionport.NewSession) (uuid.UUID, error)
)

func (f countFn) Count(ctx context.Context, e string, ip netip.Addr, s time.Time) (accountcountattemptsport.Counts, error) {
	return f(ctx, e, ip, s)
}
func (f findFn) Find(ctx context.Context, e string) (accountfindcredentialsport.Credentials, bool, error) {
	return f(ctx, e)
}
func (f attemptFn) Insert(ctx context.Context, e string, ip netip.Addr, at time.Time) (uuid.UUID, error) {
	return f(ctx, e, ip, at)
}
func (f removeFn) Remove(ctx context.Context, e string) error       { return f(ctx, e) }
func (f removeOldFn) Remove(ctx context.Context, b time.Time) error { return f(ctx, b) }
func (f insertSessFn) Insert(ctx context.Context, s accountinsertsessionport.NewSession) (uuid.UUID, error) {
	return f(ctx, s)
}

func (f *fakes) processor() Processor {
	return New(f.auth, Ports{
		CountAttempts: countFn(func(ctx context.Context, _ string, _ netip.Addr, since time.Time) (accountcountattemptsport.Counts, error) {
			if !since.Equal(now.Add(-domain.LoginWindow)) {
				f.t.Errorf("counted since %s", since)
			}
			return f.counts, f.call(ctx, "count")
		}),
		FindCredentials: findFn(func(ctx context.Context, _ string) (accountfindcredentialsport.Credentials, bool, error) {
			return f.cred, f.found, f.call(ctx, "find")
		}),
		InsertAttempt: attemptFn(func(ctx context.Context, _ string, _ netip.Addr, _ time.Time) (uuid.UUID, error) {
			return uuid.New(), f.call(ctx, "attempt")
		}),
		RemoveAttempts: removeFn(func(ctx context.Context, _ string) error { return f.call(ctx, "clear") }),
		RemoveOldAttempts: removeOldFn(func(ctx context.Context, before time.Time) error {
			if !before.Equal(now.Add(-domain.AttemptRetention)) {
				f.t.Errorf("removed attempts before %s", before)
			}
			return f.call(ctx, "cleanup")
		}),
		InsertSession: insertSessFn(func(ctx context.Context, s accountinsertsessionport.NewSession) (uuid.UUID, error) {
			f.session = s
			return uuid.New(), f.call(ctx, "session")
		}),
	}, hasher, func() time.Time { return now })
}

func TestExecute(t *testing.T) {
	userID := uuid.New()
	goodHash := hasher.Hash("right password")
	tests := []struct {
		name      string
		req       Request
		setup     func(*fakes)
		wantKind  apperr.Kind
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "success: attempt recorded first, then cleared with the session",
			req:  Request{Email: " a@example.test ", Password: "right password", IP: "192.0.2.1"},
			setup: func(f *fakes) {
				f.cred, f.found = accountfindcredentialsport.Credentials{UserID: userID, PasswordHash: goodHash}, true
			},
			wantCalls: []string{"count@1", "find@1", "attempt@1", "clear@2", "session@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "wrong password: the stored failure commits, then a 401",
			req:  Request{Email: "a@example.test", Password: "wrong", IP: "192.0.2.1"},
			setup: func(f *fakes) {
				f.cred, f.found = accountfindcredentialsport.Credentials{UserID: userID, PasswordHash: goodHash}, true
			},
			wantKind:  apperr.Unauthenticated,
			wantCalls: []string{"count@1", "find@1", "attempt@1", "cleanup@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "disabled account with the right password",
			req:  Request{Email: "a@example.test", Password: "right password", IP: "192.0.2.1"},
			setup: func(f *fakes) {
				f.cred, f.found = accountfindcredentialsport.Credentials{UserID: userID, PasswordHash: goodHash, Disabled: true}, true
			},
			wantKind:  apperr.Unauthenticated,
			wantCalls: []string{"count@1", "find@1", "attempt@1", "cleanup@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name:      "unknown email",
			req:       Request{Email: "nobody@example.test", Password: "x", IP: "2001:db8::1"},
			wantKind:  apperr.Unauthenticated,
			wantCalls: []string{"count@1", "find@1", "attempt@1", "cleanup@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name:      "limited: no lookup, no new row, no second transaction",
			req:       Request{Email: "a@example.test", Password: "right password", IP: "192.0.2.1"},
			setup:     func(f *fakes) { f.counts = accountcountattemptsport.Counts{Email: 5} },
			wantKind:  apperr.RateLimited,
			wantCalls: []string{"count@1"},
			wantTx:    []txtest.Outcome{txtest.Committed},
		},
		{
			name:      "limited by address",
			req:       Request{Email: "a@example.test", Password: "x", IP: "192.0.2.1"},
			setup:     func(f *fakes) { f.counts = accountcountattemptsport.Counts{IP: 20} },
			wantKind:  apperr.RateLimited,
			wantCalls: []string{"count@1"},
			wantTx:    []txtest.Outcome{txtest.Committed},
		},
		{
			name:     "empty email: nothing touches the database",
			req:      Request{Email: "  ", Password: "x", IP: "192.0.2.1"},
			wantKind: apperr.InvalidInput,
		},
		{
			name:     "no client address",
			req:      Request{Email: "a@example.test", Password: "x", IP: ""},
			wantKind: apperr.Internal,
		},
		{
			name:      "database error rolls back and is internal",
			req:       Request{Email: "a@example.test", Password: "x", IP: "192.0.2.1"},
			setup:     func(f *fakes) { f.failOn = "attempt" },
			wantKind:  apperr.Internal,
			wantCalls: []string{"count@1", "find@1", "attempt@1"},
			wantTx:    []txtest.Outcome{txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakes{t: t, auth: txtest.New()}
			if tt.setup != nil {
				tt.setup(f)
			}
			resp, err := f.processor().Execute(t.Context(), tt.req)
			if got := apperr.KindOf(err); got != tt.wantKind {
				t.Fatalf("kind = %q (%v), want %q", got, err, tt.wantKind)
			}
			if !slices.Equal(f.calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", f.calls, tt.wantCalls)
			}
			var outcomes []txtest.Outcome
			for _, r := range f.auth.Records() {
				outcomes = append(outcomes, r.Outcome)
			}
			if !slices.Equal(outcomes, tt.wantTx) {
				t.Errorf("transactions = %v, want %v", outcomes, tt.wantTx)
			}
			if err != nil {
				return
			}
			hash, ok := domain.HashToken(resp.Token)
			if !ok || string(hash) != string(f.session.TokenHash) || f.session.UserID != userID {
				t.Errorf("stored session %+v does not match the returned token", f.session)
			}
			if !resp.ExpiresAt.Equal(now.Add(domain.SessionLifetime)) || !f.session.ExpiresAt.Equal(resp.ExpiresAt) {
				t.Errorf("expires_at = %s, want now + 30 days", resp.ExpiresAt)
			}
		})
	}
}
