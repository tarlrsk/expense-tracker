// Package addoperator is the operator command (ADR-0035, ADR-0056, ADR-0068): make sure an
// account exists for the email, give it the operator role and, if it has no password yet, email
// it a new set-password link. It may be run again: it never makes a second account.
package addoperator

import "context"

// Orchestrator adds operators.
type Orchestrator interface {
	// Execute returns what was done. A failed email is an error here (the command says to run it
	// again); the account and role stay as they were set.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the operator's email.
type Request struct {
	Email string
}

// Response says what was done.
type Response struct {
	// Created: the account did not exist and was made.
	Created bool
	// Promoted: the account did not have the operator role before.
	Promoted bool
	// EmailSent: the account had no password, so a new link was made and emailed; earlier links
	// no longer work.
	EmailSent bool
}
