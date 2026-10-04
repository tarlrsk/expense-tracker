package updatetransaction

import (
	"context"

	categorizationlearnproc "github.com/tarlrsk/expense-tracker/api/internal/categorization/processor/learn"
	transactionsupdateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/update"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// The transactions handler takes an update processor; this orchestrator is one.
var _ transactionsupdateproc.Processor = (*orchestrator)(nil)

type orchestrator struct {
	user   tx.User
	update transactionsupdateproc.Processor
	learn  categorizationlearnproc.Processor
}

// New returns the update-transaction use case with rule learning.
func New(user tx.User, update transactionsupdateproc.Processor, learn categorizationlearnproc.Processor) Orchestrator {
	return &orchestrator{user: user, update: update, learn: learn}
}

// Execute runs the update and the learning in one user transaction, which both processors join,
// so the change and its rule commit together or not at all (ADR-0078). Only an update that
// changed the category or the merchant and left the transaction with a merchant teaches: a
// change of only the amount, date, note or currency, a PATCH that changes nothing and clearing
// the merchant teach nothing. Whether the merchant has a key and the category is active (it may
// be an archived one the transaction kept) is up to learn.
//
// The update processor's errors are returned as they are, so the response does not change.
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	var resp Response
	err := o.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		var err error
		resp, err = o.update.Execute(ctx, req)
		if err != nil || (!resp.CategoryChanged && !resp.MerchantChanged) || resp.Transaction.Merchant == "" {
			return err
		}
		_, err = o.learn.Execute(ctx, categorizationlearnproc.Request{
			UserID: req.UserID, Merchant: resp.Transaction.Merchant, CategoryID: resp.Transaction.CategoryID,
		})
		return err
	})
	if err != nil {
		return Response{}, err
	}
	return resp, nil
}
