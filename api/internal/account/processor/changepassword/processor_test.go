package changepassword

import (
	"context"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcountattemptsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/countattempts"
	accountgetcredentialsport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getcredentials"
	accountupdatepasswordport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatepassword"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	getFn       func(ctx context.Context, userID uuid.UUID) (accountgetcredentialsport.Credentials, bool, error)
	countFn     func(ctx context.Context, email string, ip netip.Addr, since time.Time) (accountcountattemptsport.Counts, error)
	attemptFn   func(ctx context.Context, email string, ip netip.Addr, at time.Time) (uuid.UUID, error)
	removeIDFn  func(ctx context.Context, id uuid.UUID) error
	removeOldFn func(ctx context.Context, before time.Time) error
	updateFn    func(ctx context.Context, u accountupdatepasswordport.Update) (bool, error)
	sessionsFn  func(ctx context.Context, userID, keep uuid.UUID) error
)

func (f getFn) Get(ctx context.Context, u uuid.UUID) (accountgetcredentialsport.Credentials, bool, error) {
	return f(ctx, u)
}
func (f countFn) Count(ctx context.Context, e string, ip netip.Addr, s time.Time) (accountcountattemptsport.Counts, error) {
	return f(ctx, e, ip, s)
}
func (f attemptFn) Insert(ctx context.Context, e string, ip netip.Addr, at time.Time) (uuid.UUID, error) {
	return f(ctx, e, ip, at)
}
func (f removeIDFn) Remove(ctx context.Context, id uuid.UUID) error   { return f(ctx, id) }
func (f removeOldFn) Remove(ctx context.Context, b time.Time) error   { return f(ctx, b) }
func (f sessionsFn) Remove(ctx context.Context, u, k uuid.UUID) error { return f(ctx, u, k) }
func (f updateFn) Update(ctx context.Context, u accountupdatepasswordport.Update) (bool, error) {
	return f(ctx, u)
}

func TestExecute(t *testing.T) {
	hasher := domain.NewHasher(domain.HashParams{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32})
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	current := hasher.Hash("the current one")
	userID, sessionID, attemptID := uuid.New(), uuid.New(), uuid.New()
	ok := Request{UserID: userID, SessionID: sessionID, IP: "192.0.2.9", CurrentPassword: "the current one", NewPassword: "the new password"}
	with := func(change func(*Request)) Request { r := ok; change(&r); return r }

	tests := []struct {
		name      string
		req       Request
		counts    accountcountattemptsport.Counts
		missing   bool
		stale     bool
		wantKind  apperr.Kind
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "success", req: ok,
			wantCalls: []string{"get@1", "count@1", "attempt@1", "unattempt@2", "update@2", "sessions@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "new password breaks the rule: nothing touches the database", req: with(func(r *Request) { r.NewPassword = "short" }),
			wantKind: apperr.InvalidInput,
		},
		{
			name: "wrong current password: the failure commits, then 400", req: with(func(r *Request) { r.CurrentPassword = "nope" }),
			wantKind:  apperr.InvalidInput,
			wantCalls: []string{"get@1", "count@1", "attempt@1", "cleanup@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "limited: nothing recorded", req: ok, counts: accountcountattemptsport.Counts{Email: 5},
			wantKind: apperr.RateLimited, wantCalls: []string{"get@1", "count@1"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "account gone", req: ok, missing: true,
			wantKind: apperr.Unauthenticated, wantCalls: []string{"get@1"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "password changed meanwhile: rolled back, the attempt stays", req: ok, stale: true,
			wantKind:  apperr.InvalidInput,
			wantCalls: []string{"get@1", "count@1", "attempt@1", "unattempt@2", "update@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := txtest.New()
			var calls []string
			call := func(name string) { calls = append(calls, name+"@"+strconv.Itoa(len(auth.Records()))) }
			var update accountupdatepasswordport.Update
			var keep, removedAttempt uuid.UUID
			p := New(auth, Ports{
				GetCredentials: getFn(func(context.Context, uuid.UUID) (accountgetcredentialsport.Credentials, bool, error) {
					call("get")
					return accountgetcredentialsport.Credentials{Email: "a@example.test", PasswordHash: current}, !tt.missing, nil
				}),
				CountAttempts: countFn(func(context.Context, string, netip.Addr, time.Time) (accountcountattemptsport.Counts, error) {
					call("count")
					return tt.counts, nil
				}),
				InsertAttempt: attemptFn(func(context.Context, string, netip.Addr, time.Time) (uuid.UUID, error) {
					call("attempt")
					return attemptID, nil
				}),
				RemoveAttempt:     removeIDFn(func(_ context.Context, id uuid.UUID) error { call("unattempt"); removedAttempt = id; return nil }),
				RemoveOldAttempts: removeOldFn(func(context.Context, time.Time) error { call("cleanup"); return nil }),
				UpdatePassword: updateFn(func(_ context.Context, u accountupdatepasswordport.Update) (bool, error) {
					call("update")
					update = u
					return !tt.stale, nil
				}),
				RemoveSessions: sessionsFn(func(_ context.Context, _, k uuid.UUID) error { call("sessions"); keep = k; return nil }),
			}, hasher, func() time.Time { return now })

			_, err := p.Execute(t.Context(), tt.req)
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			var outcomes []txtest.Outcome
			for _, r := range auth.Records() {
				outcomes = append(outcomes, r.Outcome)
			}
			if !slices.Equal(outcomes, tt.wantTx) {
				t.Errorf("transactions = %v, want %v", outcomes, tt.wantTx)
			}
			if err == nil {
				if update.IfHash == nil || *update.IfHash != current || update.UserID != userID {
					t.Errorf("update %+v: want it conditional on the checked hash", update)
				}
				if match, _ := hasher.Verify(update.Hash, ok.NewPassword); !match {
					t.Error("stored hash is not of the new password")
				}
				if keep != sessionID || removedAttempt != attemptID {
					t.Errorf("kept session %s, removed attempt %s", keep, removedAttempt)
				}
			}
		})
	}
}
