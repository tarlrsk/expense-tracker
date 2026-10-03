// Package domain holds the account module's entities and pure rules: roles, the login limits,
// the email and password rules, password hashing and tokens. It touches no database.
package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// Role is a profile's role (ADR-0056).
type Role string

// The roles.
const (
	RoleUser     Role = "user"
	RoleOperator Role = "operator"
)

// Sessions (ADR-0037, ADR-0066).
const (
	// SessionLifetime: a session expires this long after its last use.
	SessionLifetime = 30 * 24 * time.Hour
	// SessionTouchInterval: the session check moves last_used_at and expires_at only when
	// last_used_at is older than this, so a busy session is written at most once an hour.
	SessionTouchInterval = time.Hour
)

// Login rate limit (ADR-0037, ADR-0066).
const (
	// LoginWindow is how far back failed attempts are counted.
	LoginWindow = 15 * time.Minute
	// MaxFailuresPerEmail and MaxFailuresPerIP: at this many failures in the window the next
	// attempt is refused with 429 without checking the password.
	MaxFailuresPerEmail = 5
	MaxFailuresPerIP    = 20
	// AttemptRetention: each failed login deletes attempts older than this.
	AttemptRetention = 24 * time.Hour
)

// Client messages that must be the same for every case they cover (ADR-0066).
const (
	// LoginFailedMessage: unknown email, wrong password, invite not accepted, disabled account.
	LoginFailedMessage = "the email or password is incorrect"
	// UnauthenticatedMessage: no usable session token, whatever the reason.
	UnauthenticatedMessage = "log in to continue"
	// RateLimitedMessage: too many failed attempts.
	RateLimitedMessage = "too many failed attempts; try again in 15 minutes"
	// LinkInvalidMessage: unknown, used or expired link, or a disabled user.
	LinkInvalidMessage = "this link is invalid or has expired"
	// WrongCurrentPasswordMessage: POST /api/me/password with a wrong current password.
	WrongCurrentPasswordMessage = "the current password is incorrect"
)

// MaxEmailLength is the longest email accepted, in characters.
const MaxEmailLength = 254

// NormalizeEmail trims spaces around email and returns an invalid_input error when the result
// is empty or longer than MaxEmailLength characters. Case is left alone: the column is citext.
func NormalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || utf8.RuneCountInString(email) > MaxEmailLength {
		return "", apperr.New(apperr.InvalidInput, "enter an email address of at most 254 characters")
	}
	return email, nil
}

// Limited reports whether failed-attempt counts reach the login limit.
func Limited(failuresForEmail, failuresForIP int) bool {
	return failuresForEmail >= MaxFailuresPerEmail || failuresForIP >= MaxFailuresPerIP
}
