package deleteme

import (
	"context"
	"fmt"

	accountcheckpasswordproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/checkpassword"
	accountremoveaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/removeaccount"
)

type orchestrator struct {
	check  accountcheckpasswordproc.Processor
	remove accountremoveaccountproc.Processor
}

// New returns the delete-my-account use case.
func New(check accountcheckpasswordproc.Processor, remove accountremoveaccountproc.Processor) Orchestrator {
	return &orchestrator{check: check, remove: remove}
}

// Execute checks the password (its own transactions, hashing outside them, ADR-0066), then
// removes the account in the removal's own locked transaction.
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	_, err := o.check.Execute(ctx, accountcheckpasswordproc.Request{UserID: req.UserID, IP: req.IP, Password: req.Password})
	if err != nil {
		return Response{}, fmt.Errorf("delete my account: %w", err)
	}
	if _, err := o.remove.Execute(ctx, accountremoveaccountproc.Request{UserID: req.UserID}); err != nil {
		return Response{}, fmt.Errorf("delete my account: %w", err)
	}
	return Response{}, nil
}
