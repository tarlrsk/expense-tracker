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

// Line is one numbered line of the text to read: a part of what the user typed that the caller
// could not read on its own (PLAN-0003 T5). A line usually holds one item but may hold several.
type Line struct {
	// Number identifies the line; numbers are positive and distinct. Each answered item names the
	// line it came from.
	Number int
	// Text is the line as typed. It must not be blank.
	Text string
	// DateIfNone is the day, written YYYY-MM-DD, of an item of this line that has no date of its
	// own.
	DateIfNone string
}

// Request is the lines to read.
type Request struct {
	// Lines are the lines to read, in the order typed; there is at least one.
	Lines []Line
	// Today is the current day in the app time zone, written YYYY-MM-DD (ADR-0042). Relative
	// dates in the lines are read from it.
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
	// Line is the Number of the line the item came from; always one that was sent.
	Line int
	// Text is the part of the line the item came from.
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

// Response is the items in the order written, line by line; there is at least one. A line may
// have several items, or none.
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
	// cut off, or its JSON does not decode, has no item or names a line that was not sent.
	ErrUnusable = errors.New("the AI's answer is unusable")
)

// Port reads quick entry text with the AI.
type Port interface {
	// Configured reports whether the AI can be called at all (a key is set). It makes no call;
	// when it is false, Parse returns ErrNotConfigured.
	Configured() bool
	// Parse reads req.Lines into items. It refuses to run inside an open database transaction
	// (tx.ErrInside), and stops at AI_TIMEOUT or ctx's deadline, whichever comes first. An
	// invalid request (no line, a blank line, bad line numbers or dates, bad today, unknown
	// style, bad refs) is an error matching none of the sentinels, and no call is made.
	Parse(ctx context.Context, req Request) (Response, error)
}
