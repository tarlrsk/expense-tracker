package txtest_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

var errBoom = errors.New("boom")

func ok(context.Context) error { return nil }

// The fake follows the nesting rules of ADR-0032 and records each transaction it opened.
func TestFake(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	user := func(id uuid.UUID, o txtest.Outcome) txtest.Record {
		return txtest.Record{Role: tx.RoleUser, UserID: id, Outcome: o}
	}
	auth := func(o txtest.Outcome) txtest.Record { return txtest.Record{Role: tx.RoleAuth, Outcome: o} }

	tests := []struct {
		name        string
		run         func(f *txtest.Fake) error
		wantErr     error
		wantPanic   bool
		wantRecords []txtest.Record
	}{
		{
			name:        "user commit",
			run:         func(f *txtest.Fake) error { return f.WithUserTx(context.Background(), a, ok) },
			wantRecords: []txtest.Record{user(a, txtest.Committed)},
		},
		{
			name:        "auth commit",
			run:         func(f *txtest.Fake) error { return f.WithAuthTx(context.Background(), ok) },
			wantRecords: []txtest.Record{auth(txtest.Committed)},
		},
		{
			name: "error rolls back",
			run: func(f *txtest.Fake) error {
				return f.WithUserTx(context.Background(), a, func(context.Context) error { return errBoom })
			},
			wantErr: errBoom, wantRecords: []txtest.Record{user(a, txtest.RolledBack)},
		},
		{
			name: "panic rolls back",
			run: func(f *txtest.Fake) error {
				return f.WithAuthTx(context.Background(), func(context.Context) error { panic(errBoom) })
			},
			wantPanic: true, wantRecords: []txtest.Record{auth(txtest.RolledBack)},
		},
		{
			name: "same user joins",
			run: func(f *txtest.Fake) error {
				return f.WithUserTx(context.Background(), a, func(ctx context.Context) error { return f.WithUserTx(ctx, a, ok) })
			},
			wantRecords: []txtest.Record{user(a, txtest.Committed)},
		},
		{
			name: "auth joins auth",
			run: func(f *txtest.Fake) error {
				return f.WithAuthTx(context.Background(), func(ctx context.Context) error { return f.WithAuthTx(ctx, ok) })
			},
			wantRecords: []txtest.Record{auth(txtest.Committed)},
		},
		{
			name: "swallowed inner error rolls back",
			run: func(f *txtest.Fake) error {
				return f.WithUserTx(context.Background(), a, func(ctx context.Context) error {
					if err := f.WithUserTx(ctx, a, func(context.Context) error { return errBoom }); !errors.Is(err, errBoom) {
						t.Errorf("inner error = %v, want errBoom", err)
					}
					return nil
				})
			},
			wantErr: tx.ErrRolledBack, wantRecords: []txtest.Record{user(a, txtest.RolledBack)},
		},
		{
			name: "recovered inner panic rolls back",
			run: func(f *txtest.Fake) error {
				return f.WithUserTx(context.Background(), a, func(ctx context.Context) error {
					func() {
						defer func() { _ = recover() }()
						_ = f.WithUserTx(ctx, a, func(context.Context) error { panic(errBoom) })
					}()
					return nil
				})
			},
			wantErr: tx.ErrRolledBack, wantRecords: []txtest.Record{user(a, txtest.RolledBack)},
		},
		{
			name: "no join after a failed join",
			run: func(f *txtest.Fake) error {
				var second error
				_ = f.WithUserTx(context.Background(), a, func(ctx context.Context) error {
					_ = f.WithUserTx(ctx, a, func(context.Context) error { return errBoom })
					second = f.WithUserTx(ctx, a, func(context.Context) error { t.Error("ran after a failed join"); return nil })
					return nil
				})
				return second
			},
			wantErr: tx.ErrRolledBack, wantRecords: []txtest.Record{user(a, txtest.RolledBack)},
		},
		{
			name: "different user refused",
			run: func(f *txtest.Fake) error {
				var inner error
				err := f.WithUserTx(context.Background(), a, func(ctx context.Context) error {
					inner = f.WithUserTx(ctx, b, func(context.Context) error { t.Error("ran"); return nil })
					return nil
				})
				return errors.Join(inner, err)
			},
			wantErr: tx.ErrOtherUser, wantRecords: []txtest.Record{user(a, txtest.Committed)},
		},
		{
			name: "user inside auth refused",
			run: func(f *txtest.Fake) error {
				var inner error
				_ = f.WithAuthTx(context.Background(), func(ctx context.Context) error {
					inner = f.WithUserTx(ctx, a, func(context.Context) error { t.Error("ran"); return nil })
					return nil
				})
				return inner
			},
			wantErr: tx.ErrWrongRole, wantRecords: []txtest.Record{auth(txtest.Committed)},
		},
		{
			name: "auth inside user refused",
			run: func(f *txtest.Fake) error {
				var inner error
				_ = f.WithUserTx(context.Background(), a, func(ctx context.Context) error {
					inner = f.WithAuthTx(ctx, func(context.Context) error { t.Error("ran"); return nil })
					return nil
				})
				return inner
			},
			wantErr: tx.ErrWrongRole, wantRecords: []txtest.Record{user(a, txtest.Committed)},
		},
		{
			name: "finished context refused",
			run: func(f *txtest.Fake) error {
				var done context.Context
				_ = f.WithUserTx(context.Background(), a, func(ctx context.Context) error { done = ctx; return nil })
				return f.WithUserTx(done, a, func(context.Context) error { t.Error("ran"); return nil })
			},
			wantErr: tx.ErrFinished, wantRecords: []txtest.Record{user(a, txtest.Committed)},
		},
		{
			name:    "nil user refused",
			run:     func(f *txtest.Fake) error { return f.WithUserTx(context.Background(), uuid.Nil, ok) },
			wantErr: tx.ErrNoUser,
		},
		{
			name: "two transactions in sequence",
			run: func(f *txtest.Fake) error {
				return errors.Join(f.WithAuthTx(context.Background(), ok), f.WithUserTx(context.Background(), b, ok))
			},
			wantRecords: []txtest.Record{auth(txtest.Committed), user(b, txtest.Committed)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := txtest.New()
			var err error
			panicked := func() (p bool) {
				defer func() { p = recover() != nil }()
				err = tt.run(f)
				return false
			}()
			if panicked != tt.wantPanic {
				t.Fatalf("panicked = %v, want %v", panicked, tt.wantPanic)
			}
			if !tt.wantPanic && !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if got := f.Records(); !slices.Equal(got, tt.wantRecords) {
				t.Errorf("records = %+v, want %+v", got, tt.wantRecords)
			}
		})
	}
}

