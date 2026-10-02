package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

var (
	_ tx.User = (*DB)(nil)
	_ tx.Auth = (*DB)(nil)
)

// SQL run first in every transaction, in one round trip. set_config('role', ..., true) is
// SET LOCAL ROLE; every setting ends with the transaction, so a connection back in the pool is
// plain app_login again with no app.user_id.
const (
	userBeginSQL = "select set_config('statement_timeout', ?, true), set_config('role', 'app_user', true), " +
		"set_config('app.user_id', ?, true)"
	authBeginSQL = "select set_config('statement_timeout', ?, true), set_config('role', 'app_auth', true)"
)

// WithUserTx implements tx.User: fn runs as app_user acting for userID, so row-level security
// limits it to that user's rows (ADR-0019, ADR-0032).
func (d *DB) WithUserTx(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context) error) error {
	if userID == uuid.Nil {
		return tx.ErrNoUser
	}
	return tx.Run(ctx, tx.RoleUser, userID, d.opener(userBeginSQL, d.timeout, userID.String()), fn)
}

// WithAuthTx implements tx.Auth: fn runs as app_auth, which reaches the account tables only
// (ADR-0034).
func (d *DB) WithAuthTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tx.Run(ctx, tx.RoleAuth, uuid.Nil, d.opener(authBeginSQL, d.timeout), fn)
}

// UserConn returns the connection of the user transaction open in ctx, bound to ctx. It fails
// outside a transaction, inside an auth transaction and after the transaction has finished; it
// never falls back to the root handle.
func UserConn(ctx context.Context) (*gorm.DB, error) { return conn(ctx, tx.RoleUser) }

// AuthConn returns the connection of the auth transaction open in ctx, bound to ctx. It fails
// like UserConn.
func AuthConn(ctx context.Context) (*gorm.DB, error) { return conn(ctx, tx.RoleAuth) }

type connKey struct{}

// openTx is the GORM transaction of one tx.Txn, kept in the context beside it.
type openTx struct {
	txn *tx.Txn
	g   *gorm.DB
}

func conn(ctx context.Context, role tx.Role) (*gorm.DB, error) {
	t, err := tx.Require(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("database connection: %w", err)
	}
	o, _ := ctx.Value(connKey{}).(*openTx)
	if o == nil || o.txn != t {
		return nil, fmt.Errorf("database connection: %w (the transaction was not opened by internal/db)", tx.ErrNoTx)
	}
	return o.g.WithContext(ctx), nil
}

// opener begins a GORM transaction and runs beginSQL with args in it. The transaction is bound
// to ctx: database/sql rolls it back when ctx is cancelled.
func (d *DB) opener(beginSQL string, args ...any) tx.Opener {
	return func(ctx context.Context, t *tx.Txn) (context.Context, tx.Ender, error) {
		g := d.root.WithContext(ctx).Begin()
		if g.Error != nil {
			return nil, nil, fmt.Errorf("begin transaction: %w", g.Error)
		}
		if err := g.Exec(beginSQL, args...).Error; err != nil {
			_ = g.Rollback()
			return nil, nil, fmt.Errorf("set up transaction: %w", err)
		}
		return context.WithValue(ctx, connKey{}, &openTx{txn: t, g: g}), ender{g: g}, nil
	}
}

type ender struct{ g *gorm.DB }

func (e ender) Commit() error { return e.g.Commit().Error }

// Rollback ignores sql.ErrTxDone: database/sql has already rolled back a transaction whose
// context was cancelled.
func (e ender) Rollback() error {
	if err := e.g.Rollback().Error; err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}
