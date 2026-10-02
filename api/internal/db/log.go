package db

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	gormlogger "gorm.io/gorm/logger"
)

// newLogger returns GORM's slog logger set to log failed statements (error level) and statements
// slower than slow (warn level), never successful ones. Statements are logged with their
// placeholders ($1, $2 ...) and never with parameter values: amounts and merchant names are
// private. A Postgres error message can quote a value too (invalid input syntax for type uuid:
// "..."), so the error is logged by its SQLSTATE and constraint only.
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

// valueFreeLogger wraps GORM's slog logger and replaces Postgres errors by their code.
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

// valueFree replaces a Postgres error by its SQLSTATE, constraint name and, for the classes
// above, its message. Other errors (record not found, context cancelled, connection failures)
// carry no row values and are kept.
func valueFree(err error) error {
	var pgErr *pgconn.PgError
	if err == nil || !errors.As(err, &pgErr) {
		return err
	}
	var b strings.Builder
	b.WriteString("postgres error SQLSTATE ")
	b.WriteString(pgErr.Code)
	if pgErr.ConstraintName != "" {
		b.WriteString(" constraint ")
		b.WriteString(pgErr.ConstraintName)
	}
	if len(pgErr.Code) == 5 && slices.Contains(messageClasses, pgErr.Code[:2]) {
		b.WriteString(": ")
		b.WriteString(pgErr.Message)
	}
	return errors.New(b.String())
}
