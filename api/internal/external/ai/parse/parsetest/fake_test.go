package parsetest

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

func TestFake(t *testing.T) {
	f := New()
	req := parse.Request{
		Lines:      []parse.Line{{Number: 1, Text: "coffee 60", DateIfNone: "2026-10-04"}},
		Today:      "2026-10-04",
		Categories: []parse.Category{{Ref: 1, Name: "Food", Kind: parse.KindExpense}},
	}
	resp := parse.Response{Items: []parse.Item{{Line: 1, Text: "coffee 60", Amount: "60", Confidence: parse.ConfidenceHigh}}}
	f.RespondWith(resp)
	if !f.Configured() {
		t.Error("a new fake is not configured")
	}

	got, err := f.Parse(t.Context(), req)
	if err != nil || !reflect.DeepEqual(got, resp) {
		t.Fatalf("Parse = %+v, %v; want %+v", got, err, resp)
	}

	f.RespondFunc(func(r parse.Request) (parse.Response, error) {
		return parse.Response{Items: []parse.Item{{Line: r.Lines[0].Number, Text: "from func"}}}, nil
	})
	if got, err := f.Parse(t.Context(), req); err != nil || len(got.Items) != 1 || got.Items[0].Text != "from func" {
		t.Errorf("Parse with RespondFunc = %+v, %v", got, err)
	}
	f.RespondWith(resp)

	failure := errors.New("boom")
	f.FailWith(failure)
	if _, err := f.Parse(t.Context(), req); !errors.Is(err, failure) {
		t.Errorf("Parse = %v, want the set error", err)
	}

	var insideErr error
	_ = txtest.New().WithUserTx(t.Context(), uuid.New(), func(ctx context.Context) error {
		_, insideErr = f.Parse(ctx, req)
		return nil
	})
	if !errors.Is(insideErr, tx.ErrInside) || f.RefusedInside() != 1 {
		t.Errorf("Parse inside a transaction = %v (refused %d), want tx.ErrInside", insideErr, f.RefusedInside())
	}

	f.SetConfigured(false)
	if f.Configured() {
		t.Error("Configured after SetConfigured(false)")
	}
	if _, err := f.Parse(t.Context(), req); !errors.Is(err, parse.ErrNotConfigured) {
		t.Errorf("Parse not configured = %v, want ErrNotConfigured", err)
	}

	// Three calls outside a transaction while configured are recorded; the one inside and the
	// one not configured are not. The copies are independent of the fake.
	reqs := f.Requests()
	if len(reqs) != 3 || !reflect.DeepEqual(reqs[0], req) {
		t.Fatalf("Requests = %+v, want three copies of %+v", reqs, req)
	}
	reqs[0].Categories[0].Name = "changed"
	reqs[0].Lines[0].Text = "changed"
	if r := f.Requests()[0]; r.Categories[0].Name != "Food" || r.Lines[0].Text != "coffee 60" {
		t.Error("Requests shares memory with the fake")
	}
}
