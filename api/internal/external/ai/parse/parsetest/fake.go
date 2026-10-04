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
// open transaction. The zero value answers with no items and no error; it is safe for concurrent
// use.
type Fake struct {
	mu       sync.Mutex
	requests []parse.Request
	resp     parse.Response
	err      error
}

var _ parse.Port = (*Fake)(nil)

// New returns an empty Fake.
func New() *Fake { return &Fake{} }

// Parse records req (outside a transaction only), then returns the error set by FailWith or the
// answer set by RespondWith.
func (f *Fake) Parse(ctx context.Context, req parse.Request) (parse.Response, error) {
	if err := tx.MustBeOutside(ctx); err != nil {
		return parse.Response{}, fmt.Errorf("ai parse: %w", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	req.Categories = slices.Clone(req.Categories)
	f.requests = append(f.requests, req)
	if f.err != nil {
		return parse.Response{}, f.err
	}
	return parse.Response{Items: slices.Clone(f.resp.Items)}, nil
}

// RespondWith makes every later Parse return resp.
func (f *Fake) RespondWith(resp parse.Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resp = parse.Response{Items: slices.Clone(resp.Items)}
}

// FailWith makes every later Parse return err (nil answers again).
func (f *Fake) FailWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// Requests returns a copy of the requests received so far, oldest first.
func (f *Fake) Requests() []parse.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]parse.Request, len(f.requests))
	for i, r := range f.requests {
		r.Categories = slices.Clone(r.Categories)
		out[i] = r
	}
	return out
}
