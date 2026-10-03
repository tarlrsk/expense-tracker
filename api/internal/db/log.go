package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newLogger returns GORM's slog logger set to log failed statements (error level) and statements
// slower than slow (warn level), never successful ones. Statements are logged with their
// placeholders ($1, $2 ...) and never with parameter values: amounts and merchant names are
// private. Errors are logged only in a value-free form (valueFree): a Postgres error message can
// quote a value (invalid input syntax for type uuid: "..."), and so can a scan error.
func newLogger(logger *slog.Logger, slow time.Duration) gormlogger.Interface {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return valueFreeLogger{gormlogger.NewSlogLogger(logger, gormlogger.Config{
		SlowThreshold:             slow,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		LogLevel:                  gormlogger.Warn,
	})}
}

// GORM's Scan logs through a separate recorder that ignores the logger's ParamsFilter and asks
// this package-level hook instead (default: keep the values). Only Scan uses it, for logging
// only, and only this package uses GORM, so it is set once for the process.
func init() {
	gormlogger.RecorderParamsFilter = func(_ context.Context, sql string, _ ...any) (string, []any) {
		return sql, nil
	}
}

// explainedPlaceholder is how GORM's Postgres dialector renders $1 when it has no values to put
// in ($1$); Trace turns it back into $1.
var explainedPlaceholder = regexp.MustCompile(`\$(\d+)\$`)

// valueFreeLogger wraps GORM's slog logger and logs every error in its value-free form.
type valueFreeLogger struct {
	gormlogger.Interface
}

// paramsFilter is GORM's ParamsFilter interface (gorm.ParamsFilter): GORM asks the logger which
// parameter values to put into the logged SQL.
type paramsFilter interface {
	ParamsFilter(ctx context.Context, sql string, params ...any) (string, []any)
}

func (l valueFreeLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return valueFreeLogger{l.Interface.LogMode(level)}
}

func (l valueFreeLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.Interface.Trace(ctx, begin, func() (string, int64) {
		sql, rows := fc()
		return explainedPlaceholder.ReplaceAllString(sql, "$$$1"), rows
	}, valueFree(err))
}

// ParamsFilter never lets a parameter value into the log, whatever the wrapped logger says.
func (l valueFreeLogger) ParamsFilter(ctx context.Context, sql string, params ...any) (string, []any) {
	if f, ok := l.Interface.(paramsFilter); ok {
		sql, _ = f.ParamsFilter(ctx, sql, params...)
	}
	return sql, nil
}

// messageClasses are the SQLSTATE classes whose messages name only SQL objects and server
// states, never row values: connection (08), transaction state (25), authorization (28),
// rollback (40), syntax or access rule (42), resources (53, 54), object state (55), operator
// intervention such as statement timeout (57) and system errors (58). Other classes, for example
// data exceptions (22: invalid input syntax for type uuid: "..."), are logged by code only.
var messageClasses = []string{"08", "25", "28", "40", "42", "53", "54", "55", "57", "58"}

// keptErrors have fixed texts that carry no row value. valueFree returns the matching one itself,
// never the error it found it in, whose text may add more. They are: the context's end (a
// request deadline or cancel), database/sql's and the driver's connection and transaction states,
// a connection that ended (io.EOF), and GORM's own errors, including those it translates
// Postgres errors into (TranslateError: duplicated key, foreign key, check constraint).
var keptErrors = []error{
	context.Canceled, context.DeadlineExceeded,
	sql.ErrNoRows, sql.ErrTxDone, sql.ErrConnDone, driver.ErrBadConn,
	io.EOF, io.ErrUnexpectedEOF,
	gorm.ErrRecordNotFound, gorm.ErrInvalidTransaction, gorm.ErrNotImplemented, gorm.ErrMissingWhereClause,
	gorm.ErrUnsupportedRelation, gorm.ErrPrimaryKeyRequired, gorm.ErrModelValueRequired,
	gorm.ErrModelAccessibleFieldsRequired, gorm.ErrSubQueryRequired, gorm.ErrInvalidData,
	gorm.ErrUnsupportedDriver, gorm.ErrRegistered, gorm.ErrInvalidField, gorm.ErrEmptySlice,
	gorm.ErrDryRunModeUnsupported, gorm.ErrInvalidDB, gorm.ErrInvalidValue, gorm.ErrInvalidValueOfLength,
	gorm.ErrPreloadNotAllowed, gorm.ErrDuplicatedKey, gorm.ErrForeignKeyViolated, gorm.ErrCheckConstraintViolated,
}

// valueFree returns err in a form that carries no row value. It is an allowlist:
//   - a Postgres error becomes its SQLSTATE, constraint name and, for the classes above, its
//     message;
//   - an error in keptErrors becomes that error;
//   - a failure to connect (*pgconn.ConnectError: host, user and database names and the reason)
//     or a network error (*net.OpError: addresses and the system error) becomes that error;
//   - anything else, a scan or conversion error for example (converting driver.Value type
//     string ("9876.54") to a int64), is replaced by a fixed text and its Go type.
func valueFree(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgError(pgErr)
	}
	for _, kept := range keptErrors {
		if errors.Is(err, kept) {
			return kept
		}
	}
	var connErr *pgconn.ConnectError
	if errors.As(err, &connErr) {
		return connErr
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return netErr
	}
	return fmt.Errorf("error text hidden, it may quote a value: %T", err)
}

// pgError is a Postgres error by its SQLSTATE, constraint name and, for the classes in
// messageClasses, its message.
func pgError(pgErr *pgconn.PgError) *PgError {
	e := &PgError{Code: pgErr.Code, Constraint: pgErr.ConstraintName}
	if len(pgErr.Code) == 5 && slices.Contains(messageClasses, pgErr.Code[:2]) {
		e.Message = pgErr.Message
	}
	return e
}
