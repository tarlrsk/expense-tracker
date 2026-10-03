// Package domain holds the categories module's entities and pure rules: kinds, the name and icon
// rules, the per-user limit and the client messages (ADR-0039, ADR-0061, ADR-0069). It touches no
// database.
package domain

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
)

// Kind says whether a category's transactions are expenses or income. It is set at creation and
// never changes (ADR-0039).
type Kind string

// The kinds.
const (
	KindExpense Kind = "expense"
	KindIncome  Kind = "income"
)

// Category is one of a user's categories. It has no owner field: categories stay per user
// (docs/05-roadmap.md groups guardrails), and every query runs for the caller only.
type Category struct {
	ID        uuid.UUID
	Name      string
	Icon      string
	Kind      Kind
	Archived  bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Limits (ADR-0069).
const (
	// MaxNameLength is the longest name, in characters (runes).
	MaxNameLength = 50
	// MaxIconLength is the longest icon, in characters (runes).
	MaxIconLength = 32
	// MaxCategories is how many categories one user may have, archived ones included.
	MaxCategories = 200
)

// Client messages (ADR-0069).
const (
	// NameRuleMessage: a name that is empty, too long or has a control character.
	NameRuleMessage = "the name must have 1 to 50 characters and no control characters"
	// NameTakenMessage: another non-archived category of the user has the name, ignoring case.
	NameTakenMessage = "a category with this name already exists"
	// KindRuleMessage: a kind other than expense or income.
	KindRuleMessage = "the kind must be expense or income"
	// IconRuleMessage: an icon longer than MaxIconLength.
	IconRuleMessage = "the icon must be at most 32 characters long"
	// LimitMessage: creating a category beyond MaxCategories.
	LimitMessage = "the limit of 200 categories is reached; archived categories count too"
	// NotFoundMessage: an unknown id, another user's id, or an id that is not a UUID, alike.
	NotFoundMessage = "there is no such category"
	// NoChangeMessage: a PATCH body without any field.
	NoChangeMessage = "send at least one of name, icon or archived"
	// IDsRequiredMessage: a reorder body without ids.
	IDsRequiredMessage = "ids is required"
	// RepeatedIDMessage: a reorder list that names a category twice.
	RepeatedIDMessage = "each category may appear only once in ids"
	// OrderChangedMessage: a reorder list that is not exactly the user's non-archived categories.
	OrderChangedMessage = "the category list has changed; reload and try again"
)

// ParseKind returns kind as a Kind, or an invalid_input error when it is not one.
func ParseKind(kind string) (Kind, error) {
	switch k := Kind(kind); k {
	case KindExpense, KindIncome:
		return k, nil
	default:
		return "", apperr.New(apperr.InvalidInput, KindRuleMessage)
	}
}

// NormalizeName applies the name rule (ADR-0069): leading and trailing whitespace is removed and
// every inner run of whitespace (any Unicode space, tabs and line breaks included) becomes one
// plain space; the result must have 1 to MaxNameLength characters and no other control
// character. It returns the normalised name or an invalid_input error.
func NormalizeName(name string) (string, error) {
	var b strings.Builder
	space := false
	for _, r := range name {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if unicode.IsControl(r) {
			return "", apperr.New(apperr.InvalidInput, NameRuleMessage)
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	out := b.String()
	if n := utf8.RuneCountInString(out); n == 0 || n > MaxNameLength {
		return "", apperr.New(apperr.InvalidInput, NameRuleMessage)
	}
	return out, nil
}

// NormalizeIcon trims whitespace around icon and returns an invalid_input error when the result
// is longer than MaxIconLength characters or holds a control character. An empty icon (none) is allowed. The icon set is
// decided later (PLAN-0002 T9); until then an icon is free text.
func NormalizeIcon(icon string) (string, error) {
	icon = strings.TrimSpace(icon)
	if utf8.RuneCountInString(icon) > MaxIconLength || strings.ContainsFunc(icon, unicode.IsControl) {
		return "", apperr.New(apperr.InvalidInput, IconRuleMessage)
	}
	return icon, nil
}
