package createtransaction

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categorizationlearnproc "github.com/tarlrsk/expense-tracker/api/internal/categorization/processor/learn"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionscreateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/create"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	createFn func(ctx context.Context, req transactionscreateproc.Request) (transactionscreateproc.Response, error)
	learnFn  func(ctx context.Context, req categorizationlearnproc.Request) (categorizationlearnproc.Response, error)
)

func (f createFn) Execute(ctx context.Context, r transactionscreateproc.Request) (transactionscreateproc.Response, error) {
	return f(ctx, r)
}

func (f learnFn) Execute(ctx context.Context, r categorizationlearnproc.Request) (categorizationlearnproc.Response, error) {
	return f(ctx, r)
}

func TestExecute(t *testing.T) {
	user, cat, id := uuid.New(), uuid.New(), uuid.New()
	invalid := apperr.New(apperr.InvalidInput, domain.CategoryMessage)
	boom := errors.New("boom")
	tests := []struct {
		name      string
		merchant  string
		created   bool
		createErr error
		learnErr  error
		wantErr   error
		wantLearn bool
		wantTx    txtest.Outcome
	}{
		{name: "created with a merchant: learnt", merchant: "7-Eleven", created: true, wantLearn: true, wantTx: txtest.Committed},
		{name: "created without a merchant: nothing learnt", created: true, wantTx: txtest.Committed},
		{name: "a retry with the same id (200): nothing learnt", merchant: "7-Eleven", wantTx: txtest.Committed},
		{
			name: "the create's error is returned unchanged, nothing learnt", merchant: "7-Eleven", createErr: invalid,
			wantErr: invalid, wantTx: txtest.RolledBack,
		},
		{
			name: "learning fails: the create is rolled back", merchant: "7-Eleven", created: true, learnErr: boom,
			wantErr: boom, wantLearn: true, wantTx: txtest.RolledBack,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := txtest.New()
			inTx := func(ctx context.Context, name string) {
				if txn, err := tx.Require(ctx, tx.RoleUser); err != nil || txn.UserID() != user {
					t.Errorf("%s outside the caller's user transaction: %v", name, err)
				}
			}
			stored := domain.Transaction{ID: id, OwnerID: user, Merchant: tt.merchant, CategoryID: cat}
			req := transactionscreateproc.Request{UserID: user, ID: id.String(), Merchant: " " + tt.merchant + " ", CategoryID: cat.String()}
			learnt := false
			o := New(utx,
				createFn(func(ctx context.Context, got transactionscreateproc.Request) (transactionscreateproc.Response, error) {
					inTx(ctx, "create")
					if got != req {
						t.Errorf("create got %+v, want %+v", got, req)
					}
					if tt.createErr != nil {
						return transactionscreateproc.Response{}, tt.createErr
					}
					return transactionscreateproc.Response{Transaction: stored, Created: tt.created}, nil
				}),
				learnFn(func(ctx context.Context, got categorizationlearnproc.Request) (categorizationlearnproc.Response, error) {
					inTx(ctx, "learn")
					learnt = true
					// The stored merchant (trimmed), not the one sent.
					if want := (categorizationlearnproc.Request{UserID: user, Merchant: tt.merchant, CategoryID: cat}); got != want {
						t.Errorf("learn got %+v, want %+v", got, want)
					}
					return categorizationlearnproc.Response{Learnt: tt.learnErr == nil}, tt.learnErr
				}))

			resp, err := o.Execute(t.Context(), req)
			if (tt.wantErr == nil && err != nil) || !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && err.Error() != tt.wantErr.Error() {
				t.Errorf("error = %q, want it unchanged: %q", err, tt.wantErr)
			}
			if tt.createErr != nil && apperr.KindOf(err) != apperr.KindOf(tt.createErr) {
				t.Errorf("error kind = %q, want %q", apperr.KindOf(err), apperr.KindOf(tt.createErr))
			}
			if learnt != tt.wantLearn {
				t.Errorf("learn called = %v, want %v", learnt, tt.wantLearn)
			}
			want := []txtest.Record{{Role: tx.RoleUser, UserID: user, Outcome: tt.wantTx}}
			if got := utx.Records(); !slices.Equal(got, want) {
				t.Errorf("transactions = %+v, want %+v", got, want)
			}
			if err == nil && (resp.Transaction != stored || resp.Created != tt.created) {
				t.Errorf("response = %+v, want the create's", resp)
			}
		})
	}
}
