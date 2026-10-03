package removeaccount

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	lockFn   func(ctx context.Context) error
	roleFn   func(ctx context.Context, userID uuid.UUID) (domain.Role, bool, error)
	countFn  func(ctx context.Context) (int, error)
	removeFn func(ctx context.Context, userID uuid.UUID) (bool, error)
)

func (f lockFn) Lock(ctx context.Context) error { return f(ctx) }
func (f roleFn) Get(ctx context.Context, u uuid.UUID) (domain.Role, bool, error) {
	return f(ctx, u)
}
func (f countFn) Count(ctx context.Context) (int, error)                 { return f(ctx) }
func (f removeFn) Remove(ctx context.Context, u uuid.UUID) (bool, error) { return f(ctx, u) }

func TestExecute(t *testing.T) {
	target, op := uuid.New(), uuid.New()
	tests := []struct {
		name      string
		req       Request
		role      domain.Role
		missing   bool
		operators int
		wantKind  apperr.Kind
		wantCalls []string
		wantTx    []txtest.Outcome
	}{
		{
			name: "operator removes a user", req: Request{UserID: target, OperatorID: op}, role: domain.RoleUser,
			wantCalls: []string{"lock", "role", "remove"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "user deletes their own account", req: Request{UserID: target}, role: domain.RoleUser,
			wantCalls: []string{"lock", "role", "remove"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "one of two operators", req: Request{UserID: target, OperatorID: op}, role: domain.RoleOperator, operators: 2,
			wantCalls: []string{"lock", "role", "count", "remove"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "the last operator", req: Request{UserID: target}, role: domain.RoleOperator, operators: 1,
			wantKind: apperr.Conflict, wantCalls: []string{"lock", "role", "count"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "operator's own id through the admin path: nothing touches the database", req: Request{UserID: op, OperatorID: op},
			wantKind: apperr.Conflict,
		},
		{
			name: "unknown user", req: Request{UserID: target, OperatorID: op}, missing: true,
			wantKind: apperr.NotFound, wantCalls: []string{"lock", "role"}, wantTx: []txtest.Outcome{txtest.Committed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := txtest.New()
			var calls []string
			// Every port runs inside the one auth transaction, the lock first.
			call := func(ctx context.Context, name string) {
				if _, err := tx.Require(ctx, tx.RoleAuth); err != nil {
					t.Errorf("%s outside the auth transaction: %v", name, err)
				}
				calls = append(calls, name)
			}
			p := New(auth, Ports{
				LockOperators: lockFn(func(ctx context.Context) error { call(ctx, "lock"); return nil }),
				GetRole: roleFn(func(ctx context.Context, u uuid.UUID) (domain.Role, bool, error) {
					call(ctx, "role")
					return tt.role, !tt.missing, nil
				}),
				CountOperators: countFn(func(ctx context.Context) (int, error) { call(ctx, "count"); return tt.operators, nil }),
				RemoveUser: removeFn(func(ctx context.Context, u uuid.UUID) (bool, error) {
					call(ctx, "remove")
					if u != tt.req.UserID {
						t.Errorf("removed %s, want %s", u, tt.req.UserID)
					}
					return true, nil
				}),
			})
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
		})
	}
}
