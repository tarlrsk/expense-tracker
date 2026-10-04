package createtransaction

import (
	"context"

	categorizationlearnproc "github.com/tarlrsk/expense-tracker/api/internal/categorization/processor/learn"
	transactionscreateproc "github.com/tarlrsk/expense-tracker/api/internal/transactions/processor/create"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// The transactions handler takes a create processor; this orchestrator is one.
var _ transactionscreateproc.Processor = (*orchestrator)(nil)

type orchestrator struct {
	user   tx.User
	create transactionscreateproc.Processor
	learn  categorizationlearnproc.Processor
}

// New returns the create-transaction use case with rule learning.
func New(user tx.User, create transactionscreateproc.Processor, learn categorizationlearnproc.Processor) Orchestrator {
	return &orchestrator{user: user, create: create, learn: learn}
}

// Execute runs the create and the learning in one user transaction, which both processors join,
// so the transaction and its rule commit together or not at all (ADR-0078). Only a create that
// inserted (Created true) a transaction with a merchant teaches: a retry with the same id
// (ADR-0040) and a transaction without a merchant teach nothing. Whether the merchant has a key
// and the category is active is up to learn.
//
// The create processor's errors are returned as they are, so the response does not change.
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	var resp Response
	err := o.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		var err error
		resp, err = o.create.Execute(ctx, req)
		if err != nil || !resp.Created || resp.Transaction.Merchant == "" {
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
