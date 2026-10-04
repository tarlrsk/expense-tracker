package parse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	categorizationdomain "github.com/tarlrsk/expense-tracker/api/internal/categorization/domain"
	aiparse "github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse/parsetest"
	"github.com/tarlrsk/expense-tracker/api/internal/smartentry/domain"
	smartentrymatchrulesproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/matchrules"
	smartentryparseproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/parse"
	smartentryreserveproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/reserve"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type (
	parseFn   func(ctx context.Context, req smartentryparseproc.Request) (smartentryparseproc.Response, error)
	reserveFn func(ctx context.Context, req smartentryreserveproc.Request) (smartentryreserveproc.Response, error)
	matchFn   func(ctx context.Context, req smartentrymatchrulesproc.Request) (smartentrymatchrulesproc.Response, error)
)

func (f parseFn) Execute(ctx context.Context, r smartentryparseproc.Request) (smartentryparseproc.Response, error) {
	return f(ctx, r)
}

func (f reserveFn) Execute(ctx context.Context, r smartentryreserveproc.Request) (smartentryreserveproc.Response, error) {
	return f(ctx, r)
}

func (f matchFn) Execute(ctx context.Context, r smartentrymatchrulesproc.Request) (smartentrymatchrulesproc.Response, error) {
	return f(ctx, r)
}

func date(s string) transactionsdomain.Date {
	d, ok := transactionsdomain.ParseDate(s)
	if !ok {
		panic(s)
	}
	return d
}

func amount(s string) transactionsdomain.Amount {
	a, err := transactionsdomain.ParseAmount(s)
	if err != nil {
		panic(err)
	}
	return a
}

// env is the orchestrator with fake processors and the fake AI. The local parser answers items;
// the reservation answers reserved; rules maps merchant keys to categories.
type env struct {
	t        *testing.T
	user     uuid.UUID
	items    []smartentryparseproc.Item
	reserved bool
	cats     []categoriesdomain.Category
	rules    map[string]uuid.UUID
	ai       *parsetest.Fake
	logs     *bytes.Buffer
	calls    []string
	matched  [][]string // the merchants of each match call
}

func newEnv(t *testing.T) *env {
	return &env{t: t, user: uuid.New(), reserved: true, rules: map[string]uuid.UUID{}, ai: parsetest.New(), logs: &bytes.Buffer{}}
}

func (e *env) orchestrator() Orchestrator {
	inTx := func(ctx context.Context, name string) {
		if err := tx.MustBeOutside(ctx); err != nil {
			e.t.Errorf("%s called inside a transaction", name)
		}
	}
	return New(
		parseFn(func(ctx context.Context, req smartentryparseproc.Request) (smartentryparseproc.Response, error) {
			inTx(ctx, "parse")
			e.calls = append(e.calls, "parse")
			if req.UserID != e.user {
				e.t.Errorf("parse for %s", req.UserID)
			}
			return smartentryparseproc.Response{Items: e.items}, nil
		}),
		reserveFn(func(ctx context.Context, req smartentryreserveproc.Request) (smartentryreserveproc.Response, error) {
			inTx(ctx, "reserve")
			e.calls = append(e.calls, "reserve")
			if req.UserID != e.user {
				e.t.Errorf("reserve for %s", req.UserID)
			}
			return smartentryreserveproc.Response{Reserved: e.reserved, Day: date("2026-10-04"), Categories: e.cats}, nil
		}),
		matchFn(func(ctx context.Context, req smartentrymatchrulesproc.Request) (smartentrymatchrulesproc.Response, error) {
			inTx(ctx, "match")
			e.calls = append(e.calls, "match")
			e.matched = append(e.matched, req.Merchants)
			resp := smartentrymatchrulesproc.Response{Matches: make([]smartentrymatchrulesproc.Match, len(req.Merchants))}
			for i, m := range req.Merchants {
				if c, ok := e.rules[categorizationdomain.MerchantKey(m)]; ok && m != "" {
					resp.Matches[i] = smartentrymatchrulesproc.Match{Found: true, Rule: categorizationdomain.Rule{CategoryID: c}}
				}
			}
			return resp, nil
		}),
		e.ai, slog.New(slog.NewJSONHandler(e.logs, nil)),
	)
}

