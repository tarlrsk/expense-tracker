package updatetransaction

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categorizationlearnproc "github.com/tarlrsk/expense-tracker/api/internal/categorization/processor/learn"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionsupdateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/update"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	updateFn func(ctx context.Context, req transactionsupdateproc.Request) (transactionsupdateproc.Response, error)
	learnFn  func(ctx context.Context, req categorizationlearnproc.Request) (categorizationlearnproc.Response, error)
)

func (f updateFn) Execute(ctx context.Context, r transactionsupdateproc.Request) (transactionsupdateproc.Response, error) {
	return f(ctx, r)
}

func (f learnFn) Execute(ctx context.Context, r categorizationlearnproc.Request) (categorizationlearnproc.Response, error) {
	return f(ctx, r)
}

func TestExecute(t *testing.T) {
	user, cat, id := uuid.New(), uuid.New(), uuid.New()
	notFound := apperr.New(apperr.NotFound, domain.NotFoundMessage)
	invalid := apperr.New(apperr.InvalidInput, domain.CategoryMessage)
	boom := errors.New("boom")
	tests := []struct {
		name                             string
		merchant                         string // after the update
		categoryChanged, merchantChanged bool
		updateErr                        error
		learnErr                         error
		wantErr                          error
		wantLearn                        bool
		wantTx                           txtest.Outcome
	}{
		{name: "category changed: learnt", merchant: "Grab", categoryChanged: true, wantLearn: true, wantTx: txtest.Committed},
		{name: "merchant changed: learnt", merchant: "Grab", merchantChanged: true, wantLearn: true, wantTx: txtest.Committed},
		{
			name: "category and merchant changed: learnt", merchant: "Grab", categoryChanged: true, merchantChanged: true,
			wantLearn: true, wantTx: txtest.Committed,
		},
		{name: "merchant cleared: nothing learnt", merchantChanged: true, wantTx: txtest.Committed},
		{name: "category changed on a transaction without a merchant: nothing learnt", categoryChanged: true, wantTx: txtest.Committed},
		{name: "only amount, date, note or currency, or nothing, changed: nothing learnt", merchant: "Grab", wantTx: txtest.Committed},
		{name: "not found: returned unchanged", updateErr: notFound, wantErr: notFound, wantTx: txtest.RolledBack},
		{name: "invalid input: returned unchanged", updateErr: invalid, wantErr: invalid, wantTx: txtest.RolledBack},
		{
			name: "learning fails: the update is rolled back", merchant: "Grab", categoryChanged: true, learnErr: boom,
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
			merchant := " Grab "
			req := transactionsupdateproc.Request{UserID: user, ID: id, Merchant: &merchant}
			stored := transactionsupdateproc.Response{
				Transaction:     domain.Transaction{ID: id, OwnerID: user, Merchant: tt.merchant, CategoryID: cat},
				CategoryChanged: tt.categoryChanged, MerchantChanged: tt.merchantChanged,
			}
			learnt := false
			o := New(utx,
				updateFn(func(ctx context.Context, got transactionsupdateproc.Request) (transactionsupdateproc.Response, error) {
					inTx(ctx, "update")
					if got != req {
						t.Errorf("update got %+v, want %+v", got, req)
					}
					if tt.updateErr != nil {
						return transactionsupdateproc.Response{}, tt.updateErr
					}
					return stored, nil
				}),
				learnFn(func(ctx context.Context, got categorizationlearnproc.Request) (categorizationlearnproc.Response, error) {
					inTx(ctx, "learn")
					learnt = true
					// The transaction after the update: its merchant and category.
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
			if tt.updateErr != nil && apperr.KindOf(err) != apperr.KindOf(tt.updateErr) {
				t.Errorf("error kind = %q, want %q", apperr.KindOf(err), apperr.KindOf(tt.updateErr))
			}
			if learnt != tt.wantLearn {
				t.Errorf("learn called = %v, want %v", learnt, tt.wantLearn)
			}
			want := []txtest.Record{{Role: tx.RoleUser, UserID: user, Outcome: tt.wantTx}}
			if got := utx.Records(); !slices.Equal(got, want) {
				t.Errorf("transactions = %+v, want %+v", got, want)
			}
			if err == nil && resp != stored {
				t.Errorf("response = %+v, want the update's %+v", resp, stored)
			}
		})
	}
}