// MustBeOutside and Require see the fake's transactions as they would see the real ones.
func TestContextMarker(t *testing.T) {
	f := txtest.New()
	a := uuid.New()
	var inUser, inAuth, requireUser, requireWrong error
	var gotUser uuid.UUID
	var finished context.Context
	_ = f.WithUserTx(context.Background(), a, func(ctx context.Context) error {
		inUser, finished = tx.MustBeOutside(ctx), ctx
		var txn *tx.Txn
		txn, requireUser = tx.Require(ctx, tx.RoleUser)
		if txn != nil {
			gotUser = txn.UserID()
		}
		_, requireWrong = tx.Require(ctx, tx.RoleAuth)
		return nil
	})
	_ = f.WithAuthTx(context.Background(), func(ctx context.Context) error { inAuth = tx.MustBeOutside(ctx); return nil })
	_, requireDone := tx.Require(finished, tx.RoleUser)
	_, requireNone := tx.Require(context.Background(), tx.RoleUser)

	for name, c := range map[string]struct{ got, want error }{
		"MustBeOutside outside":       {tx.MustBeOutside(context.Background()), nil},
		"MustBeOutside in user":       {inUser, tx.ErrInside},
		"MustBeOutside in auth":       {inAuth, tx.ErrInside},
		"MustBeOutside after finish":  {tx.MustBeOutside(finished), nil},
		"Require user in user":        {requireUser, nil},
		"Require auth in user":        {requireWrong, tx.ErrWrongRole},
		"Require after finish":        {requireDone, tx.ErrFinished},
		"Require without transaction": {requireNone, tx.ErrNoTx},
	} {
		if !errors.Is(c.got, c.want) || (c.want == nil && c.got != nil) {
			t.Errorf("%s: %v, want %v", name, c.got, c.want)
		}
	}
	if gotUser != a {
		t.Errorf("UserID = %s, want %s", gotUser, a)
	}
}
