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
	req := parse.Request{Text: "coffee 60", Today: "2026-10-04", Categories: []parse.Category{{Ref: 1, Name: "Food", Kind: parse.KindExpense}}}
	resp := parse.Response{Items: []parse.Item{{Text: "coffee 60", Amount: "60", Confidence: parse.ConfidenceHigh}}}
	f.RespondWith(resp)

	got, err := f.Parse(t.Context(), req)
	if err != nil || !reflect.DeepEqual(got, resp) {
		t.Fatalf("Parse = %+v, %v; want %+v", got, err, resp)
	}

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
	if !errors.Is(insideErr, tx.ErrInside) {
		t.Errorf("Parse inside a transaction = %v, want tx.ErrInside", insideErr)
	}

	// Two calls outside a transaction are recorded; the one inside is not. The copies are
	// independent of the fake.
	reqs := f.Requests()
	if len(reqs) != 2 || !reflect.DeepEqual(reqs[0], req) {
		t.Fatalf("Requests = %+v, want two copies of %+v", reqs, req)
	}
	reqs[0].Categories[0].Name = "changed"
	if f.Requests()[0].Categories[0].Name != "Food" {
		t.Error("Requests shares memory with the fake")
	}
}
