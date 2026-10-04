package parse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	categoriesdomain "github.com/tarlrsk/expense-tracker/api/internal/categories/domain"
	aiparse "github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/smartentry/domain"
	smartentrymatchrulesproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/matchrules"
	smartentryparseproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/parse"
	smartentryreserveproc "github.com/tarlrsk/expense-tracker/api/internal/smartentry/processor/reserve"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type orchestrator struct {
	parse   smartentryparseproc.Processor
	reserve smartentryreserveproc.Processor
	match   smartentrymatchrulesproc.Processor
	ai      aiparse.Port
	logger  *slog.Logger
}

// New returns the quick-entry use case.
func New(parse smartentryparseproc.Processor, reserve smartentryreserveproc.Processor,
	match smartentrymatchrulesproc.Processor, ai aiparse.Port, logger *slog.Logger,
) Orchestrator {
	return &orchestrator{parse: parse, reserve: reserve, match: match, ai: ai, logger: logger}
}

// Execute runs, each step in its own user transaction and the AI call between them (ADR-0032):
//  1. the local parse with its rule lookup; when every item is resolved, nothing else runs;
//  2. when the AI is configured, the reservation of one use toward the daily limit, committed
//     before the call so a failed call still counts, with the caller's active categories;
//  3. the AI call with only the unresolved items, as numbered lines, each with the day the
//     parser worked out for it (today when it could not decide);
//  4. the checks of every AI item (ADR-0071), then the caller's merchant rules over the AI's
//     merchants: a rule's category comes first (§M5).
//
// The AI's items take the place of their line, in the AI's order; a line the AI gave no item
// for keeps the parser's reading.
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	text, err := domain.CheckText(req.Text)
	if err != nil {
		return Response{}, err
	}
	local, err := o.parse.Execute(ctx, smartentryparseproc.Request{UserID: req.UserID, Text: text})
	if err != nil {
		return Response{}, fmt.Errorf("parse quick entry: %w", err)
	}
	switch n := len(local.Items); {
	case n == 0:
		return Response{}, apperr.New(apperr.InvalidInput, domain.NoItemMessage)
	case n > domain.MaxItems:
		return Response{}, apperr.New(apperr.InvalidInput, domain.TooManyItemsMessage)
	}

	items := make([]Item, len(local.Items))
	var pending []int // the items the AI is asked about
	for i, it := range local.Items {
		items[i] = localItem(it)
		if !it.Resolved {
			pending = append(pending, i)
		}
	}
	if len(pending) == 0 {
		return Response{Items: items, AI: domain.AINotNeeded}, nil
	}
	if !o.ai.Configured() {
		return Response{Items: items, AI: domain.AINotConfigured}, nil
	}

	reserved, err := o.reserve.Execute(ctx, smartentryreserveproc.Request{UserID: req.UserID})
	if err != nil {
		return Response{}, fmt.Errorf("parse quick entry: %w", err)
	}
	if !reserved.Reserved {
		return Response{Items: items, AI: domain.AILimitReached}, nil
	}

	today := reserved.Day
	lines := make([]aiparse.Line, len(pending))
	for n, i := range pending {
		day := local.Items[i].OccurredOn
		if day.IsZero() {
			day = today
		}
		lines[n] = aiparse.Line{Number: n + 1, Text: local.Items[i].Text, DateIfNone: day.String()}
	}
	categories, refs := offer(reserved.Categories)
	style := req.Style
	if style == "" {
		style = aiparse.StyleAsTyped
	}
	answer, err := o.ai.Parse(ctx, aiparse.Request{Lines: lines, Today: today.String(), Categories: categories, Style: style})
	if err != nil {
		return o.aiFailed(ctx, req.UserID, items, err)
	}
	for _, a := range answer.Items {
		if a.Line < 1 || a.Line > len(lines) { // the port's own check; kept so a bad port cannot panic here
			return o.aiFailed(ctx, req.UserID, items, fmt.Errorf("%w: an item names a line that was not sent", aiparse.ErrUnusable))
		}
	}

	aiItems := make([]domain.AIItem, len(answer.Items))
	proposals := make([]domain.Proposal, len(answer.Items))
	merchants := make([]string, len(answer.Items))
	for j, a := range answer.Items {
		aiItems[j] = domain.AIItem{
			Text: a.Text, Amount: a.Amount, OccurredOn: a.OccurredOn, Merchant: a.Merchant,
			CategoryRef: a.CategoryRef, Confidence: domain.Confidence(a.Confidence),
		}
		proposals[j] = domain.CheckAIItem(aiItems[j], today, refs)
		proposals[j].Text = itemText(proposals[j].Text, lines[a.Line-1].Text)
		merchants[j] = proposals[j].Merchant
	}
	matched, err := o.match.Execute(ctx, smartentrymatchrulesproc.Request{UserID: req.UserID, Merchants: merchants})
	if err != nil {
		return Response{}, fmt.Errorf("parse quick entry: %w", err)
	}

	byLine := make(map[int][]Item, len(lines))
	for j, a := range answer.Items {
		p, by := proposals[j], domain.ResolvedByAI
		if m := matched.Matches[j]; m.Found {
			p, by = domain.ApplyRule(aiItems[j], p, m.Rule.CategoryID), domain.ResolvedByRule
		}
		byLine[a.Line] = append(byLine[a.Line], proposalItem(p, by))
	}
	lineOf := make(map[int]int, len(pending)) // item index -> line number
	for n, i := range pending {
		lineOf[i] = n + 1
	}
	out := make([]Item, 0, len(items)+len(answer.Items))
	for i, it := range items {
		if n, sent := lineOf[i]; sent && len(byLine[n]) > 0 {
			out = append(out, byLine[n]...)
			continue
		}
		out = append(out, it)
	}
	return Response{Items: out, AI: domain.AIUsed}, nil
}

