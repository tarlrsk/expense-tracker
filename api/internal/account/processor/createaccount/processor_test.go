package createaccount

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountinsertuserport "github.com/tarlrsk/expense-tracker/api/internal/account/port/insertuser"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	userFn    func(ctx context.Context, email string) (accountinsertuserport.NewUser, error)
	profileFn func(ctx context.Context, userID uuid.UUID, role domain.Role) error
)

func (f userFn) Insert(ctx context.Context, e string) (accountinsertuserport.NewUser, error) {
	return f(ctx, e)
}
func (f profileFn) Insert(ctx context.Context, u uuid.UUID, r domain.Role) error { return f(ctx, u, r) }

func TestExecute(t *testing.T) {
	id, created := uuid.New(), time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		email       string
		userErr     error
		wantKind    apperr.Kind
		wantInserts int
		wantTx      []txtest.Outcome
	}{
		{name: "created", email: "  ann@example.test ", wantInserts: 2, wantTx: []txtest.Outcome{txtest.Committed}},
		{name: "not one plain address: no transaction", email: "Ann <ann@example.test>", wantKind: apperr.InvalidInput},
		{
			name: "email taken: conflict, rolled back", email: "ann@example.test", userErr: accountinsertuserport.ErrEmailTaken,
			wantKind: apperr.Conflict, wantInserts: 1, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "database error: internal", email: "ann@example.test", userErr: errors.New("boom"),
			wantKind: apperr.Internal, wantInserts: 1, wantTx: []txtest.Outcome{txtest.RolledBack},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := txtest.New()
			inserts := 0
			var profileFor uuid.UUID
			var profileRole domain.Role
			p := New(auth, Ports{
				InsertUser: userFn(func(_ context.Context, email string) (accountinsertuserport.NewUser, error) {
					inserts++
					if email != "ann@example.test" {
						t.Errorf("inserted email %q, want it trimmed", email)
					}
					return accountinsertuserport.NewUser{ID: id, CreatedAt: created}, tt.userErr
				}),
				InsertProfile: profileFn(func(_ context.Context, u uuid.UUID, r domain.Role) error {
					inserts++
					profileFor, profileRole = u, r
					return nil
				}),
			})
			resp, err := p.Execute(t.Context(), Request{Email: tt.email})
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if inserts != tt.wantInserts {
				t.Errorf("inserts = %d, want %d", inserts, tt.wantInserts)
			}
			records := auth.Records()
			if len(records) != len(tt.wantTx) {
				t.Fatalf("transactions = %+v, want %v", records, tt.wantTx)
			}
			for i, r := range records {
				if r.Outcome != tt.wantTx[i] {
					t.Errorf("transaction %d: %s, want %s", i, r.Outcome, tt.wantTx[i])
				}
			}
			if err != nil {
				return
			}
			want := domain.Account{ID: id, Email: "ann@example.test", Role: domain.RoleUser, Status: domain.StatusInvited, CreatedAt: created}
			if resp.Account != want {
				t.Errorf("account = %+v, want %+v", resp.Account, want)
			}
			if profileFor != id || profileRole != domain.RoleUser {
				t.Errorf("profile for %s with role %q", profileFor, profileRole)
			}
		})
	}
}
