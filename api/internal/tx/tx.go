// Package tx declares how business code asks for a database transaction (ADR-0032).
//
// Processors and orchestrators depend on the User and Auth interfaces only. The open transaction
// travels in the context: a call that receives a context from inside a transaction joins it. The
// package knows nothing about GORM or connections; internal/db implements the interfaces, and
// txtest has a fake for tests without a database.
//
// Nesting rules (ADR-0032):
//   - the same role for the same user joins the open transaction (no savepoint);
//   - any other combination is an error and opens nothing;
//   - an error from a joined call makes the whole transaction roll back, even when the outer
//     function ignores it;
//   - a context whose transaction has finished cannot open or join a transaction again;
//   - a transaction whose context is cancelled or past its deadline when fn returns is rolled
//     back, and the error satisfies errors.Is for the context's error (ADR-0043).
package tx

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/google/uuid"
)

// User runs fn inside a transaction as role app_user acting for userID, so row-level security
// limits every query to that user's rows. fn gets a context that carries the transaction; it
// commits when fn returns nil and rolls back when fn returns an error or panics.
type User interface {
	WithUserTx(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context) error) error
}

// Auth runs fn inside a transaction as role app_auth, which reaches the account tables only
// (ADR-0034). Only the account module gets an Auth.
type Auth interface {
	WithAuthTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Errors returned by transactors and by MustBeOutside. They are wrapped with details; test them
// with errors.Is.
var (
	// ErrNoTx: no transaction is open in the context.
	ErrNoTx = errors.New("no transaction is open in this context")
	// ErrWrongRole: a transaction of the other role is open in the context.
	ErrWrongRole = errors.New("a transaction of another role is open in this context")
	// ErrOtherUser: a user transaction for a different user is open in the context.
	ErrOtherUser = errors.New("a transaction for another user is open in this context")
	// ErrFinished: the context's transaction has already committed or rolled back.
	ErrFinished = errors.New("the transaction of this context has finished")
	// ErrRolledBack: a joined call failed, so the whole transaction was rolled back.
	ErrRolledBack = errors.New("a call inside the transaction failed, so it was rolled back")
	// ErrNoUser: WithUserTx was called without a user id.
	ErrNoUser = errors.New("no user id for a user transaction")
	// ErrInside: an outside call (email, AI) was made inside an open transaction.
	ErrInside = errors.New("outside call made inside an open transaction")
)

// Role is the database role a transaction runs as.
type Role string

const (
	// RoleUser is app_user: one user's own data, under row-level security.
	RoleUser Role = "user"
	// RoleAuth is app_auth: account tables only.
	RoleAuth Role = "auth"
)

// Txn marks an open (or finished) transaction in a context. It holds no connection: the
// implementation keeps its own handle in the context beside it.
type Txn struct {
	role         Role
	userID       uuid.UUID
	finished     atomic.Bool
	rollbackOnly atomic.Bool
	cause        atomic.Pointer[error] // the first error of a joined call
}

// Role is the role the transaction runs as.
func (t *Txn) Role() Role { return t.role }

// UserID is the user of a RoleUser transaction; uuid.Nil for RoleAuth.
func (t *Txn) UserID() uuid.UUID { return t.userID }

type ctxKey struct{}

func current(ctx context.Context) *Txn {
	t, _ := ctx.Value(ctxKey{}).(*Txn)
	return t
}

// Require returns the open transaction of role in ctx. It fails with ErrNoTx, ErrWrongRole or
// ErrFinished. Database adaptors' connection getters use it (internal/db).
func Require(ctx context.Context, role Role) (*Txn, error) {
	t := current(ctx)
	switch {
	case t == nil:
		return nil, fmt.Errorf("%w (a %s transaction is needed)", ErrNoTx, role)
	case t.finished.Load():
		return nil, ErrFinished
	case t.role != role:
		return nil, fmt.Errorf("%w (open: %s, needed: %s)", ErrWrongRole, t.role, role)
	}
	return t, nil
}

// MustBeOutside returns ErrInside when ctx carries an open transaction of either role. Adaptors
// of outside services call it first: outside calls run between transactions (ADR-0032). A
// context whose transaction has finished is outside.
func MustBeOutside(ctx context.Context) error {
	if t := current(ctx); t != nil && !t.finished.Load() {
		return fmt.Errorf("%w (%s)", ErrInside, t.role)
	}
	return nil
}

// Ender ends a transaction opened by an Opener.
type Ender interface {
	Commit() error
	Rollback() error
}

// Opener begins a real transaction for t. ctx already carries t; the opener returns ctx with its
// own handle added (or ctx unchanged) and how to end the transaction.
type Opener func(ctx context.Context, t *Txn) (context.Context, Ender, error)

// Run applies the nesting rules for one WithUserTx or WithAuthTx call; every transactor uses it,
// so the real one and the fake behave the same. userID is uuid.Nil for RoleAuth.
//
// When ctx carries no transaction, Run calls open, runs fn with the context it returns, and
// commits when fn returns nil, no joined call failed and ctx is not done; otherwise it rolls
// back. A panic in fn rolls back and keeps panicking. When ctx carries an open transaction of
// the same role and user, fn joins it. Any other case returns an error without calling fn.
func Run(ctx context.Context, role Role, userID uuid.UUID, open Opener, fn func(ctx context.Context) error) error {
	if t := current(ctx); t != nil {
		switch {
		case t.finished.Load():
			return ErrFinished
		case t.role != role:
			return fmt.Errorf("%w (open: %s, wanted: %s)", ErrWrongRole, t.role, role)
		case t.userID != userID:
			return ErrOtherUser
		case t.rollbackOnly.Load():
			return t.rolledBack()
		}
		return join(ctx, t, fn)
	}

	t := &Txn{role: role, userID: userID}
	txCtx, end, err := open(context.WithValue(ctx, ctxKey{}, t), t)
	if err != nil {
		t.finished.Store(true)
		return err
	}
	return finish(txCtx, t, end, fn)
}

// finish runs fn in a new transaction and ends it.
func finish(ctx context.Context, t *Txn, end Ender, fn func(ctx context.Context) error) (err error) {
	ended := false
	defer func() {
		if !ended { // fn panicked: roll back, mark finished, and let the panic continue
			_ = end.Rollback()
		}
		t.finished.Store(true)
	}()

	fnErr := fn(ctx)
	ended = true
	ctxErr := ctx.Err()
	switch {
	case fnErr != nil:
		return withRollback(withContextErr(fnErr, ctxErr), end.Rollback())
	case t.rollbackOnly.Load():
		return withRollback(withContextErr(t.rolledBack(), ctxErr), end.Rollback())
	case ctxErr != nil:
		// The real transaction may already be gone (database/sql rolls it back on cancel), so
		// commit would only say "already committed or rolled back"; the caller needs to know why.
		return withRollback(fmt.Errorf("transaction not committed: %w", ctxErr), end.Rollback())
	}
	if err := end.Commit(); err != nil {
		// The context may have ended between the check above and the commit.
		return withContextErr(fmt.Errorf("commit: %w", err), ctx.Err())
	}
	return nil
}

// withContextErr adds ctxErr to err unless it is nil or err already carries it, so a handler can
// tell a timeout or a cancelled request (ADR-0043) from other failures with errors.Is.
func withContextErr(err, ctxErr error) error {
	if ctxErr == nil || errors.Is(err, ctxErr) {
		return err
	}
	return fmt.Errorf("%w (context: %w)", err, ctxErr)
}

// join runs fn inside the open transaction t. A failure or panic marks t rollback-only.
func join(ctx context.Context, t *Txn, fn func(ctx context.Context) error) error {
	done := false
	defer func() {
		if !done {
			t.markRollbackOnly(errJoinedPanic)
		}
	}()
	err := fn(ctx)
	done = true
	if err != nil {
		t.markRollbackOnly(err)
	}
	return err
}

func (t *Txn) markRollbackOnly(cause error) {
	t.cause.CompareAndSwap(nil, &cause)
	t.rollbackOnly.Store(true)
}

// errJoinedPanic is the cause recorded when a joined call panicked.
var errJoinedPanic = errors.New("a panic")

// rolledBack is ErrRolledBack with a short description of the joined call's error, not the error
// itself: the outer caller chose to ignore that error, so its kind (not found, conflict ...) must
// not decide the response, and its text may quote a row value (a Postgres message such as
// invalid input syntax for type uuid: "...", or a scan error), which must not reach a log.
func (t *Txn) rolledBack() error {
	if c := t.cause.Load(); c != nil {
		return fmt.Errorf("%w (cause: %s)", ErrRolledBack, describe(*c))
	}
	return ErrRolledBack
}

// sqlStater is what a Postgres error offers (*pgconn.PgError has it); this package does not
// import the driver.
type sqlStater interface {
	SQLState() string
}

// describe names err without its text: its SQLSTATE when it is a Postgres error, otherwise its
// Go type.
func describe(err error) string {
	if errors.Is(err, errJoinedPanic) {
		return errJoinedPanic.Error()
	}
	var s sqlStater
	if errors.As(err, &s) {
		return "SQLSTATE " + s.SQLState()
	}
	return fmt.Sprintf("%T", err)
}

func withRollback(err, rollbackErr error) error {
	if rollbackErr != nil {
		return errors.Join(err, fmt.Errorf("rollback: %w", rollbackErr))
	}
	return err
}
