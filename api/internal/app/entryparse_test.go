package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	accountdomain "github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	aiparse "github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/smartentry/domain"
)

// API-level tests of POST /api/entry/parse (PLAN-0003 T5, docs/04-api.md): the local parser
// first, the AI (the fake, never the real one) only for what is unresolved, the daily limit, and
// the caller's own rules and categories only.

// entryClock is the clock of these tests: 17:30 UTC on 2 October 2026 is 3 October in Bangkok.
var entryClock = time.Date(2026, time.October, 2, 17, 30, 0, 0, time.UTC)

const (
	entryToday     = "2026-10-03"
	entryYesterday = "2026-10-02"
)

// withParseLimit sets AI_DAILY_PARSE_LIMIT.
func withParseLimit(n int) func(*registry.Deps) {
	return func(d *registry.Deps) { d.Config.AI.DailyParseLimit = n }
}

// proposalKeys are exactly the keys of an item of the answer.
var proposalKeys = []string{"text", "amount", "occurred_on", "merchant", "category_id", "confidence", "resolved_by"}

type proposal struct {
	Text       string     `json:"text"`
	Amount     string     `json:"amount"`
	OccurredOn string     `json:"occurred_on"`
	Merchant   string     `json:"merchant"`
	CategoryID *uuid.UUID `json:"category_id"`
	Confidence string     `json:"confidence"`
	ResolvedBy string     `json:"resolved_by"`
}

type entryAnswer struct {
	Items []proposal `json:"items"`
	AI    string     `json:"ai"`
}

func (e *apiEnv) postParse(tok, body string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.do(http.MethodPost, "/api/entry/parse", tok, body)
}

// mustParseText posts text and returns the answer, failing unless 200 with exactly the
// documented keys.
func (e *apiEnv) mustParseText(tok, text string) entryAnswer {
	e.t.Helper()
	rec := e.postParse(tok, jsonBody(e.t, map[string]string{"text": text}))
	if rec.Code != http.StatusOK {
		e.t.Fatalf("parse %q: %d %s", text, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		e.t.Errorf("Cache-Control = %q", got)
	}
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		e.t.Fatal(err)
	}
	assertKeys(e.t, "answer", decodeObject(e.t, rec), "items", "ai")
	for _, it := range raw.Items {
		assertKeys(e.t, "item", it, proposalKeys...)
	}
	var a entryAnswer
	decodeStrict(e.t, rec, &a)
	return a
}

// show writes a proposal in short: "text | amount | date | merchant | category | confidence | by",
// with the category's name ("-" for null).
func (p proposal) show(names map[uuid.UUID]string) string {
	cat := "-"
	if p.CategoryID != nil {
		cat = names[*p.CategoryID]
		if cat == "" {
			cat = "unknown " + p.CategoryID.String()
		}
	}
	return strings.Join([]string{p.Text, p.Amount, p.OccurredOn, p.Merchant, cat, p.Confidence, p.ResolvedBy}, " | ")
}

// assertAnswer fails unless the answer has the AI status and the items, in order.
func assertAnswer(t *testing.T, got entryAnswer, names map[uuid.UUID]string, ai string, items ...string) {
	t.Helper()
	shown := make([]string, len(got.Items))
	for i, p := range got.Items {
		shown[i] = p.show(names)
	}
	if got.AI != ai || !slices.Equal(shown, items) {
		t.Errorf("answer: ai %q, items\n%s\nwant ai %q, items\n%s", got.AI, strings.Join(shown, "\n"), ai, strings.Join(items, "\n"))
	}
}

// categoryNames maps the user's category ids to their names.
func (e *apiEnv) categoryNames(tok string) map[uuid.UUID]string {
	e.t.Helper()
	out := map[uuid.UUID]string{}
	for _, c := range e.listCategories(tok) {
		out[c.ID] = c.Name
	}
	return out
}