// aiFailed answers a failed AI call. When the request's own deadline is over, the request fails
// (timeout, ADR-0043). A call made inside a transaction is a fault of this code. Any other
// failure is logged, without the text, key or answer (the port's errors quote none), and the
// parser's items come back.
func (o *orchestrator) aiFailed(ctx context.Context, userID uuid.UUID, items []Item, err error) (Response, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Response{}, fmt.Errorf("parse quick entry: %w: %w", ctxErr, err)
	}
	if errors.Is(err, tx.ErrInside) {
		return Response{}, fmt.Errorf("parse quick entry: %w", err)
	}
	status := domain.AIUnavailable
	if errors.Is(err, aiparse.ErrNotConfigured) {
		status = domain.AINotConfigured
	}
	o.logger.LogAttrs(ctx, slog.LevelError, "AI parse failed",
		slog.String("use_case", "parse quick entry"), slog.String("user_id", userID.String()),
		slog.String("ai", string(status)), slog.String("error", err.Error()))
	return Response{Items: items, AI: status}, nil
}

// offer numbers the categories 1..n in the caller's order for the AI, and maps each number back.
// Real ids are not sent.
func offer(cats []categoriesdomain.Category) ([]aiparse.Category, map[int]uuid.UUID) {
	out := make([]aiparse.Category, len(cats))
	refs := make(map[int]uuid.UUID, len(cats))
	for i, c := range cats {
		out[i] = aiparse.Category{Ref: i + 1, Name: c.Name, Kind: aiparse.Kind(c.Kind)}
		refs[i+1] = c.ID
	}
	return out, refs
}

// itemText is the AI's text of an item when it is part of the line it came from, otherwise the
// whole line: the text of a proposal is always something the user typed.
func itemText(aiText, line string) string {
	if aiText != "" && strings.Contains(line, aiText) {
		return aiText
	}
	return line
}

// localItem is an item as the local parser read it: resolved by a rule (high), or not resolved
// (low, no category).
func localItem(it smartentryparseproc.Item) Item {
	item := Item{
		Text: it.Text, Amount: it.Amount, HasAmount: it.HasAmount, OccurredOn: it.OccurredOn, Merchant: it.Merchant,
		Confidence: domain.ConfidenceLow, ResolvedBy: domain.ResolvedByNone,
	}
	if it.Resolved {
		item.CategoryID, item.Confidence, item.ResolvedBy = it.CategoryID, domain.ConfidenceHigh, domain.ResolvedByRule
	}
	return item
}

func proposalItem(p domain.Proposal, by domain.ResolvedBy) Item {
	return Item{
		Text: p.Text, Amount: p.Amount, HasAmount: p.HasAmount, OccurredOn: p.OccurredOn, Merchant: p.Merchant,
		CategoryID: p.CategoryID, Confidence: p.Confidence, ResolvedBy: by,
	}
}
