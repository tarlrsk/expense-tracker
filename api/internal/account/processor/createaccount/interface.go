// Package createaccount is the one place that creates accounts (ADR-0036, docs/05-roadmap.md
// guardrails): the users row and the profile in one auth transaction; the database trigger seeds
// the default categories. The invite and the operator command both use it.
package createaccount

import (
	"context"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
)

// Processor creates accounts.
type Processor interface {
	// Execute creates an account without a password, with the user role. It returns
	// invalid_input when the email is not one plain address (domain.CheckNewEmail) and conflict
	// when the email already has an account.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the new account's email. Later sources (self-registration) add a field here.
type Request struct {
	Email string
}

// Response is the new account as the operator's list shows it.
type Response struct {
	Account domain.Account
}