// usageOf is the user's stored parse counts as "day=count", by day.
func (e *apiEnv) usageOf(userID uuid.UUID) []string {
	e.t.Helper()
	rows, err := e.super.QueryContext(e.t.Context(),
		"select day::text, parse_count from ai_usage where owner_id = $1 order by day", userID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var (
			day string
			n   int
		)
		if err := rows.Scan(&day, &n); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s=%d", day, n))
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func (e *apiEnv) assertUsage(userID uuid.UUID, want ...string) {
	e.t.Helper()
	if got := e.usageOf(userID); !slices.Equal(got, want) {
		e.t.Errorf("ai_usage = %v, want %v", got, want)
	}
}

// aiRead is how the fake AI reads one line: the items it returns for it, with the category by
// name ("" for none).
type aiRead struct {
	text, amount, date, merchant, category string
	high                                   bool
}

// aiReads makes the fake AI answer each line whose text is a key of reads with those items, in
// order, numbered with the line; any other line gets no item.
func aiReads(reads map[string][]aiRead) func(aiparse.Request) (aiparse.Response, error) {
	return func(r aiparse.Request) (aiparse.Response, error) {
		var resp aiparse.Response
		for _, l := range r.Lines {
			for _, rd := range reads[l.Text] {
				item := aiparse.Item{
					Line: l.Number, Text: rd.text, Amount: rd.amount, OccurredOn: rd.date, Merchant: rd.merchant,
					Confidence: aiparse.ConfidenceLow,
				}
				if rd.high {
					item.Confidence = aiparse.ConfidenceHigh
				}
				for _, c := range r.Categories {
					if c.Name == rd.category {
						item.CategoryRef = c.Ref
					}
				}
				resp.Items = append(resp.Items, item)
			}
		}
		return resp, nil
	}
}

// newAIRequests returns the fake AI's requests after the first n.
func (e *apiEnv) newAIRequests(n int) []aiparse.Request {
	return e.ai.Requests()[n:]
}

func TestEntryParse(t *testing.T) {
	e := newAPIEnv(t, withClock(entryClock))
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	names := e.categoryNames(tok)
	food, transport := e.categoryID(tok, "Food"), e.categoryID(tok, "Transport")
	e.addRule(a.id, "grab", "Grab", transport)
	txs := func() int { return e.count("select count(*) from transactions where owner_id = $1", a.id) }

	t.Run("grab 145 with a rule: resolved locally, no AI request at all, nothing counted", func(t *testing.T) {
		e := e.with(t)
		got := e.mustParseText(tok, "grab 145")
		assertAnswer(t, got, names, "not_needed", "grab 145 | 145.00 | "+entryToday+" | Grab | Transport | high | rule")
		if n := len(e.ai.Requests()); n != 0 {
			t.Errorf("the AI got %d requests", n)
		}
		e.assertUsage(a.id)
	})

	t.Run("ข้าวมันไก่ 60: the AI reads it with one of the caller's categories; one use counted", func(t *testing.T) {
		e := e.with(t)
		n := len(e.ai.Requests())
		e.ai.RespondFunc(aiReads(map[string][]aiRead{
			"ข้าวมันไก่ 60": {{"ข้าวมันไก่ 60", "60", entryToday, "ข้าวมันไก่", "Food", true}},
		}))
		got := e.mustParseText(tok, "ข้าวมันไก่ 60")
		assertAnswer(t, got, names, "used", "ข้าวมันไก่ 60 | 60.00 | "+entryToday+" | ข้าวมันไก่ | Food | high | ai")
		reqs := e.newAIRequests(n)
		if len(reqs) != 1 {
			t.Fatalf("the AI got %d requests", len(reqs))
		}
		if r := reqs[0]; !slices.Equal(r.Lines, []aiparse.Line{{Number: 1, Text: "ข้าวมันไก่ 60", DateIfNone: entryToday}}) ||
			r.Today != entryToday || r.Style != aiparse.StyleAsTyped {
			t.Errorf("AI request %+v", r)
		}
		e.assertUsage(a.id, entryToday+"=1")
	})

	t.Run("coffee 60, grab 145 yesterday: the rule for grab, the AI for coffee with yesterday", func(t *testing.T) {
		e := e.with(t)
		n := len(e.ai.Requests())
		e.ai.RespondFunc(aiReads(map[string][]aiRead{
			"coffee 60": {{"coffee 60", "60", entryYesterday, "coffee", "Food", true}},
		}))
		got := e.mustParseText(tok, "coffee 60, grab 145 yesterday")
		assertAnswer(t, got, names, "used",
			"coffee 60 | 60.00 | "+entryYesterday+" | coffee | Food | high | ai",
			"grab 145 yesterday | 145.00 | "+entryYesterday+" | Grab | Transport | high | rule")
		reqs := e.newAIRequests(n)
		if len(reqs) != 1 || !slices.Equal(reqs[0].Lines, []aiparse.Line{{Number: 1, Text: "coffee 60", DateIfNone: entryYesterday}}) {
			t.Errorf("AI requests %+v", reqs)
		}
	})

	t.Run("only unresolved lines are sent, each with its day; the answer keeps the typed order", func(t *testing.T) {
		e := e.with(t)
		n := len(e.ai.Requests())
		e.ai.RespondFunc(aiReads(map[string][]aiRead{
			"rice 1k เมื่อวานซืน": {{"rice 1k เมื่อวานซืน", "1000", "2026-10-01", "rice", "Food", true}},
			"latte 80": {{"latte 80", "80", entryYesterday, "latte", "Food", false}},
			// "ข้าว 50 yesterday" gets no item: it keeps the parser's reading.
		}))
		got := e.mustParseText(tok, "rice 1k เมื่อวานซืน, grab 145, latte 80, ข้าว 50 yesterday")
		assertAnswer(t, got, names, "used",
			"rice 1k เมื่อวานซืน | 1000.00 | 2026-10-01 | rice | Food | high | ai",
			"grab 145 | 145.00 | "+entryYesterday+" | Grab | Transport | high | rule",
			"latte 80 | 80.00 | "+entryYesterday+" | latte | Food | low | ai",
			"ข้าว 50 yesterday | 50.00 | "+entryYesterday+" | ข้าว | - | low | none")
		want := []aiparse.Line{
			{Number: 1, Text: "rice 1k เมื่อวานซืน", DateIfNone: "2026-10-01"},
			{Number: 2, Text: "latte 80", DateIfNone: entryYesterday},
			{Number: 3, Text: "ข้าว 50 yesterday", DateIfNone: entryYesterday},
		}
		if reqs := e.newAIRequests(n); len(reqs) != 1 || !slices.Equal(reqs[0].Lines, want) {
			t.Errorf("AI requests %+v, want lines %+v", reqs, want)
		}
	})

	t.Run("an item whose day the parser cannot decide is sent with today", func(t *testing.T) {
		e := e.with(t)
		n := len(e.ai.Requests())
		e.ai.RespondWith(aiparse.Response{})
		got := e.mustParseText(tok, "yesterday coffee 60, rice 50, tea 40 today")
		if got.AI != "used" || len(got.Items) != 3 || got.Items[1].OccurredOn != "" || got.Items[1].ResolvedBy != "none" {
			t.Errorf("answer %+v", got)
		}
		want := []aiparse.Line{
			{Number: 1, Text: "yesterday coffee 60", DateIfNone: entryYesterday},
			{Number: 2, Text: "rice 50", DateIfNone: entryToday},
			{Number: 3, Text: "tea 40 today", DateIfNone: entryToday},
		}
		if reqs := e.newAIRequests(n); len(reqs) != 1 || !slices.Equal(reqs[0].Lines, want) {
			t.Errorf("AI requests %+v, want lines %+v", reqs, want)
		}
	})

	t.Run("one line read as two items: both take the line's place, in the AI's order", func(t *testing.T) {
		e := e.with(t)
		e.ai.RespondFunc(aiReads(map[string][]aiRead{
			"taxi 90 lunch 50": {
				{"taxi 90", "90", entryToday, "taxi", "Transport", true},
				{"lunch 50", "50", entryToday, "lunch", "Food", true},
			},
		}))
		got := e.mustParseText(tok, "grab 60, taxi 90 lunch 50, grab 20")
		assertAnswer(t, got, names, "used",
			"grab 60 | 60.00 | "+entryToday+" | Grab | Transport | high | rule",
			"taxi 90 | 90.00 | "+entryToday+" | taxi | Transport | high | ai",
			"lunch 50 | 50.00 | "+entryToday+" | lunch | Food | high | ai",
			"grab 20 | 20.00 | "+entryToday+" | Grab | Transport | high | rule")
	})

	t.Run("a rule beats the AI's category, and fills a category the AI left out", func(t *testing.T) {
		e := e.with(t)
		e.addRule(a.id, "latte", "Latte", food)
		e.addRule(a.id, "7eleven", "7-Eleven", food)
		e.ai.RespondFunc(aiReads(map[string][]aiRead{
			"latte 1k":    {{"latte 1k", "1000", entryToday, "latte", "Transport", true}},
			"7 eleven 60": {{"7 eleven 60", "60", entryToday, "7 Eleven", "", true}},
		}))
		got := e.mustParseText(tok, "latte 1k, 7 eleven 60")
		assertAnswer(t, got, names, "used",
			"latte 1k | 1000.00 | "+entryToday+" | latte | Food | high | rule",
			"7 eleven 60 | 60.00 | "+entryToday+" | 7 Eleven | Food | high | rule")
	})

	t.Run("the AI sees only the caller's active categories, numbered, and no id", func(t *testing.T) {
		e := e.with(t)
		b := e.newAccount(accountOpts{})
		e.mustCreateCategory(e.freshSession(b.id), "B Secret Qxv")
		shopping := e.categoryID(tok, "Shopping")
		e.mustPatchCategory(tok, shopping, `{"archived":true}`)
		t.Cleanup(func() { e.mustPatchCategory(tok, shopping, `{"archived":false}`) })
		n := len(e.ai.Requests())
		e.ai.RespondWith(aiparse.Response{})
		e.mustParseText(tok, "cafe 75")
		reqs := e.newAIRequests(n)
		if len(reqs) != 1 {
			t.Fatalf("the AI got %d requests", len(reqs))
		}
		var want []aiparse.Category
		for _, c := range e.listCategories(tok) {
			if !c.Archived {
				want = append(want, aiparse.Category{Ref: len(want) + 1, Name: c.Name, Kind: aiparse.Kind(c.Kind)})
			}
		}
		if !slices.Equal(reqs[0].Categories, want) {
			t.Errorf("categories %+v\nwant %+v", reqs[0].Categories, want)
		}
		sent, _ := json.Marshal(reqs[0])
		if regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-`).Match(sent) || strings.Contains(string(sent), "Shopping") ||
			strings.Contains(string(sent), "B Secret Qxv") {
			t.Errorf("the AI request holds an id, an archived or another user's category: %s", sent)
		}
	})

	t.Run("a category the AI names that is not offered is dropped, and the item is low", func(t *testing.T) {
		e := e.with(t)
		e.ai.RespondWith(aiparse.Response{Items: []aiparse.Item{
			{Line: 1, Text: "cafe 75", Amount: "75", OccurredOn: entryToday, Merchant: "cafe", CategoryRef: 99, Confidence: aiparse.ConfidenceHigh},
		}})
		got := e.mustParseText(tok, "cafe 75")
		assertAnswer(t, got, names, "used", "cafe 75 | 75.00 | "+entryToday+" | cafe | - | low | ai")
	})

	t.Run("nothing is saved: no transaction, no rule; only the count moves", func(t *testing.T) {
		e := e.with(t)
		before, rules, usage := txs(), e.merchantRules(a.id), e.usageOf(a.id)
		e.ai.RespondFunc(aiReads(map[string][]aiRead{"noodles 50": {{"noodles 50", "50", entryToday, "noodles", "Food", true}}}))
		e.mustParseText(tok, "grab 145, noodles 50")
		if txs() != before {
			t.Error("a parse saved a transaction")
		}
		e.assertRulesUnchanged(a.id, rules, "a parse")
		if after := e.usageOf(a.id); len(after) != 1 || after[0] == usage[0] {
			t.Errorf("usage %v -> %v, want one more", usage, after)
		}
	})

	t.Run("the AI is never called inside a transaction", func(t *testing.T) {
		if n := e.ai.RefusedInside(); n != 0 {
			t.Errorf("%d AI calls were made inside a transaction", n)
		}
	})
}

// The AI's failures: the parser's items still come back with a status; a call that was made
// counts, one that was not does not; the log names the failure without the text.
func TestEntryParseWithoutAI(t *testing.T) {
	e := newAPIEnv(t, withClock(entryClock))
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	names := e.categoryNames(tok)
	e.addRule(a.id, "grab", "Grab", e.categoryID(tok, "Transport"))
	local := []string{
		"coffee 60 | 60.00 | " + entryToday + " | coffee | - | low | none",
		"grab 145 | 145.00 | " + entryToday + " | Grab | Transport | high | rule",
	}

	t.Run("not configured: no call, nothing counted", func(t *testing.T) {
		e := e.with(t)
		e.ai.SetConfigured(false)
		defer e.ai.SetConfigured(true)
		assertAnswer(t, e.mustParseText(tok, "coffee 60, grab 145"), names, "not_configured", local...)
		if n := len(e.ai.Requests()); n != 0 {
			t.Errorf("the AI got %d requests", n)
		}
		e.assertUsage(a.id)
	})

	for i, tt := range []struct {
		name string
		err  error
	}{
		{"unavailable (down or AI_TIMEOUT)", fmt.Errorf("ai parse: %w: %w", aiparse.ErrUnavailable, errors.New("timeout"))},
		{"unusable answer", fmt.Errorf("ai parse: %w: the model refused", aiparse.ErrUnusable)},
		{"another error", errors.New("ai parse: Anthropic answered 400")},
	} {
		t.Run(tt.name+": the call counts", func(t *testing.T) {
			e := e.with(t)
			e.ai.FailWith(tt.err)
			defer e.ai.FailWith(nil)
			assertAnswer(t, e.mustParseText(tok, "coffee 60, grab 145"), names, "unavailable", local...)
			e.assertUsage(a.id, fmt.Sprintf("%s=%d", entryToday, i+1))
		})
	}

	t.Run("the failure is logged without the text", func(t *testing.T) {
		logs := e.logs.String()
		if !strings.Contains(logs, "AI parse failed") {
			t.Error("no AI failure in the log")
		}
		if strings.Contains(logs, "coffee") || strings.Contains(logs, "60.00") {
			t.Error("the log quotes the text")
		}
	})

	t.Run("an answer naming a line that was not sent is unusable", func(t *testing.T) {
		e := e.with(t)
		e.ai.RespondWith(aiparse.Response{Items: []aiparse.Item{{Line: 5, Text: "coffee 60", Amount: "60"}}})
		assertAnswer(t, e.mustParseText(tok, "coffee 60, grab 145"), names, "unavailable", local...)
	})
}

// The daily limit: per user, counted only when the AI is called, reset at midnight in the app
// time zone; at the limit the parser's items come back with limit_reached.
func TestEntryParseLimit(t *testing.T) {
	var (
		mu  sync.Mutex
		now = entryClock
	)
	setNow := func(t time.Time) { mu.Lock(); now = t; mu.Unlock() }
	clock := func(d *registry.Deps) {
		d.Clock = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	}
	e := newAPIEnv(t, clock, withParseLimit(2))
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	names := e.categoryNames(aTok)
	e.addRule(a.id, "grab", "Grab", e.categoryID(aTok, "Transport"))
	e.ai.RespondFunc(aiReads(map[string][]aiRead{"coffee 60": {{"coffee 60", "60", entryToday, "coffee", "Food", true}}}))

	t.Run("two uses, then limit_reached with the parser's items and no call", func(t *testing.T) {
		e := e.with(t)
		for range 2 {
			if got := e.mustParseText(aTok, "coffee 60, grab 145"); got.AI != "used" {
				t.Fatalf("ai %q, want used", got.AI)
			}
		}
		n := len(e.ai.Requests())
		got := e.mustParseText(aTok, "coffee 60, grab 145")
		assertAnswer(t, got, names, "limit_reached",
			"coffee 60 | 60.00 | "+entryToday+" | coffee | - | low | none",
			"grab 145 | 145.00 | "+entryToday+" | Grab | Transport | high | rule")
		if len(e.ai.Requests()) != n {
			t.Error("the AI was called at the limit")
		}
		e.assertUsage(a.id, entryToday+"=2")
	})

	t.Run("at the limit, a text resolved locally still answers not_needed", func(t *testing.T) {
		e := e.with(t)
		if got := e.mustParseText(aTok, "grab 145"); got.AI != "not_needed" {
			t.Errorf("ai %q", got.AI)
		}
	})

	t.Run("cross-user: A at the limit does not stop B, and B's use is not A's", func(t *testing.T) {
		e := e.with(t)
		if got := e.mustParseText(bTok, "coffee 60"); got.AI != "used" {
			t.Errorf("B: ai %q, want used", got.AI)
		}
		e.assertUsage(b.id, entryToday+"=1")
		e.assertUsage(a.id, entryToday+"=2")
	})

	t.Run("a new day in Bangkok starts at 0; the UTC day does not matter", func(t *testing.T) {
		e := e.with(t)
		c := e.newAccount(accountOpts{})
		cTok := e.freshSession(c.id)
		// 16:59:59 UTC is 23:59:59 on 2 October in Bangkok; 17:00 UTC is midnight of 3 October.
		// Both are 2 October in UTC.
		setNow(time.Date(2026, time.October, 2, 16, 59, 59, 0, time.UTC))
		for range 3 {
			e.mustParseText(cTok, "coffee 60")
		}
		e.assertUsage(c.id, "2026-10-02=2")
		if got := e.mustParseText(cTok, "coffee 60"); got.AI != "limit_reached" {
			t.Errorf("before midnight: ai %q", got.AI)
		}
		setNow(time.Date(2026, time.October, 2, 17, 0, 0, 0, time.UTC))
		if got := e.mustParseText(cTok, "coffee 60"); got.AI != "used" {
			t.Errorf("after midnight: ai %q, want used", got.AI)
		}
		e.assertUsage(c.id, "2026-10-02=2", "2026-10-03=1")
		setNow(entryClock)
	})

	t.Run("limit 0: the AI is never called and nothing is counted", func(t *testing.T) {
		e := newAPIEnv(t, withClock(entryClock), withParseLimit(0))
		d := e.newAccount(accountOpts{})
		got := e.mustParseText(e.freshSession(d.id), "coffee 60")
		if got.AI != "limit_reached" || len(e.ai.Requests()) != 0 {
			t.Errorf("ai %q, %d AI requests", got.AI, len(e.ai.Requests()))
		}
		e.assertUsage(d.id)
	})
}

// User B's rules, categories and usage never reach user A's proposals or limit.
func TestEntryParseCrossUser(t *testing.T) {
	e := newAPIEnv(t, withClock(entryClock), withParseLimit(1))
	a, b := e.newAccount(accountOpts{}), e.newAccount(accountOpts{})
	aTok, bTok := e.freshSession(a.id), e.freshSession(b.id)
	aNames := e.categoryNames(aTok)
	bTransport := e.categoryID(bTok, "Transport")
	e.addRule(b.id, "grab", "Grab (B)", bTransport)
	// B is at the limit today.
	e.exec("insert into ai_usage (owner_id, day, parse_count) values ($1, $2::date, 1)", b.id, entryToday)
	e.ai.RespondFunc(aiReads(map[string][]aiRead{"grab 145": {{"grab 145", "145", entryToday, "grab", "Transport", true}}}))

	got := e.mustParseText(aTok, "grab 145")
	assertAnswer(t, got, aNames, "used", "grab 145 | 145.00 | "+entryToday+" | grab | Transport | high | ai")
	if id := got.Items[0].CategoryID; id == nil || *id == bTransport {
		t.Errorf("A's proposal has B's category: %v", id)
	}
	reqs := e.ai.Requests()
	if len(reqs) != 1 {
		t.Fatalf("the AI got %d requests", len(reqs))
	}
	bNames := e.categoryNames(bTok)
	for _, c := range reqs[0].Categories {
		for id, name := range bNames {
			if name == c.Name && aNames[id] != "" {
				t.Errorf("B's category %s was offered for A", id)
			}
		}
	}
	e.assertUsage(a.id, entryToday+"=1")
	e.assertUsage(b.id, entryToday+"=1")

	// B, at the limit, still gets its own rule; B's use is unchanged.
	gotB := e.mustParseText(bTok, "grab 145, coffee 60")
	if gotB.AI != "limit_reached" || gotB.Items[0].ResolvedBy != "rule" || gotB.Items[0].Merchant != "Grab (B)" {
		t.Errorf("B's answer %+v", gotB)
	}
	e.assertUsage(b.id, entryToday+"=1")
}

func TestEntryParseRefusesInput(t *testing.T) {
	e := newAPIEnv(t, withClock(entryClock))
	a := e.newAccount(accountOpts{})
	tok := e.freshSession(a.id)
	twentyOne := strings.TrimSuffix(strings.Repeat("x 1, ", 21), ", ")
	for _, tt := range []struct {
		name, body, msg string
	}{
		{"no text", `{}`, domain.TextRuleMessage},
		{"null text", `{"text":null}`, domain.TextRuleMessage},
		{"empty text", `{"text":""}`, domain.TextRuleMessage},
		{"blank text", `{"text":" \n\t "}`, domain.TextRuleMessage},
		{"1,001 characters", jsonBody(t, map[string]string{"text": strings.Repeat("ก", 998) + " 60 "}), domain.TextRuleMessage},
		{"a date word only", `{"text":"เมื่อวาน"}`, domain.NoItemMessage},
		{"separators only", `{"text":" , ;"}`, domain.NoItemMessage},
		{"21 items", jsonBody(t, map[string]string{"text": twentyOne}), domain.TooManyItemsMessage},
		{"text as a number", `{"text":5}`, ""},
		{"an unknown field", `{"text":"grab 145","style":"clean_name"}`, ""},
		{"not an object", `["grab 145"]`, ""},
		{"malformed", `{"text":`, ""},
		{"empty body", ``, ""},
		{"trailing data", `{"text":"grab 145"} {}`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertBadRequest(t, tt.name, e.postParse(tok, tt.body), tt.msg)
		})
	}
	if n := len(e.ai.Requests()); n != 0 {
		t.Errorf("the AI got %d requests", n)
	}
	e.with(t).assertUsage(a.id)

	t.Run("1,000 characters and 20 items are accepted", func(t *testing.T) {
		e := e.with(t)
		e.ai.SetConfigured(false)
		if got := e.mustParseText(tok, strings.Repeat("ก", 997)+" 60"); len(got.Items) != 1 {
			t.Errorf("items %d", len(got.Items))
		}
		if got := e.mustParseText(tok, strings.TrimSuffix(strings.Repeat("x 1, ", 20), ", ")); len(got.Items) != 20 {
			t.Errorf("items %d", len(got.Items))
		}
	})
}

func TestEntryParseNeedsSession(t *testing.T) {
	e := newAPIEnv(t)
	for _, bad := range []string{"", accountdomain.NewToken().Plain} {
		rec := e.postParse(bad, `{"text":"grab 145"}`)
		if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
			t.Errorf("token %q: %d %s, want 401", bad, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("Cache-Control = %q", got)
		}
	}
	if n := len(e.ai.Requests()); n != 0 {
		t.Errorf("the AI got %d requests", n)
	}
}
