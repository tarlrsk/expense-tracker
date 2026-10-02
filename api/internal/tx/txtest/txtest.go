// Package txtest has a fake transactor for processor and orchestrator tests without a database.
//
// The fake implements tx.User and tx.Auth with the same nesting rules as internal/db (both use
// tx.Run), so tx.MustBeOutside and joining behave as in production. It records every transaction
// it opened and how it ended, for tests to assert on.
package txtest

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Outcome is how a recorded transaction ended.
type Outcome string

const (
	// Open: the transaction has not ended yet.
	Open Outcome = "open"
	// Committed: fn returned nil and no joined call failed.
	Committed Outcome = "committed"
	// RolledBack: fn returned an error or panicked, or a joined call failed.
	RolledBack Outcome = "rolled back"
)

// Record is one transaction the fake opened. Joined calls open no transaction and add no record.
type Record struct {
	Role    tx.Role
	UserID  uuid.UUID // uuid.Nil for tx.RoleAuth
	Outcome Outcome
}

// Fake is a transactor without a database. The zero value is ready to use; it is safe for
// concurrent use.
type Fake struct {
	mu      sync.Mutex
	records []Record
}

var (
	_ tx.User = (*Fake)(nil)
	_ tx.Auth = (*Fake)(nil)
)

// New returns an empty Fake.
func New() *Fake { return &Fake{} }

// WithUserTx implements tx.User.
func (f *Fake) WithUserTx(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context) error) error {
	if userID == uuid.Nil {
		return tx.ErrNoUser
	}
	return tx.Run(ctx, tx.RoleUser, userID, f.opener(tx.RoleUser, userID), fn)
}

// WithAuthTx implements tx.Auth.
func (f *Fake) WithAuthTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.Run(ctx, tx.RoleAuth, uuid.Nil, f.opener(tx.RoleAuth, uuid.Nil), fn)
}

// Records returns a copy of every transaction opened so far, oldest first.
func (f *Fake) Records() []Record {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Record(nil), f.records...)
}

func (f *Fake) opener(role tx.Role, userID uuid.UUID) tx.Opener {
	return func(ctx context.Context, _ *tx.Txn) (context.Context, tx.Ender, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.records = append(f.records, Record{Role: role, UserID: userID, Outcome: Open})
		return ctx, ender{f: f, i: len(f.records) - 1}, nil
	}
}

type ender struct {
	f *Fake
	i int
}

func (e ender) Commit() error   { e.set(Committed); return nil }
func (e ender) Rollback() error { e.set(RolledBack); return nil }

func (e ender) set(o Outcome) {
	e.f.mu.Lock()
	defer e.f.mu.Unlock()
	e.f.records[e.i].Outcome = o
}
