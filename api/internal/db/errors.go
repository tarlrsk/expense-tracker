package db

import (
	"strings"

	"gorm.io/gorm"
)

// Err returns the error of a GORM result in a form that carries no row value (ADR-0067). Every
// database adaptor returns its database errors only through Err; a lint rule (forbidigo in
// .golangci.yml) forbids reading gorm.DB.Error, Row, Rows or ScanRows anywhere else.
//
// The text is hidden but the kind is kept (valueFree):
//   - errors.Is still matches gorm.ErrRecordNotFound, gorm.ErrDuplicatedKey, the other fixed GORM
//     and database/sql errors and the context errors;
//   - a Postgres error becomes a *PgError with its SQLSTATE and constraint name, and its message
//     only for the classes known to hold no value (messageClasses);
//   - anything else (a scan or conversion error, for example) becomes a fixed text with its Go type.
//
// Err returns nil when the result has no error.
func Err(result *gorm.DB) error {
	if result == nil {
		return nil
	}
	return valueFree(result.Error)
}

// PgError is a Postgres error without any row value: its SQLSTATE, the constraint it names (if
// any) and, only for the SQLSTATE classes in messageClasses, the server's message.
type PgError struct {
	// Code is the SQLSTATE, for example 23505 (unique violation).
	Code string
	// Constraint is the name of the violated constraint, or "".
	Constraint string
	// Message is the server's message for value-free classes, otherwise "".
	Message string
}

func (e *PgError) Error() string {
	var b strings.Builder
	b.WriteString("postgres error SQLSTATE ")
	b.WriteString(e.Code)
	if e.Constraint != "" {
		b.WriteString(" constraint ")
		b.WriteString(e.Constraint)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	return b.String()
}

// SQLState returns the SQLSTATE, like *pgconn.PgError does (tx names a rolled-back cause by it).
func (e *PgError) SQLState() string { return e.Code }

// kindByCode maps SQLSTATEs to the GORM errors that GORM's TranslateError would give. GORM
// translates only some statements (not Raw or Exec), so PgError matches them itself.
var kindByCode = map[string]error{
	"23505": gorm.ErrDuplicatedKey,
	"23503": gorm.ErrForeignKeyViolated,
	"23514": gorm.ErrCheckConstraintViolated,
}

// Is makes errors.Is(err, gorm.ErrDuplicatedKey) (and the foreign-key and check-constraint
// errors) hold for the matching SQLSTATE, whether or not GORM translated the error.
func (e *PgError) Is(target error) bool {
	kind, ok := kindByCode[e.Code]
	return ok && kind == target //nolint:errorlint // comparing sentinels on purpose
}
