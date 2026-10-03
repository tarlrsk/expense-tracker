// Package send is the outside service that sends one plain-text email (ADR-0028, ADR-0032).
//
// Business code depends on Port only. The SMTP adaptor is built once in app (through
// registry/mail) and carried in registry.Deps; tests use sendtest.Fake. Send is an outside call:
// it must run between database transactions, never inside one.
package send

import "context"

// Message is one plain-text email.
type Message struct {
	// To is the recipient's address.
	To string
	// Subject is the subject line.
	Subject string
	// Text is the plain-text body.
	Text string
}

// Port sends email.
type Port interface {
	// Send delivers m, or fails. It refuses to run inside an open database transaction
	// (tx.ErrInside) and stops at ctx's deadline.
	Send(ctx context.Context, m Message) error
}
