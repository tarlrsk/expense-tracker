// Package parse is the outside service that reads quick entry text with the AI (ADR-0009,
// ADR-0032, PLAN-0003 T4).
//
// Business code depends on Port only. The Anthropic adaptor is built once in app (through
// registry/ai) and carried in registry.Deps; tests use parsetest.Fake. Parse is an outside call:
// it must run between database transactions, never inside one.
//
// The types are plain: the answer is the AI's reading as text, not yet checked against the
// transaction rules. The caller checks each item (smartentry/domain.CheckAIItem) and maps the
// category numbers back to its own categories; real category ids never leave the API.
package parse

import (
	"context"
	"errors"
)

// Style says how freely the AI may write an item's merchant (the description shown to the user).
// Only the prompt changes with it.
type Style string

// The description styles.
const (
	// StyleAsTyped keeps the user's own words; only the amount and date words are taken out. It
	// is the default, and "" means it too.
	StyleAsTyped Style = "as_typed"
	// StyleLightTidy also fixes capital letters and obvious typos, without translating.
	StyleLightTidy Style = "light_tidy"
	// StyleCleanName lets the AI rewrite the merchant as a clean merchant name.
	StyleCleanName Style = "clean_name"
)

// Kind is a category's kind.
type Kind string

// The category kinds (ADR-0039).
const (
	KindExpense Kind = "expense"
	KindIncome  Kind = "income"
)

// Category is one category the AI may choose. Ref is a short number the caller assigns (1..n in
// its own order); the AI answers with it, and the caller maps it back to the category.
type Category struct {
	Ref  int
	Name string
	Kind Kind
}

// Request is one text to read.
type Request struct {
	// Text is what the user typed. It must not be empty.
	Text string
	// Today is the current day in the app time zone, written YYYY-MM-DD (ADR-0042). Relative
	// dates in the text are read from it.
	Today string
	// Categories are the user's active categories; refs must be positive and distinct.
	Categories []Category
	// Style is the description style ("" is StyleAsTyped).
	Style Style
}

// Confidence is how sure the AI is of an item.
type Confidence string

// The confidence levels.
const (
	ConfidenceHigh Confidence = "high"
	ConfidenceLow  Confidence = "low"
)

// Item is one item as the AI read it, before any check.
type Item struct {
	// Text is the part of the user's text the item came from.
	Text string
	// Amount is the amount in baht as text, such as "145.50"; "" when the AI found none.
	Amount string
	// OccurredOn is the day as YYYY-MM-DD; "" when the AI could not work it out.
	OccurredOn string
	// Merchant is the description, written in the requested style; it may be "".
	Merchant string
	// CategoryRef is the Ref of the chosen category; 0 for none. A number that was not offered
	// is turned into 0 with ConfidenceLow.
	CategoryRef int
	// Confidence is ConfidenceHigh or ConfidenceLow; any other answer becomes ConfidenceLow.
	Confidence Confidence
}

// Response is the items in the order written; there is at least one.
type Response struct {
	Items []Item
}

// Errors a caller can tell apart with errors.Is. They are wrapped with context; no error quotes
// the API key or the user's text.
var (
	// ErrNotConfigured: no API key is set, so no call was made.
	ErrNotConfigured = errors.New("the AI is not configured")
	// ErrUnavailable: the AI could not be reached in time: a network error, a rate limit (429),
	// a server error (5xx) or a deadline (AI_TIMEOUT or the request's own). A deadline or
	// cancellation also matches the context's error.
	ErrUnavailable = errors.New("the AI is unavailable")
	// ErrUnusable: the AI answered, but the answer cannot be used as a whole: it refused, it was
	// cut off, or its JSON does not decode or has no item.
	ErrUnusable = errors.New("the AI's answer is unusable")
)

// Port reads quick entry text with the AI.
type Port interface {
	// Parse reads req.Text into items. It refuses to run inside an open database transaction
	// (tx.ErrInside), and stops at AI_TIMEOUT or ctx's deadline, whichever comes first. An
	// invalid request (empty text, bad today, unknown style, bad refs) is an error matching none
	// of the sentinels, and no call is made.
	Parse(ctx context.Context, req Request) (Response, error)
}
