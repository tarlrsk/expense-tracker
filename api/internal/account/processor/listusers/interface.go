// Package listusers is the operator's list of accounts (GET /api/admin/users, ADR-0068). It never
// returns financial data, password hashes or tokens.
package listusers

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Processor lists accounts.
type Processor interface {
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is empty for now; paging would add fields here.
type Request struct{}

// Response is every account, ordered by created_at, then id.
type Response struct {
	Users []domain.Account
}
