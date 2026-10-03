// Package domain holds the account module's entities and pure rules: roles, the login limits,
// the email and password rules, password hashing and tokens. It touches no database.
package domain

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

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

// Messages of the T6 use cases (ADR-0038, ADR-0068).
const (
	// NoSuchUserMessage: an admin path names a user id that does not exist (or is not a UUID).
	NoSuchUserMessage = "there is no such user"
	// EmailTakenMessage: an invite for an email that already has an account.
	EmailTakenMessage = "this email already has an account"
	// LastOperatorMessage: removing the last operator, by an operator or by themselves.
	LastOperatorMessage = "the last operator account cannot be removed; make another account an operator first"
	// RemoveSelfMessage: an operator removing their own account through the admin path.
	RemoveSelfMessage = "you cannot remove your own account here; delete it from Settings instead"
	// WrongPasswordMessage: DELETE /api/me with a wrong password.
	WrongPasswordMessage = "the password is incorrect"
	// DisplayNameRuleMessage: a display name longer than MaxDisplayNameLength.
	DisplayNameRuleMessage = "the display name must be at most 50 characters long"
	// InviteEmailMessage: an invite email that is not one plain address.
	InviteEmailMessage = "enter one plain email address of at most 254 characters, such as name@example.com"
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

// CheckNewEmail is the rule for the email of a new account (ADR-0068): trimmed, at most
// MaxEmailLength characters, and exactly one plain address as net/mail reads it — no display
// name, no list, no angle brackets, nothing net/mail would rewrite. It returns the trimmed email
// or an invalid_input error.
func CheckNewEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" || utf8.RuneCountInString(email) > MaxEmailLength {
		return "", apperr.New(apperr.InvalidInput, InviteEmailMessage)
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" || addr.Address != email {
		return "", apperr.New(apperr.InvalidInput, InviteEmailMessage)
	}
	return email, nil
}

// MaxDisplayNameLength is the longest display name, in characters (runes; ADR-0068).
const MaxDisplayNameLength = 50

// NormalizeDisplayName trims spaces around name and returns an invalid_input error when the
// result is longer than MaxDisplayNameLength characters. An empty name is allowed.
func NormalizeDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		return "", apperr.New(apperr.InvalidInput, DisplayNameRuleMessage)
	}
	return name, nil
}

// Status is where an account stands, as the operator's list shows it (ADR-0068).
type Status string

// The statuses.
const (
	// StatusInvited: no password has been set yet.
	StatusInvited Status = "invited"
	// StatusActive: a password has been set.
	StatusActive Status = "active"
)

// StatusOf returns the status of an account with or without a password.
func StatusOf(hasPassword bool) Status {
	if hasPassword {
		return StatusActive
	}
	return StatusInvited
}

// Account is a user as the operator's list shows it. It never holds a password hash, a token or
// financial data (ADR-0019).
type Account struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Role        Role
	Status      Status
	CreatedAt   time.Time
	// LastActiveAt is the latest last use of any of the user's sessions; nil without one.
	LastActiveAt *time.Time
}

// Limited reports whether failed-attempt counts reach the login limit.
func Limited(failuresForEmail, failuresForIP int) bool {
	return failuresForEmail >= MaxFailuresPerEmail || failuresForIP >= MaxFailuresPerIP
}
