// Package parsetest has a fake AI parser for tests: it records requests and returns a set answer
// instead of calling the AI.
package parsetest

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Fake implements parse.Port in memory. Like the Anthropic adaptor it refuses to run inside an
// open transaction. The zero value is configured and answers with no items and no error; it is
// safe for concurrent use.
type Fake struct {
	mu            sync.Mutex
	requests      []parse.Request
	resp          parse.Response
	respond       func(parse.Request) (parse.Response, error)
	err           error
	notConfigured bool
	refused       int
}

var _ parse.Port = (*Fake)(nil)

// New returns an empty Fake.
func New() *Fake { return &Fake{} }

// Configured implements parse.Port: true unless SetConfigured(false) was called.
func (f *Fake) Configured() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.notConfigured
}

// Parse refuses inside an open transaction (counted by RefusedInside) and, like the Anthropic
// adaptor, answers parse.ErrNotConfigured without recording when not configured. Otherwise it
// records req, then returns the error set by FailWith, the answer of the function set by
// RespondFunc, or the answer set by RespondWith, in that order.
func (f *Fake) Parse(ctx context.Context, req parse.Request) (parse.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := tx.MustBeOutside(ctx); err != nil {
		f.refused++
		return parse.Response{}, fmt.Errorf("ai parse: %w", err)
	}
	if f.notConfigured {
		return parse.Response{}, fmt.Errorf("ai parse: %w", parse.ErrNotConfigured)
	}
	f.requests = append(f.requests, clone(req))
	switch {
	case f.err != nil:
		return parse.Response{}, f.err
	case f.respond != nil:
		resp, err := f.respond(clone(req))
		return parse.Response{Items: slices.Clone(resp.Items)}, err
	}
	return parse.Response{Items: slices.Clone(f.resp.Items)}, nil
}

// RespondWith makes every later Parse return resp (and clears RespondFunc).
func (f *Fake) RespondWith(resp parse.Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resp, f.respond = parse.Response{Items: slices.Clone(resp.Items)}, nil
}

// RespondFunc makes every later Parse answer with fn's result for the request (nil clears it).
func (f *Fake) RespondFunc(fn func(parse.Request) (parse.Response, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.respond = fn
}

// FailWith makes every later Parse return err (nil answers again).
func (f *Fake) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// SetConfigured sets what Configured answers (true by default).
func (f *Fake) SetConfigured(configured bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notConfigured = !configured
}

// Requests returns a copy of the requests received so far, oldest first.
func (f *Fake) Requests() []parse.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]parse.Request, len(f.requests))
	for i, r := range f.requests {
		out[i] = clone(r)
	}
	return out
}

// RefusedInside returns how many calls were refused because a transaction was open.
func (f *Fake) RefusedInside() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refused
}

func clone(r parse.Request) parse.Request {
	r.Lines = slices.Clone(r.Lines)
	r.Categories = slices.Clone(r.Categories)
	return r
}
