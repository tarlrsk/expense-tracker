package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db/dbtest"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

var errBoom = errors.New("boom")

// Inside WithUserTx the connection is app_user acting for the user, under the configured
// statement timeout; nil commits, an error or a panic rolls back.
func TestWithUserTx(t *testing.T) {
	d := testDB(t)
	user := newUser(t, d)

	t.Run("session", func(t *testing.T) {
		err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
			c, err := UserConn(ctx)
			if err != nil {
				return err
			}
			var cur, sess, uid, timeout string
			if err := c.Raw(`select current_user, session_user, app.current_user_id()::text,
				current_setting('statement_timeout')`).Row().Scan(&cur, &sess, &uid, &timeout); err != nil {
				return err
			}
			if cur != "app_user" || sess != dbtest.LoginRole || uid != user.String() || timeout != "7s" {
				t.Errorf("current_user %q, session_user %q, user %q, statement_timeout %q; want app_user, %s, %s, 7s",
					cur, sess, uid, timeout, dbtest.LoginRole, user)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	tests := []struct {
		name     string
		fn       func(ctx context.Context) error
		wantErr  error
		panics   bool
		wantName string
	}{
		{name: "nil commits", fn: func(ctx context.Context) error { return rename(ctx, "Committed") }, wantName: "Committed"},
		{
			name: "error rolls back",
			fn: func(ctx context.Context) error {
				if err := rename(ctx, "Lost"); err != nil {
					return err
				}
				return errBoom
			},
			wantErr: errBoom, wantName: "Committed",
		},
		{
			name: "panic rolls back and panics again",
			fn: func(ctx context.Context) error {
				if err := rename(ctx, "Lost"); err != nil {
					return err
				}
				panic(errBoom)
			},
			panics: true, wantName: "Committed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			recovered := func() (r any) {
				defer func() { r = recover() }()
				err = d.WithUserTx(t.Context(), user, tt.fn)
				return nil
			}()
			if tt.panics != (recovered != nil) {
				t.Fatalf("panic = %v, want panic %v", recovered, tt.panics)
			}
			if !tt.panics && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got := categoryName(t, user); got != tt.wantName {
				t.Errorf("category name = %q, want %q", got, tt.wantName)
			}
		})
	}

	t.Run("nil user", func(t *testing.T) {
		called := false
		err := d.WithUserTx(t.Context(), uuid.Nil, func(context.Context) error { called = true; return nil })
		if !errors.Is(err, tx.ErrNoUser) || called {
			t.Errorf("error = %v, fn called %v; want ErrNoUser and no call", err, called)
		}
	})

	// A failed statement aborts the transaction; Postgres turns COMMIT into ROLLBACK and the
	// caller still hears about it even when fn swallowed the error.
	t.Run("swallowed statement error", func(t *testing.T) {
		err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
			if err := rename(ctx, "Lost"); err != nil {
				return err
			}
			c, err := UserConn(ctx)
			if err != nil {
				return err
			}
			_ = c.Exec("select 1/0").Error
			return nil
		})
		if err == nil {
			t.Fatal("WithUserTx returned nil after a failed statement")
		}
		if got := categoryName(t, user); got != "Committed" {
			t.Errorf("category name = %q, want it unchanged", got)
		}
	})
}

