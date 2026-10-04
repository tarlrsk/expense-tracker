package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

func TestCheckAIItem(t *testing.T) {
	today, _ := transactionsdomain.ParseDate("2026-10-04")
	food, transport := uuid.New(), uuid.New()
	categories := map[int]uuid.UUID{1: food, 2: transport}

	// good is an item with every field valid and high confidence.
	good := AIItem{
		Text: "กาแฟ 60", Amount: "60", OccurredOn: "2026-10-03", Merchant: "กาแฟ", CategoryRef: 1, Confidence: ConfidenceHigh,
	}
	with := func(change func(*AIItem)) AIItem {
		i := good
		change(&i)
		return i
	}

	// wantP is the expected proposal: amount and date as text, "" for none.
	type wantP struct {
		text, amount, date, merchant string
		category                     uuid.UUID
		confidence                   Confidence
	}
	tests := []struct {
		name string
		in   AIItem
		want wantP
	}{
		{"all valid keeps high", good, wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", food, ConfidenceHigh}},
		{
			"all valid keeps low", with(func(i *AIItem) { i.Confidence = ConfidenceLow }),
			wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", food, ConfidenceLow},
		},
		{
			"unknown confidence is low", with(func(i *AIItem) { i.Confidence = "medium" }),
			wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", food, ConfidenceLow},
		},
		{
			"decimals and spaces", with(func(i *AIItem) { i.Amount, i.Merchant, i.Text = " 145.5 ", "  Grab  ", " grab 145.5 " }),
			wantP{"grab 145.5", "145.50", "2026-10-03", "Grab", food, ConfidenceHigh},
		},
		{
			"the maximum amount", with(func(i *AIItem) { i.Amount = "9999999999.99" }),
			wantP{"กาแฟ 60", "9999999999.99", "2026-10-03", "กาแฟ", food, ConfidenceHigh},
		},
		{
			"one year ahead is allowed", with(func(i *AIItem) { i.OccurredOn = "2027-10-04" }),
			wantP{"กาแฟ 60", "60.00", "2027-10-04", "กาแฟ", food, ConfidenceHigh},
		},

		// Amount.
		{"no amount", with(func(i *AIItem) { i.Amount = "" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"zero amount", with(func(i *AIItem) { i.Amount = "0" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"negative amount", with(func(i *AIItem) { i.Amount = "-60" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"three decimals", with(func(i *AIItem) { i.Amount = "60.505" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"too large", with(func(i *AIItem) { i.Amount = "10000000000" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"thousands mark", with(func(i *AIItem) { i.Amount = "1,200" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"currency sign", with(func(i *AIItem) { i.Amount = "฿60" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"Thai digits", with(func(i *AIItem) { i.Amount = "๖๐" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},
		{"1k", with(func(i *AIItem) { i.Amount = "1k" }), wantP{"กาแฟ 60", "", "2026-10-03", "กาแฟ", food, ConfidenceLow}},

		// Date.
		{"no date", with(func(i *AIItem) { i.OccurredOn = "" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},
		{"not a date", with(func(i *AIItem) { i.OccurredOn = "yesterday" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},
		{"day first", with(func(i *AIItem) { i.OccurredOn = "03/10/2026" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},
		{"no such day", with(func(i *AIItem) { i.OccurredOn = "2026-02-30" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},
		{"before 2000", with(func(i *AIItem) { i.OccurredOn = "1999-12-31" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},
		{"over a year ahead", with(func(i *AIItem) { i.OccurredOn = "2027-10-05" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},
		{"Buddhist-era year", with(func(i *AIItem) { i.OccurredOn = "2569-10-03" }), wantP{"กาแฟ 60", "60.00", "", "กาแฟ", food, ConfidenceLow}},

		// Merchant.
		{"no merchant", with(func(i *AIItem) { i.Merchant = "" }), wantP{"กาแฟ 60", "60.00", "2026-10-03", "", food, ConfidenceLow}},
		{"blank merchant", with(func(i *AIItem) { i.Merchant = "   " }), wantP{"กาแฟ 60", "60.00", "2026-10-03", "", food, ConfidenceLow}},
		{
			"merchant too long", with(func(i *AIItem) { i.Merchant = strings.Repeat("ก", 101) }),
			wantP{"กาแฟ 60", "60.00", "2026-10-03", "", food, ConfidenceLow},
		},
		{
			"merchant at the limit", with(func(i *AIItem) { i.Merchant = strings.Repeat("ก", 100) }),
			wantP{"กาแฟ 60", "60.00", "2026-10-03", strings.Repeat("ก", 100), food, ConfidenceHigh},
		},
		{
			"merchant with a control character", with(func(i *AIItem) { i.Merchant = "caf\x07e" }),
			wantP{"กาแฟ 60", "60.00", "2026-10-03", "", food, ConfidenceLow},
		},

		// Category.
		{"second category", with(func(i *AIItem) { i.CategoryRef = 2 }), wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", transport, ConfidenceHigh}},
		{"no category", with(func(i *AIItem) { i.CategoryRef = 0 }), wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", uuid.Nil, ConfidenceLow}},
		{"category not in the list", with(func(i *AIItem) { i.CategoryRef = 3 }), wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", uuid.Nil, ConfidenceLow}},
		{"negative category", with(func(i *AIItem) { i.CategoryRef = -1 }), wantP{"กาแฟ 60", "60.00", "2026-10-03", "กาแฟ", uuid.Nil, ConfidenceLow}},

		// Several bad fields, and nothing read at all: the item is still kept.
		{
			"everything bad", AIItem{Text: "hello there", Amount: "lots", OccurredOn: "soon", Merchant: "", CategoryRef: 7, Confidence: ConfidenceHigh},
			wantP{"hello there", "", "", "", uuid.Nil, ConfidenceLow},
		},
		{"empty item", AIItem{}, wantP{"", "", "", "", uuid.Nil, ConfidenceLow}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAIItem(tt.in, today, categories)
			amount := ""
			if got.HasAmount {
				amount = got.Amount.String()
			} else if got.Amount != 0 {
				t.Errorf("Amount = %v without HasAmount", got.Amount)
			}
			gotP := wantP{got.Text, amount, got.OccurredOn.String(), got.Merchant, got.CategoryID, got.Confidence}
			if gotP != tt.want {
				t.Errorf("CheckAIItem = %+v\nwant          %+v", gotP, tt.want)
			}
		})
	}
}

// A nil id in the caller's list is treated as no category.
func TestCheckAIItemNilCategory(t *testing.T) {
	today, _ := transactionsdomain.ParseDate("2026-10-04")
	got := CheckAIItem(AIItem{Amount: "5", OccurredOn: "2026-10-04", Merchant: "x", CategoryRef: 1, Confidence: ConfidenceHigh},
		today, map[int]uuid.UUID{1: uuid.Nil})
	if got.CategoryID != uuid.Nil || got.Confidence != ConfidenceLow {
		t.Errorf("CheckAIItem = %+v, want no category and low", got)
	}
}