// show writes a proposal in short: "text | amount | date | merchant | category | confidence | by".
func show(it Item, names map[uuid.UUID]string) string {
	a := ""
	if it.HasAmount {
		a = it.Amount.String()
	}
	c := "-"
	if it.CategoryID != uuid.Nil {
		c = names[it.CategoryID]
	}
	return fmt.Sprintf("%s | %s | %s | %s | %s | %s | %s", it.Text, a, it.OccurredOn, it.Merchant, c, it.Confidence, it.ResolvedBy)
}

func TestExecute(t *testing.T) {
	food, transport, salary := uuid.New(), uuid.New(), uuid.New()
	names := map[uuid.UUID]string{food: "food", transport: "transport", salary: "salary"}
	cats := []categoriesdomain.Category{
		{ID: food, Name: "Food", Kind: categoriesdomain.KindExpense},
		{ID: transport, Name: "Transport", Kind: categoriesdomain.KindExpense},
		{ID: salary, Name: "Salary", Kind: categoriesdomain.KindIncome},
	}
	grab := smartentryparseproc.Item{
		Text: "grab 145", Amount: amount("145"), HasAmount: true, OccurredOn: date("2026-10-04"), Merchant: "Grab",
		CategoryID: transport, Resolved: true,
	}
	coffee := smartentryparseproc.Item{Text: "coffee 60", Amount: amount("60"), HasAmount: true, OccurredOn: date("2026-10-03"), Merchant: "coffee"}
	undecided := smartentryparseproc.Item{Text: "rice 1k", OccurredOn: transactionsdomain.Date{}, Merchant: "rice 1k"}
	taxi := smartentryparseproc.Item{Text: "taxi 90 lunch 50", OccurredOn: date("2026-10-04"), Merchant: "taxi 90 lunch 50"}

	t.Run("all resolved: no reservation and no AI", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{grab}
		resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: " grab 145 "})
		if err != nil || resp.AI != domain.AINotNeeded || len(resp.Items) != 1 {
			t.Fatalf("Execute = %+v, %v", resp, err)
		}
		if got := show(resp.Items[0], names); got != "grab 145 | 145.00 | 2026-10-04 | Grab | transport | high | rule" {
			t.Errorf("item %s", got)
		}
		if !slices.Equal(e.calls, []string{"parse"}) || len(e.ai.Requests()) != 0 {
			t.Errorf("calls %v, AI requests %d", e.calls, len(e.ai.Requests()))
		}
	})

	t.Run("not configured: no reservation, no call", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{grab, coffee}
		e.ai.SetConfigured(false)
		resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "grab 145, coffee 60 yesterday"})
		if err != nil || resp.AI != domain.AINotConfigured {
			t.Fatalf("Execute = %+v, %v", resp, err)
		}
		want := []string{
			"grab 145 | 145.00 | 2026-10-04 | Grab | transport | high | rule",
			"coffee 60 | 60.00 | 2026-10-03 | coffee | - | low | none",
		}
		if got := showAll(resp.Items, names); !slices.Equal(got, want) {
			t.Errorf("items\n got %q\nwant %q", got, want)
		}
		if !slices.Equal(e.calls, []string{"parse"}) {
			t.Errorf("calls %v", e.calls)
		}
	})

	t.Run("limit reached: the parser's items, no call", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{coffee, grab}
		e.reserved = false
		resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "coffee 60 yesterday, grab 145"})
		if err != nil || resp.AI != domain.AILimitReached {
			t.Fatalf("Execute = %+v, %v", resp, err)
		}
		want := []string{
			"coffee 60 | 60.00 | 2026-10-03 | coffee | - | low | none",
			"grab 145 | 145.00 | 2026-10-04 | Grab | transport | high | rule",
		}
		if got := showAll(resp.Items, names); !slices.Equal(got, want) {
			t.Errorf("items\n got %q\nwant %q", got, want)
		}
		if !slices.Equal(e.calls, []string{"parse", "reserve"}) || len(e.ai.Requests()) != 0 {
			t.Errorf("calls %v, AI requests %d", e.calls, len(e.ai.Requests()))
		}
	})

	t.Run("used: only unresolved lines are sent, and the answer is placed by line", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{coffee, grab, undecided, taxi}
		e.cats = cats
		e.rules["taxi"] = transport // the rule beats the AI's category
		e.ai.RespondWith(aiparse.Response{Items: []aiparse.Item{
			{Line: 1, Text: "coffee 60", Amount: "60", OccurredOn: "2026-10-03", Merchant: "coffee", CategoryRef: 1, Confidence: aiparse.ConfidenceHigh},
			{Line: 3, Text: "taxi 90", Amount: "90", OccurredOn: "2026-10-04", Merchant: "taxi", CategoryRef: 1, Confidence: aiparse.ConfidenceHigh},
			{Line: 3, Text: "invented", Amount: "50", OccurredOn: "2026-10-04", Merchant: "lunch", CategoryRef: 0, Confidence: aiparse.ConfidenceHigh},
			// line 2 gets no item: it keeps the parser's reading
		}})
		resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "..."})
		if err != nil || resp.AI != domain.AIUsed {
			t.Fatalf("Execute = %+v, %v", resp, err)
		}
		want := []string{
			"coffee 60 | 60.00 | 2026-10-03 | coffee | food | high | ai",
			"grab 145 | 145.00 | 2026-10-04 | Grab | transport | high | rule",
			"rice 1k |  |  | rice 1k | - | low | none",
			"taxi 90 | 90.00 | 2026-10-04 | taxi | transport | high | rule",
			"taxi 90 lunch 50 | 50.00 | 2026-10-04 | lunch | - | low | ai",
		}
		if got := showAll(resp.Items, names); !slices.Equal(got, want) {
			t.Errorf("items\n got %q\nwant %q", got, want)
		}
		reqs := e.ai.Requests()
		if len(reqs) != 1 {
			t.Fatalf("AI requests %d", len(reqs))
		}
		wantLines := []aiparse.Line{
			{Number: 1, Text: "coffee 60", DateIfNone: "2026-10-03"},
			{Number: 2, Text: "rice 1k", DateIfNone: "2026-10-04"}, // undecided: today
			{Number: 3, Text: "taxi 90 lunch 50", DateIfNone: "2026-10-04"},
		}
		wantCats := []aiparse.Category{
			{Ref: 1, Name: "Food", Kind: aiparse.KindExpense},
			{Ref: 2, Name: "Transport", Kind: aiparse.KindExpense},
			{Ref: 3, Name: "Salary", Kind: aiparse.KindIncome},
		}
		if r := reqs[0]; !slices.Equal(r.Lines, wantLines) || !slices.Equal(r.Categories, wantCats) || r.Today != "2026-10-04" ||
			r.Style != aiparse.StyleAsTyped {
			t.Errorf("AI request %+v", r)
		}
		if !slices.Equal(e.calls, []string{"parse", "reserve", "match"}) || !slices.Equal(e.matched[0], []string{"coffee", "taxi", "lunch"}) {
			t.Errorf("calls %v, matched %v", e.calls, e.matched)
		}
	})

	t.Run("the style is passed on", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{coffee}
		if _, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "x", Style: aiparse.StyleCleanName}); err != nil {
			t.Fatal(err)
		}
		if r := e.ai.Requests(); len(r) != 1 || r[0].Style != aiparse.StyleCleanName {
			t.Errorf("AI requests %+v", r)
		}
	})

	for _, tt := range []struct {
		name string
		err  error
		want domain.AIStatus
	}{
		{"unavailable", fmt.Errorf("ai parse: %w: Anthropic answered 529", aiparse.ErrUnavailable), domain.AIUnavailable},
		{"unusable", fmt.Errorf("ai parse: %w: the model refused", aiparse.ErrUnusable), domain.AIUnavailable},
		{"another error", errors.New("ai parse: Anthropic answered 400"), domain.AIUnavailable},
		{"AI_TIMEOUT", fmt.Errorf("ai parse: %w: %w", aiparse.ErrUnavailable, context.DeadlineExceeded), domain.AIUnavailable},
		{"not configured after all", fmt.Errorf("ai parse: %w", aiparse.ErrNotConfigured), domain.AINotConfigured},
	} {
		t.Run("the AI fails: "+tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.items = []smartentryparseproc.Item{coffee, grab}
			e.ai.FailWith(tt.err)
			resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "coffee 60 yesterday, grab 145"})
			if err != nil || resp.AI != tt.want {
				t.Fatalf("Execute = %+v, %v; want AI %s", resp, err, tt.want)
			}
			want := []string{
				"coffee 60 | 60.00 | 2026-10-03 | coffee | - | low | none",
				"grab 145 | 145.00 | 2026-10-04 | Grab | transport | high | rule",
			}
			if got := showAll(resp.Items, names); !slices.Equal(got, want) {
				t.Errorf("items\n got %q\nwant %q", got, want)
			}
			logs := e.logs.String()
			if !strings.Contains(logs, "AI parse failed") || !strings.Contains(logs, e.user.String()) {
				t.Errorf("failure not logged: %s", logs)
			}
			if strings.Contains(logs, "coffee") || strings.Contains(logs, "grab") {
				t.Errorf("the log quotes the text: %s", logs)
			}
		})
	}

	t.Run("an answer naming a line that was not sent is unusable", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{coffee}
		e.ai.RespondWith(aiparse.Response{Items: []aiparse.Item{{Line: 2, Text: "coffee 60"}}})
		resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "coffee 60"})
		if err != nil || resp.AI != domain.AIUnavailable || len(resp.Items) != 1 || resp.Items[0].ResolvedBy != domain.ResolvedByNone {
			t.Fatalf("Execute = %+v, %v", resp, err)
		}
	})

	t.Run("the request's own deadline: timeout", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{coffee}
		ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Hour))
		e.ai.RespondFunc(func(aiparse.Request) (aiparse.Response, error) {
			cancel() // stands for the deadline passing during the call
			return aiparse.Response{}, fmt.Errorf("ai parse: %w: %w", aiparse.ErrUnavailable, context.Canceled)
		})
		_, err := e.orchestrator().Execute(ctx, Request{UserID: e.user, Text: "coffee 60"})
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want the context's error", err)
		}

		e = newEnv(t)
		e.items = []smartentryparseproc.Item{coffee}
		ctx, cancel = context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		e.ai.RespondFunc(func(aiparse.Request) (aiparse.Response, error) {
			<-ctx.Done()
			return aiparse.Response{}, fmt.Errorf("ai parse: %w: %w", aiparse.ErrUnavailable, ctx.Err())
		})
		_, err = e.orchestrator().Execute(ctx, Request{UserID: e.user, Text: "coffee 60"})
		if apperr.KindOf(err) != apperr.Timeout {
			t.Errorf("error = %v (kind %s), want timeout", err, apperr.KindOf(err))
		}
	})

	t.Run("a call inside a transaction is an internal error", func(t *testing.T) {
		e := newEnv(t)
		e.items = []smartentryparseproc.Item{coffee}
		e.ai.FailWith(fmt.Errorf("ai parse: %w", tx.ErrInside))
		_, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "coffee 60"})
		if apperr.KindOf(err) != apperr.Internal || !errors.Is(err, tx.ErrInside) {
			t.Errorf("error = %v, want internal", err)
		}
	})
}