// Row-level security through the real wrapper: a user sees only their own rows.
func TestUserTxRLS(t *testing.T) {
	d := testDB(t)
	a, b := newUser(t, d), newUser(t, d)
	err := d.WithUserTx(t.Context(), a, func(ctx context.Context) error {
		c, err := UserConn(ctx)
		if err != nil {
			return err
		}
		var all, own, others int
		if err := c.Raw(`select count(*), count(*) filter (where owner_id = ?), count(*) filter (where owner_id = ?)
			from categories`, a, b).Row().Scan(&all, &own, &others); err != nil {
			return err
		}
		if all != 13 || own != 13 || others != 0 {
			t.Errorf("A sees %d categories, %d own, %d of B's; want 13, 13, 0", all, own, others)
		}
		var n int64
		if err := c.Raw("select count(*) from categories where owner_id = ?", b).Scan(&n).Error; err != nil {
			return err
		}
		if n != 0 {
			t.Errorf("A reads %d of B's categories by owner_id", n)
		}
		res := c.Exec("update categories set name = 'Hacked' where owner_id = ?", b)
		if res.Error != nil || res.RowsAffected != 0 {
			t.Errorf("A's update of B's rows: %d rows, error %v; want 0 rows", res.RowsAffected, res.Error)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := categoryName(t, b); got != "Food" {
		t.Errorf("B's category renamed to %q", got)
	}
}

// Inside WithAuthTx the connection is app_auth: account tables yes, financial tables no.
func TestWithAuthTx(t *testing.T) {
	d := testDB(t)
	err := d.WithAuthTx(t.Context(), func(ctx context.Context) error {
		c, err := AuthConn(ctx)
		if err != nil {
			return err
		}
		var cur, sess string
		var users int
		if err := c.Raw("select current_user, session_user, (select count(*) from users)").Row().Scan(&cur, &sess, &users); err != nil {
			return err
		}
		if cur != "app_auth" || sess != dbtest.LoginRole {
			t.Errorf("current_user %q, session_user %q; want app_auth, %s", cur, sess, dbtest.LoginRole)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = d.WithAuthTx(t.Context(), func(ctx context.Context) error {
		c, err := AuthConn(ctx)
		if err != nil {
			return err
		}
		var n int64
		return c.Raw("select count(*) from categories").Scan(&n).Error
	})
	if sqlState(err) != insufficientPrivilege {
		t.Errorf("app_auth reading categories: %v, want permission denied", err)
	}
}

// UserConn and AuthConn work only inside a transaction of their own role that is still open.
func TestConn(t *testing.T) {
	d := testDB(t)
	user := newUser(t, d)

	var finishedUser, finishedAuth context.Context
	if err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error { finishedUser = ctx; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := d.WithAuthTx(t.Context(), func(ctx context.Context) error { finishedAuth = ctx; return nil }); err != nil {
		t.Fatal(err)
	}

	type connFunc = func(context.Context) error
	userConn := func(ctx context.Context) error { _, err := UserConn(ctx); return err }
	authConn := func(ctx context.Context) error { _, err := AuthConn(ctx); return err }
	inUser := func(f connFunc) error {
		var got error
		if err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error { got = f(ctx); return nil }); err != nil {
			t.Fatal(err)
		}
		return got
	}
	inAuth := func(f connFunc) error {
		var got error
		if err := d.WithAuthTx(t.Context(), func(ctx context.Context) error { got = f(ctx); return nil }); err != nil {
			t.Fatal(err)
		}
		return got
	}

	tests := []struct {
		name string
		got  error
		want error
	}{
		{"UserConn outside", userConn(t.Context()), tx.ErrNoTx},
		{"AuthConn outside", authConn(t.Context()), tx.ErrNoTx},
		{"UserConn in auth", inAuth(userConn), tx.ErrWrongRole},
		{"AuthConn in user", inUser(authConn), tx.ErrWrongRole},
		{"UserConn in user", inUser(userConn), nil},
		{"AuthConn in auth", inAuth(authConn), nil},
		{"UserConn after finish", userConn(finishedUser), tx.ErrFinished},
		{"AuthConn after finish", authConn(finishedAuth), tx.ErrFinished},
		{"UserConn after auth finish", userConn(finishedAuth), tx.ErrFinished},
	}
	for _, tt := range tests {
		if !errors.Is(tt.got, tt.want) || (tt.want == nil && tt.got != nil) {
			t.Errorf("%s: error = %v, want %v", tt.name, tt.got, tt.want)
		}
	}

	// A transaction opened by another transactor (the txtest fake) has no connection here.
	fake := txtest.New()
	var inFake error
	if err := fake.WithUserTx(t.Context(), user, func(ctx context.Context) error { inFake = userConn(ctx); return nil }); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(inFake, tx.ErrNoTx) || !strings.Contains(inFake.Error(), "not opened by internal/db") {
		t.Errorf("UserConn in a fake transaction: %v, want ErrNoTx, not opened by internal/db", inFake)
	}
}

// The nesting rules of ADR-0032 on the real database.
func TestNesting(t *testing.T) {
	d := testDB(t)
	a, b := newUser(t, d), newUser(t, d)

	t.Run("same user joins", func(t *testing.T) {
		ident := func(ctx context.Context) (pid int, xid int64) {
			c, err := UserConn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.Raw("select pg_backend_pid(), txid_current()").Row().Scan(&pid, &xid); err != nil {
				t.Fatal(err)
			}
			return pid, xid
		}
		err := d.WithUserTx(t.Context(), a, func(ctx context.Context) error {
			outerPID, outerXID := ident(ctx)
			return d.WithUserTx(ctx, a, func(ctx context.Context) error {
				if pid, xid := ident(ctx); pid != outerPID || xid != outerXID {
					t.Errorf("inner call on backend %d transaction %d; outer on %d, %d", pid, xid, outerPID, outerXID)
				}
				return nil
			})
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("inner error rolls back everything", func(t *testing.T) {
		before := categoryName(t, a)
		var innerErr error
		err := d.WithUserTx(t.Context(), a, func(ctx context.Context) error {
			if err := rename(ctx, "Outer"); err != nil {
				return err
			}
			innerErr = d.WithUserTx(ctx, a, func(ctx context.Context) error {
				if err := rename(ctx, "Inner"); err != nil {
					return err
				}
				return errBoom
			})
			return nil // swallowed on purpose
		})
		if !errors.Is(innerErr, errBoom) {
			t.Errorf("inner error = %v, want errBoom", innerErr)
		}
		if !errors.Is(err, tx.ErrRolledBack) {
			t.Errorf("outer error = %v, want ErrRolledBack", err)
		}
		if got := categoryName(t, a); got != before {
			t.Errorf("category name = %q, want %q (nothing committed)", got, before)
		}
	})

	notCalled := func(t *testing.T) (func(context.Context) error, *bool) {
		called := false
		return func(context.Context) error { called = true; return nil }, &called
	}
	refused := []struct {
		name string
		run  func(t *testing.T, inner func(context.Context) error) error
		want error
	}{
		{
			name: "different user",
			run: func(t *testing.T, inner func(context.Context) error) error {
				var got error
				_ = d.WithUserTx(t.Context(), a, func(ctx context.Context) error { got = d.WithUserTx(ctx, b, inner); return nil })
				return got
			},
			want: tx.ErrOtherUser,
		},
		{
			name: "user inside auth",
			run: func(t *testing.T, inner func(context.Context) error) error {
				var got error
				_ = d.WithAuthTx(t.Context(), func(ctx context.Context) error { got = d.WithUserTx(ctx, a, inner); return nil })
				return got
			},
			want: tx.ErrWrongRole,
		},
		{
			name: "auth inside user",
			run: func(t *testing.T, inner func(context.Context) error) error {
				var got error
				_ = d.WithUserTx(t.Context(), a, func(ctx context.Context) error { got = d.WithAuthTx(ctx, inner); return nil })
				return got
			},
			want: tx.ErrWrongRole,
		},
		{
			name: "finished context",
			run: func(t *testing.T, inner func(context.Context) error) error {
				var done context.Context
				_ = d.WithUserTx(t.Context(), a, func(ctx context.Context) error { done = ctx; return nil })
				return d.WithUserTx(done, a, inner)
			},
			want: tx.ErrFinished,
		},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			inner, called := notCalled(t)
			if err := tt.run(t, inner); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
			if *called {
				t.Error("the refused fn ran")
			}
		})
	}
}

// Outside every wrapper the login role reaches no table, and a connection back in the pool is
// plain app_login again.
func TestDefaultDeny(t *testing.T) {
	d := testDB(t, func(c *Config) { c.MaxOpenConns = 1 })
	user := newUser(t, d)

	for _, table := range []string{"users", "categories", "transactions", "profiles", "sessions"} {
		var n int
		if err := rootQuery(t, d, "select count(*) from "+table, &n); sqlState(err) != insufficientPrivilege {
			t.Errorf("%s outside a transaction: %v, want permission denied", table, err)
		}
	}

	if err := d.WithUserTx(t.Context(), user, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var cur, uid, timeout string
	if err := rootQuery(t, d, `select current_user, coalesce(current_setting('app.user_id', true), ''),
		current_setting('statement_timeout')`, &cur, &uid, &timeout); err != nil {
		t.Fatal(err)
	}
	if cur != dbtest.LoginRole || uid != "" || timeout != "0" {
		t.Errorf("after the transaction: current_user %q, app.user_id %q, statement_timeout %q; want %s, empty, 0",
			cur, uid, timeout, dbtest.LoginRole)
	}
}

// The statement timeout stops a runaway query and the transaction rolls back; so does a
// cancelled context.
func TestTimeoutAndCancel(t *testing.T) {
	d := testDB(t, func(c *Config) { c.StatementTimeout = 200 * time.Millisecond })
	user := newUser(t, d)
	before := categoryName(t, user)

	err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
		if err := rename(ctx, "Lost"); err != nil {
			return err
		}
		c, err := UserConn(ctx)
		if err != nil {
			return err
		}
		return c.Exec("select pg_sleep(5)").Error
	})
	if sqlState(err) != queryCanceled {
		t.Errorf("slow statement: %v, want SQLSTATE %s", err, queryCanceled)
	}
	if got := categoryName(t, user); got != before {
		t.Errorf("category name = %q after the timeout, want %q", got, before)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err = d.WithUserTx(ctx, user, func(ctx context.Context) error {
		if err := rename(ctx, "Lost"); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("WithUserTx after cancel: %v, want context.Canceled", err)
	}
	if got := categoryName(t, user); got != before {
		t.Errorf("category name = %q after cancel, want %q", got, before)
	}

	// The request deadline passes while fn works; fn returns nil, nothing is committed and the
	// caller can tell it was the deadline (ADR-0043).
	ctx, cancel = context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	err = d.WithUserTx(ctx, user, func(ctx context.Context) error {
		if err := rename(ctx, "Lost"); err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WithUserTx after the deadline: %v, want context.DeadlineExceeded", err)
	}
	if got := categoryName(t, user); got != before {
		t.Errorf("category name = %q after the deadline, want %q", got, before)
	}
}

// cleanConnection checks that the next connection from a pool of one is plain app_login: no
// role switched, no app.user_id, no statement timeout and no transaction left open.
func cleanConnection(t *testing.T, d *DB) {
	t.Helper()
	var cur, uid, timeout string
	if err := rootQuery(t, d, `select current_user, coalesce(current_setting('app.user_id', true), ''),
		current_setting('statement_timeout')`, &cur, &uid, &timeout); err != nil {
		t.Fatalf("next connection: %v", err)
	}
	if cur != dbtest.LoginRole || uid != "" || timeout != "0" {
		t.Errorf("next connection: current_user %q, app.user_id %q, statement_timeout %q; want %s, empty, 0",
			cur, uid, timeout, dbtest.LoginRole)
	}
	// SAVEPOINT works only inside a transaction block.
	if err := d.root.WithContext(t.Context()).Exec("savepoint clean_check").Error; sqlState(err) != noActiveTransaction {
		t.Errorf("savepoint on the next connection: %v, want SQLSTATE %s (no transaction open)", err, noActiveTransaction)
	}
}

// After a cancel during a statement, a statement timeout or a panic, the connection that goes
// back to the pool (or its replacement) is plain app_login again.
func TestConnectionAfterFailure(t *testing.T) {
	d := testDB(t, func(c *Config) { c.MaxOpenConns = 1; c.StatementTimeout = 300 * time.Millisecond })
	user := newUser(t, d)
	sleep := func(ctx context.Context) error {
		c, err := UserConn(ctx)
		if err != nil {
			return err
		}
		return c.Exec("select pg_sleep(5)").Error
	}

	t.Run("cancel during a statement", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		timer := time.AfterFunc(100*time.Millisecond, cancel)
		defer timer.Stop()
		if err := d.WithUserTx(ctx, user, sleep); !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
		cleanConnection(t, d)
	})

	t.Run("statement timeout", func(t *testing.T) {
		if err := d.WithUserTx(t.Context(), user, sleep); sqlState(err) != queryCanceled {
			t.Errorf("error = %v, want SQLSTATE %s", err, queryCanceled)
		}
		cleanConnection(t, d)
	})

	t.Run("panic", func(t *testing.T) {
		func() {
			defer func() { _ = recover() }()
			_ = d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
				if err := rename(ctx, "Lost"); err != nil {
					return err
				}
				panic(errBoom)
			})
		}()
		cleanConnection(t, d)
	})
}

// A pooled connection whose session was changed without LOCAL (set_config(..., false), as SET
// ROLE without LOCAL would) still gives every transaction the role and user it asked for: the
// begin statement overrides both. The session is poisoned as app_login, through a raw connection
// of the pool of one, so the transactions below run on that same connection.
func TestStaleSessionOverridden(t *testing.T) {
	d := testDB(t, func(c *Config) { c.MaxOpenConns = 1 })
	a, b := newUser(t, d), newUser(t, d)

	for _, poisonRole := range []string{"app_user", "app_auth"} {
		t.Run("session role "+poisonRole, func(t *testing.T) {
			conn, err := d.sqlDB.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			var poisonedPID int
			if err := conn.QueryRowContext(t.Context(), `select pg_backend_pid()
				from (select set_config('role', $1, false), set_config('app.user_id', $2, false)) s`,
				poisonRole, a.String()).Scan(&poisonedPID); err != nil {
				t.Fatal(err)
			}
			if err := conn.Close(); err != nil { // back to the pool, still poisoned
				t.Fatal(err)
			}

			err = d.WithUserTx(t.Context(), b, func(ctx context.Context) error {
				c, err := UserConn(ctx)
				if err != nil {
					return err
				}
				var pid, all, ofA int
				var cur, uid string
				if err := c.Raw(`select pg_backend_pid(), current_user, app.current_user_id()::text,
					(select count(*) from categories), (select count(*) from categories where owner_id = ?)`, a).
					Row().Scan(&pid, &cur, &uid, &all, &ofA); err != nil {
					return err
				}
				if pid != poisonedPID {
					t.Fatalf("ran on backend %d, not on the poisoned %d", pid, poisonedPID)
				}
				if cur != "app_user" || uid != b.String() || all != 13 || ofA != 0 {
					t.Errorf("current_user %q, user %q, %d categories, %d of A's; want app_user, %s, 13, 0",
						cur, uid, all, ofA, b)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			err = d.WithAuthTx(t.Context(), func(ctx context.Context) error {
				c, err := AuthConn(ctx)
				if err != nil {
					return err
				}
				var cur string
				if err := c.Raw("select current_user").Row().Scan(&cur); err != nil {
					return err
				}
				if cur != "app_auth" {
					t.Errorf("auth transaction runs as %q, want app_auth", cur)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// UserOnly, the value app puts in registry.Deps, opens user transactions but is not a tx.Auth.
func TestUserOnly(t *testing.T) {
	u := (&DB{}).UserOnly()
	if _, ok := u.(tx.Auth); ok {
		t.Error("UserOnly() implements tx.Auth")
	}
	if _, ok := u.(*DB); ok {
		t.Error("UserOnly() is the *DB itself")
	}
}

// Outside calls are refused only while a transaction is open.
func TestMustBeOutside(t *testing.T) {
	d := testDB(t)
	user := newUser(t, d)

	var inUser, inAuth error
	var finished context.Context
	if err := d.WithUserTx(t.Context(), user, func(ctx context.Context) error {
		inUser, finished = tx.MustBeOutside(ctx), ctx
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.WithAuthTx(t.Context(), func(ctx context.Context) error { inAuth = tx.MustBeOutside(ctx); return nil }); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct{ got, want error }{
		"outside":        {tx.MustBeOutside(t.Context()), nil},
		"in user tx":     {inUser, tx.ErrInside},
		"in auth tx":     {inAuth, tx.ErrInside},
		"after finished": {tx.MustBeOutside(finished), nil},
	} {
		if !errors.Is(c.got, c.want) || (c.want == nil && c.got != nil) {
			t.Errorf("%s: %v, want %v", name, c.got, c.want)
		}
	}
}

// Open refuses a login role that could read data without switching, and a database without
// tables. No error text contains the password.
func TestStartupCheck(t *testing.T) {
	ctx := t.Context()
	cfg := Config{StatementTimeout: testTimeout}
	const grants = "grant app_user, app_auth to %s with inherit false, set true"

	t.Run("app_login accepted", func(t *testing.T) {
		d, err := open(ctx, dbtest.LoginConfig(t), cfg)
		if err != nil {
			t.Fatalf("app_login refused: %v", err)
		}
		_ = d.Close()
	})

	t.Run("superuser refused", func(t *testing.T) {
		_, err := open(ctx, dbtest.Config(t), cfg)
		if !errors.Is(err, ErrRoleTooPowerful) || !strings.Contains(err.Error(), "superuser") {
			t.Errorf("superuser: %v, want ErrRoleTooPowerful naming superuser", err)
		}
	})

	refused := []struct {
		name, attributes string
		extra            []string
		want             string
	}{
		{name: "bypassrls", attributes: "bypassrls noinherit", extra: []string{grants}, want: "BYPASSRLS"},
		{name: "replication", attributes: "replication noinherit", extra: []string{grants}, want: "has REPLICATION"},
		{name: "createrole", attributes: "createrole noinherit", extra: []string{grants}, want: "has CREATEROLE"},
		{name: "createdb", attributes: "createdb noinherit", extra: []string{grants}, want: "has CREATEDB"},
		{
			name: "direct select on users", attributes: "noinherit",
			extra: []string{grants, "grant select on public.users to %s"}, want: "can read public.users without",
		},
		{
			name: "direct select on transactions", attributes: "noinherit",
			extra: []string{grants, "grant select on public.transactions to %s"}, want: "can read public.transactions without",
		},
		{
			name: "direct select on sessions", attributes: "noinherit",
			extra: []string{grants, "grant select on public.sessions to %s"}, want: "can read public.sessions without",
		},
		{name: "inherits app_auth", attributes: "inherit", extra: []string{"grant app_user, app_auth to %s with inherit true, set true"}, want: "can read"},
		{
			name: "inherit option without reading", attributes: "noinherit",
			extra: []string{"grant app_user, app_auth to %s with inherit true, set true"}, want: "memberships other than SET",
		},
		{
			name: "admin on app_user", attributes: "noinherit",
			extra: []string{grants, "grant app_user to %s with admin true"}, want: "memberships other than SET without INHERIT or ADMIN on app_user and app_auth: app_user",
		},
		{
			name: "SET on pg_execute_server_program", attributes: "noinherit",
			extra: []string{grants, "grant pg_execute_server_program to %s with inherit false, set true"},
			want:  "memberships other than SET without INHERIT or ADMIN on app_user and app_auth: pg_execute_server_program",
		},
		{name: "cannot switch", attributes: "noinherit", want: "cannot SET ROLE"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			connURL, password := throwAwayRole(t, tt.attributes, "", tt.extra...)
			d, err := Open(ctx, Config{URL: connURL, StatementTimeout: testTimeout})
			if err == nil {
				_ = d.Close()
				t.Fatal("accepted")
			}
			if !errors.Is(err, ErrRoleTooPowerful) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want ErrRoleTooPowerful mentioning %q", err, tt.want)
			}
			if strings.Contains(err.Error(), password) {
				t.Errorf("error contains the password: %v", err)
			}
		})
	}

	// A role that owns the tables, in a database of its own (the shared test database's tables
	// stay with the migration owner).
	t.Run("owner of the tables", func(t *testing.T) {
		name := throwAwayDB(t)
		connURL, password := throwAwayRole(t, "noinherit", name, grants)
		role := roleOf(t, connURL)
		db := superuserOn(t, name)
		t.Cleanup(func() { // before the role is dropped
			if _, err := db.ExecContext(context.WithoutCancel(t.Context()), "drop owned by "+ident(role)); err != nil {
				t.Errorf("drop owned: %v", err)
			}
		})
		for _, table := range migratedTables {
			for _, q := range []string{"create table " + table + " ()", "alter table " + table + " owner to " + ident(role)} {
				if _, err := db.ExecContext(t.Context(), q); err != nil {
					t.Fatalf("%s: %v", q, err)
				}
			}
		}
		_, err := Open(ctx, Config{URL: connURL, StatementTimeout: testTimeout})
		if !errors.Is(err, ErrRoleTooPowerful) || !strings.Contains(err.Error(), "owns, or belongs to the owner of, public.categories") {
			t.Errorf("error = %v, want ErrRoleTooPowerful naming the owned tables", err)
		}
		if err != nil && strings.Contains(err.Error(), password) {
			t.Errorf("error contains the password: %v", err)
		}
	})

	t.Run("not migrated", func(t *testing.T) {
		login := dbtest.LoginConfig(t)
		login.Database = throwAwayDB(t)
		_, err := open(ctx, login, cfg)
		if !errors.Is(err, ErrNotMigrated) || !strings.Contains(err.Error(), "make migrate") {
			t.Errorf("error = %v, want ErrNotMigrated", err)
		}
	})

	// Only users and transactions exist: every table of the migrations is required.
	t.Run("partly migrated", func(t *testing.T) {
		login := dbtest.LoginConfig(t)
		login.Database = throwAwayDB(t)
		db := superuserOn(t, login.Database)
		for _, q := range []string{"create table public.users ()", "create table public.transactions ()"} {
			if _, err := db.ExecContext(t.Context(), q); err != nil {
				t.Fatalf("%s: %v", q, err)
			}
		}
		_, err := open(ctx, login, cfg)
		if !errors.Is(err, ErrNotMigrated) || !strings.Contains(err.Error(), "public.categories") ||
			strings.Contains(err.Error(), "public.users") {
			t.Errorf("error = %v, want ErrNotMigrated naming the missing tables only", err)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		connURL, password := throwAwayRole(t, "noinherit", "", grants)
		wrong := strings.Replace(connURL, password, "x"+password[1:], 1)
		_, err := Open(ctx, Config{URL: wrong, StatementTimeout: testTimeout})
		if err == nil {
			t.Fatal("connected with a wrong password")
		}
		if strings.Contains(err.Error(), password[1:]) {
			t.Errorf("error contains the password: %v", err)
		}
	})

	t.Run("bad settings", func(t *testing.T) {
		const secret = "s3cret-pw"
		for _, c := range []Config{
			{URL: "", StatementTimeout: testTimeout},
			{URL: "postgres://u:" + secret + "@[bad", StatementTimeout: testTimeout},
			{URL: "postgres://u:" + secret + "@127.0.0.1:1/x_test", StatementTimeout: 0},
		} {
			_, err := Open(ctx, c)
			if err == nil {
				t.Errorf("Open accepted %+v", c.StatementTimeout)
				continue
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error contains the password: %v", err)
			}
		}
	})
}
