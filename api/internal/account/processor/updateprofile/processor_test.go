package updateprofile

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	nameFn  func(ctx context.Context, userID uuid.UUID, name string) (bool, error)
	styleFn func(ctx context.Context, userID uuid.UUID, style domain.DescriptionStyle) (bool, error)
	getFn   func(ctx context.Context, userID uuid.UUID) (accountgetprofileport.Profile, bool, error)
)

func (f nameFn) Update(ctx context.Context, u uuid.UUID, n string) (bool, error) { return f(ctx, u, n) }
func (f styleFn) Update(ctx context.Context, u uuid.UUID, s domain.DescriptionStyle) (bool, error) {
	return f(ctx, u, s)
}
func (f getFn) Get(ctx context.Context, u uuid.UUID) (accountgetprofileport.Profile, bool, error) {
	return f(ctx, u)
}

func ptr(s string) *string { return &s }

func TestExecute(t *testing.T) {
	user := uuid.New()
	boom := errors.New("boom")
	tests := []struct {
		name      string
		req       Request
		noRow     bool // the writes find no profile
		nameErr   error
		gone      bool // the read-back finds no profile
		wantKind  apperr.Kind
		wantErr   error
		wantCalls []string
		wantTx    []tx.Role
		wantOut   []txtest.Outcome
	}{
		{
			name: "nothing set: only the read", req: Request{UserID: user},
			wantCalls: []string{"get"}, wantTx: []tx.Role{tx.RoleAuth}, wantOut: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "the display name, trimmed", req: Request{UserID: user, DisplayName: ptr("  Ann ")},
			wantCalls: []string{"name Ann", "get"},
			wantTx:    []tx.Role{tx.RoleUser, tx.RoleAuth}, wantOut: []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "the description style", req: Request{UserID: user, DescriptionStyle: ptr("light_tidy")},
			wantCalls: []string{"style light_tidy", "get"},
			wantTx:    []tx.Role{tx.RoleUser, tx.RoleAuth}, wantOut: []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "both in one user transaction", req: Request{UserID: user, DisplayName: ptr("Bea"), DescriptionStyle: ptr("clean_name")},
			wantCalls: []string{"name Bea", "style clean_name", "get"},
			wantTx:    []tx.Role{tx.RoleUser, tx.RoleAuth}, wantOut: []txtest.Outcome{txtest.Committed, txtest.Committed},
		},
		{
			name: "an unknown style: nothing written, though the name is good",
			req:  Request{UserID: user, DisplayName: ptr("Bea"), DescriptionStyle: ptr("fancy")}, wantKind: apperr.InvalidInput,
		},
		{
			name: "an empty style", req: Request{UserID: user, DescriptionStyle: ptr("")}, wantKind: apperr.InvalidInput,
		},
		{
			name:     "a long name: nothing written, though the style is good",
			req:      Request{UserID: user, DisplayName: ptr(strings.Repeat("a", 51)), DescriptionStyle: ptr("as_typed")},
			wantKind: apperr.InvalidInput,
		},
		{
			name: "no profile to write: unauthenticated", req: Request{UserID: user, DescriptionStyle: ptr("as_typed")}, noRow: true,
			wantKind: apperr.Unauthenticated, wantCalls: []string{"style as_typed"},
			wantTx: []tx.Role{tx.RoleUser}, wantOut: []txtest.Outcome{txtest.Committed},
		},
		{
			name: "a failed write rolls back both", req: Request{UserID: user, DisplayName: ptr("Bea"), DescriptionStyle: ptr("as_typed")},
			nameErr: boom, wantErr: boom, wantCalls: []string{"name Bea"},
			wantTx: []tx.Role{tx.RoleUser}, wantOut: []txtest.Outcome{txtest.RolledBack},
		},
		{
			name: "the account is gone at the read: unauthenticated", req: Request{UserID: user}, gone: true,
			wantKind: apperr.Unauthenticated, wantCalls: []string{"get"},
			wantTx: []tx.Role{tx.RoleAuth}, wantOut: []txtest.Outcome{txtest.Committed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ftx := txtest.New()
			var calls []string
			inUserTx := func(ctx context.Context, u uuid.UUID) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user || u != user {
					t.Errorf("write outside the caller's user transaction (%v) or for %s", err, u)
				}
			}
			want := accountgetprofileport.Profile{ID: user, DescriptionStyle: domain.DescriptionStyleCleanName}
			p := New(ftx, ftx, Ports{
				UpdateDisplayName: nameFn(func(ctx context.Context, u uuid.UUID, n string) (bool, error) {
					inUserTx(ctx, u)
					calls = append(calls, "name "+n)
					return !tt.noRow && tt.nameErr == nil, tt.nameErr
				}),
				UpdateDescriptionStyle: styleFn(func(ctx context.Context, u uuid.UUID, s domain.DescriptionStyle) (bool, error) {
					inUserTx(ctx, u)
					calls = append(calls, "style "+string(s))
					return !tt.noRow, nil
				}),
				GetProfile: getFn(func(ctx context.Context, u uuid.UUID) (accountgetprofileport.Profile, bool, error) {
					if _, err := tx.Require(ctx, tx.RoleAuth); err != nil || u != user {
						t.Errorf("read outside an auth transaction (%v) or for %s", err, u)
					}
					calls = append(calls, "get")
					return want, !tt.gone, nil
				}),
			})
			resp, err := p.Execute(t.Context(), tt.req)
			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			case tt.wantKind != "":
				if apperr.KindOf(err) != tt.wantKind {
					t.Errorf("error = %v (kind %s), want %s", err, apperr.KindOf(err), tt.wantKind)
				}
			default:
				if err != nil || resp.Profile != want {
					t.Errorf("Execute = %+v, %v; want %+v", resp.Profile, err, want)
				}
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %q, want %q", calls, tt.wantCalls)
			}
			var roles []tx.Role
			var outcomes []txtest.Outcome
			for _, r := range ftx.Records() {
				roles, outcomes = append(roles, r.Role), append(outcomes, r.Outcome)
			}
			if !slices.Equal(roles, tt.wantTx) || !slices.Equal(outcomes, tt.wantOut) {
				t.Errorf("transactions = %v %v, want %v %v", roles, outcomes, tt.wantTx, tt.wantOut)
			}
		})
	}
}
