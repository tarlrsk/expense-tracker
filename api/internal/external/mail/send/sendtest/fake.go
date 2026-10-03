// Package sendtest has a fake mailer for tests: it records messages instead of sending them.
package sendtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Fake implements send.Port in memory. Like the SMTP adaptor it refuses to run inside an open
// transaction. The zero value is ready to use; it is safe for concurrent use.
type Fake struct {
	mu   sync.Mutex
	sent []send.Message
	err  error
}

var _ send.Port = (*Fake)(nil)

// New returns an empty Fake.
func New() *Fake { return &Fake{} }

// Send records m, or returns the error set by FailWith.
func (f *Fake) Send(ctx context.Context, m send.Message) error {
	if err := tx.MustBeOutside(ctx); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

// FailWith makes every later Send return err (nil sends again).
func (f *Fake) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Sent returns a copy of the messages sent so far, oldest first.
func (f *Fake) Sent() []send.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]send.Message(nil), f.sent...)
}