func TestExecuteRefusesInput(t *testing.T) {
	many := make([]smartentryparseproc.Item, domain.MaxItems+1)
	for i := range many {
		many[i] = smartentryparseproc.Item{Text: "x"}
	}
	tests := []struct {
		name      string
		text      string
		items     []smartentryparseproc.Item
		wantMsg   string
		wantParse bool
	}{
		{name: "empty", text: "", wantMsg: domain.TextRuleMessage},
		{name: "blank", text: " \n ", wantMsg: domain.TextRuleMessage},
		{name: "too long", text: strings.Repeat("ก", 1001), wantMsg: domain.TextRuleMessage},
		{name: "no item", text: "yesterday", wantMsg: domain.NoItemMessage, wantParse: true},
		{name: "21 items", text: "x", items: many, wantMsg: domain.TooManyItemsMessage, wantParse: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.items = tt.items
			_, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: tt.text})
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Kind != apperr.InvalidInput || ae.Message != tt.wantMsg {
				t.Fatalf("error = %v, want invalid_input %q", err, tt.wantMsg)
			}
			if tt.wantParse != slices.Contains(e.calls, "parse") || slices.Contains(e.calls, "reserve") || len(e.ai.Requests()) != 0 {
				t.Errorf("calls %v, AI requests %d", e.calls, len(e.ai.Requests()))
			}
		})
	}

	t.Run("20 items are accepted", func(t *testing.T) {
		e := newEnv(t)
		e.items = many[:domain.MaxItems]
		e.ai.SetConfigured(false)
		if resp, err := e.orchestrator().Execute(t.Context(), Request{UserID: e.user, Text: "x"}); err != nil || len(resp.Items) != domain.MaxItems {
			t.Errorf("Execute = %d items, %v", len(resp.Items), err)
		}
	})
}

func showAll(items []Item, names map[uuid.UUID]string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = show(it, names)
	}
	return out
}
