package db

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
)

// Err hides the text of a database error and keeps its kind (ADR-0067), on real errors from the
// test database.
func TestErrOnRealErrors(t *testing.T) {
	d := testDB(t)
	const secret = "SECRET-31415"
	existing := newUser(t, d)
	var existingEmail string
	if err := dbtest.DB(t).QueryRowContext(t.Context(), "select email::text from users where id = $1", existing).
		Scan(&existingEmail); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name           string
		query          func(c *gorm.DB) *gorm.DB
		wantIs         error  // errors.Is must hold
		wantState      string // SQLSTATE of the *PgError, when set
		wantConstraint string
	}{
		{
			name: "invalid uuid quotes the value (22P02)",
			query: func(c *gorm.DB) *gorm.DB {
				var r struct{ ID uuid.UUID }
				return c.Raw("select ?::uuid as id", secret).Scan(&r)
			},
			wantState: "22P02",
		},
		{
			name: "scan error quotes the value",
			query: func(c *gorm.DB) *gorm.DB {
				var r struct{ N int64 }
				return c.Raw("select ?::text as n", secret).Scan(&r)
			},
		},
		{
			name: "record not found",
			query: func(c *gorm.DB) *gorm.DB {
				var r struct{ ID uuid.UUID }
				return c.Table("users").Select("id").Where("id = ?", uuid.New()).Take(&r)
			},
			wantIs: gorm.ErrRecordNotFound,
		},
		{
			name: "duplicate key",
			query: func(c *gorm.DB) *gorm.DB {
				return c.Exec("insert into users (email) values (?)", existingEmail)
			},
			wantIs: gorm.ErrDuplicatedKey, wantState: "23505", wantConstraint: "users_email_key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got error
			_ = d.WithAuthTx(t.Context(), func(ctx context.Context) error {
				c, err := AuthConn(ctx)
				if err != nil {
					return err
				}
				got = Err(tt.query(c))
				return got
			})
			if got == nil {
				t.Fatal("Err = nil, want an error")
			}
			if strings.Contains(got.Error(), secret) || strings.Contains(got.Error(), existingEmail) {
				t.Errorf("Err kept a value: %v", got)
			}
			if tt.wantIs != nil && !errors.Is(got, tt.wantIs) {
				t.Errorf("Err = %v, want errors.Is %v", got, tt.wantIs)
			}
			var pgErr *PgError
			if tt.wantState != "" && (!errors.As(got, &pgErr) || pgErr.SQLState() != tt.wantState || pgErr.Message != "") {
				t.Errorf("Err = %#v, want a *PgError with SQLSTATE %s and no message", got, tt.wantState)
			}
			if tt.wantConstraint != "" && (pgErr == nil || pgErr.Constraint != tt.wantConstraint) {
				t.Errorf("Err = %#v, want constraint %s", got, tt.wantConstraint)
			}
			if tt.wantState == "" && tt.wantIs == nil && !strings.Contains(got.Error(), "error text hidden") {
				t.Errorf("Err = %v, want the hidden-text form", got)
			}
		})
	}

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var got error
		_ = d.WithAuthTx(ctx, func(ctx context.Context) error {
			c, err := AuthConn(ctx)
			if err != nil {
				return err
			}
			cancel()
			got = Err(c.Exec("select pg_sleep(1)"))
			return got
		})
		if !errors.Is(got, context.Canceled) {
			t.Errorf("Err = %v, want context.Canceled", got)
		}
	})

	t.Run("no error", func(t *testing.T) {
		err := d.WithAuthTx(t.Context(), func(ctx context.Context) error {
			c, err := AuthConn(ctx)
			if err != nil {
				return err
			}
			return Err(c.Exec("select 1"))
		})
		if err != nil {
			t.Errorf("Err = %v, want nil", err)
		}
	})
}

// Without a database: nil results, and the constraint name and value-free message classes.
func TestErr(t *testing.T) {
	if Err(nil) != nil || Err(&gorm.DB{}) != nil {
		t.Error("Err of a result without an error is not nil")
	}
	got := Err(&gorm.DB{Error: &pgconn.PgError{
		Code: "23514", ConstraintName: "users_email_check", Message: `new row violates check: "SECRET"`,
	}})
	var pgErr *PgError
	if !errors.As(got, &pgErr) || pgErr.Code != "23514" || pgErr.Constraint != "users_email_check" || pgErr.Message != "" {
		t.Errorf("Err = %#v", got)
	}
	if !errors.Is(got, gorm.ErrCheckConstraintViolated) || errors.Is(got, gorm.ErrDuplicatedKey) {
		t.Errorf("Err = %v: kind lost or wrong", got)
	}
	if strings.Contains(got.Error(), "SECRET") || !strings.Contains(got.Error(), "constraint users_email_check") {
		t.Errorf("text = %q", got.Error())
	}
	got = Err(&gorm.DB{Error: &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"}})
	if !errors.As(got, &pgErr) || pgErr.Message != "canceling statement due to statement timeout" {
		t.Errorf("Err = %#v, want the message of a value-free class", got)
	}
}
