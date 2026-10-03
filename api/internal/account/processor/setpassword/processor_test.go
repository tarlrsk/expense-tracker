package setpassword

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountinsertsessionport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertsession"
	accountupdatepasswordport "github.com/tarlrsk/expense-tracker/api/internal/account/port/updatepassword"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	usableFn  func(ctx context.Context, hash []byte, now time.Time) (bool, error)
	useFn     func(ctx context.Context, hash []byte, now time.Time) (uuid.UUID, bool, error)
	updateFn  func(ctx context.Context, u accountupdatepasswordport.Update) (bool, error)
	removeFn  func(ctx context.Context, userID, keep uuid.UUID) error
	sessionFn func(ctx context.Context, s accountinsertsessionport.NewSession) (uuid.UUID, error)
)

func (f usableFn) Usable(ctx context.Context, h []byte, n time.Time) (bool, error) {
	return f(ctx, h, n)
}
func (f useFn) Use(ctx context.Context, h []byte, n time.Time) (uuid.UUID, bool, error) {
	return f(ctx, h, n)
}
func (f updateFn) Update(ctx context.Context, u accountupdatepasswordport.Update) (bool, error) {
	return f(ctx, u)
}
func (f removeFn) Remove(ctx context.Context, u, k uuid.UUID) error { return f(ctx, u, k) }
func (f sessionFn) Insert(ctx context.Context, s accountinsertsessionport.NewSession) (uuid.UUID, error) {
	return f(ctx, s)
}

func TestExecute(t *testing.T) {
	hasher := domain.NewHasher(domain.HashParams{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32})
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	link := domain.NewToken()
	userID := uuid.New()
	tests := []struct {
		name      string
		req       Request
		usable    bool
		used      bool
		updated   bool
		wantKind  apperr.Kind
		wantMsg   string
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "success", req: Request{Token: link.Plain, Password: "long enough pw"}, usable: true, used: true, updated: true,
			wantCalls: []string{"usable@1", "use@2", "update@2", "remove@2", "session@2"},
			wantTx:    []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "short password: nothing touches the database", req: Request{Token: link.Plain, Password: "short"},
			wantKind: apperr.InvalidInput, wantMsg: domain.PasswordRuleMessage,
		},
		{
			name: "malformed link: no transaction", req: Request{Token: "nope", Password: "long enough pw"},
			wantKind: apperr.InvalidInput, wantMsg: domain.LinkInvalidMessage,
		},
		{
			name: "unusable link: one cheap check, no second transaction", req: Request{Token: link.Plain, Password: "long enough pw"},
			wantKind: apperr.InvalidInput, wantMsg: domain.LinkInvalidMessage,
			wantCalls: []string{"usable@1"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "link used meanwhile: nothing written", req: Request{Token: link.Plain, Password: "long enough pw"}, usable: true,
			wantKind: apperr.InvalidInput, wantMsg: domain.LinkInvalidMessage,
			wantCalls: []string{"usable@1", "use@2"}, wantTx: []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "user row missing: rolled back", req: Request{Token: link.Plain, Password: "long enough pw"}, usable: true, used: true,
			wantKind: apperr.Internal, wantCalls: []string{"usable@1", "use@2", "update@2"},
			wantTx: []txtest.Outcome{txtest.Committed, txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := txtest.New()
			var calls []string
			call := func(name string) { calls = append(calls, name+"@"+strconv.Itoa(len(auth.Records()))) }
			var stored accountupdatepasswordport.Update
			kept := uuid.New()
			p := New(auth, Ports{
				FindLink: usableFn(func(context.Context, []byte, time.Time) (bool, error) { call("usable"); return tt.usable, nil }),
				UseLink: useFn(func(_ context.Context, h []byte, _ time.Time) (uuid.UUID, bool, error) {
					call("use")
					if string(h) != string(link.Hash) {
						t.Error("wrong link hash")
					}
					return userID, tt.used, nil
				}),
				UpdatePassword: updateFn(func(_ context.Context, u accountupdatepasswordport.Update) (bool, error) {
					call("update")
					stored = u
					return tt.updated, nil
				}),
				RemoveSessions: removeFn(func(_ context.Context, _, keep uuid.UUID) error { call("remove"); kept = keep; return nil }),
				InsertSession: sessionFn(func(context.Context, accountinsertsessionport.NewSession) (uuid.UUID, error) {
					call("session")
					return uuid.New(), nil
				}),
			}, hasher, func() time.Time { return now })

			_, err := p.Execute(t.Context(), tt.req)
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			var e *apperr.Error
			if tt.wantMsg != "" && (!errors.As(err, &e) || e.Message != tt.wantMsg) {
				t.Errorf("error = %v, want message %q", err, tt.wantMsg)
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
				if ok, _ := hasher.Verify(stored.Hash, tt.req.Password); !ok || stored.UserID != userID || stored.IfHash != nil {
					t.Errorf("stored %+v", stored)
				}
				if kept != uuid.Nil {
					t.Errorf("kept session %s, want none kept", kept)
				}
			}
		})
	}
}
