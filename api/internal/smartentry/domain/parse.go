// Package domain holds the smart entry module's pure rules: the local parser that reads quick
// entry text into items without AI (docs/03-modules.md §M4, PLAN-0003 T3). It touches no
// database.
//
// The parser never guesses: an item it cannot read with certainty is marked not complete, and the
// AI reads it later.
package domain

import (
	"regexp"
	"strings"
	"unicode"

	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// Item is one item of the text, in the order written.
type Item struct {
	// Text is the item's part of the text, trimmed.
	Text string
	// Amount is the amount read; HasAmount is false when there is none (no number, more than one
	// number, or a number that breaks the amount rules of ADR-0071).
	Amount    transactionsdomain.Amount
	HasAmount bool
	// OccurredOn is the item's day; the zero Date when it cannot be decided (more than one date
	// word in the item, or different date words at the start and end of the text).
	OccurredOn transactionsdomain.Date
	// Merchant is what is left once the amount, its currency marker and the date word are taken
	// out, joined by single spaces, as written. It is "" when it breaks the merchant rules.
	Merchant string
	// Complete is true when the item has exactly one valid amount, a merchant, a day and no
	// stray currency marker.
	Complete bool
}

// dateWords are the words that name a day, as days before today; they count only as a whole
// token, in any case.
var dateWords = map[string]int{
	"today":       0,
	"วันนี้":      0,
	"yesterday":   1,
	"เมื่อวาน":    1,
	"เมื่อวานนี้": 1,
	"เมื่อวานซืน": 2,
}

// currencyMarkers mark an amount as baht, in lower case. One may be attached to the amount
// (฿60, 60บาท) or be the token just before or after it (60 baht, THB 60).
var currencyMarkers = []string{"฿", "บาท", "baht", "thb"}

// thousandsPattern is a number written with thousands marks, such as 1,200 or 12,000,000. Its
// commas do not split the text into items.
var thousandsPattern = regexp.MustCompile(`[0-9]{1,3}(?:,[0-9]{3})+`)

// amountPattern is an amount as the parser reads it: digits, or digits with thousands marks,
// with an optional fraction. ParseAmount then applies the amount rules.
var amountPattern = regexp.MustCompile(`^(?:[0-9]+|[0-9]{1,3}(?:,[0-9]{3})+)(?:\.[0-9]+)?$`)

// Parse reads text into items. today is the current day in the app time zone (ADR-0042).
//
// Items are split on commas, semicolons and line breaks; a comma inside a number with thousands
// marks (1,200) does not split. A date word that is the first or the last token of the whole
// text is the day of every item without a date word of its own; a part that is only such a word
// gives no item. Empty and whitespace-only text gives no items.
func Parse(text string, today transactionsdomain.Date) []Item {
	var chunks [][]string
	var raw []string
	for _, c := range split(text) {
		if c = strings.TrimSpace(c); c != "" {
			raw = append(raw, c)
			chunks = append(chunks, strings.Fields(c))
		}
	}
	if len(chunks) == 0 {
		return nil
	}

	start, hasStart := dateWord(chunks[0][0])
	last := chunks[len(chunks)-1]
	end, hasEnd := dateWord(last[len(last)-1])
	textDate := today
	textDateKnown := true
	switch {
	case hasStart && hasEnd && start != end:
		textDateKnown = false
	case hasStart:
		textDate = today.AddDays(-start)
	case hasEnd:
		textDate = today.AddDays(-end)
	}

	items := make([]Item, 0, len(chunks))
	for i, tokens := range chunks {
		if ((i == 0 && hasStart) || (i == len(chunks)-1 && hasEnd)) && len(tokens) == 1 {
			continue // only the text's date word
		}
		items = append(items, parseItem(raw[i], tokens, today, textDate, textDateKnown))
	}
	return items
}

// split cuts text at commas, semicolons and line breaks, keeping the commas of numbers with
// thousands marks.
func split(text string) []string {
	keep := map[int]bool{}
	for _, m := range thousandsPattern.FindAllStringIndex(text, -1) {
		if (m[0] > 0 && isASCIIDigit(text[m[0]-1])) || (m[1] < len(text) && isASCIIDigit(text[m[1]])) {
			continue // part of a longer run of digits: not a thousands mark
		}
		for i := m[0]; i < m[1]; i++ {
			if text[i] == ',' {
				keep[i] = true
			}
		}
	}
	var chunks []string
	from := 0
	for i := 0; i < len(text); i++ {
		if c := text[i]; c == ';' || c == '\n' || (c == ',' && !keep[i]) {
			chunks = append(chunks, text[from:i])
			from = i + 1
		}
	}
	return append(chunks, text[from:])
}

// parseItem reads one item from its tokens. textDate is the day of an item without a date word;
// textDateKnown is false when the text's start and end date words differ.
func parseItem(text string, tokens []string, today, textDate transactionsdomain.Date, textDateKnown bool) Item {
	item := Item{Text: text}
	var numbers, dates, markers []int
	attached := map[int]int{} // markers attached to a number token
	days := 0
	for i, tok := range tokens {
		if d, ok := dateWord(tok); ok {
			dates = append(dates, i)
			days = d
			continue
		}
		if isMarker(tok) {
			markers = append(markers, i)
			continue
		}
		if _, n, ok := number(tok); ok {
			numbers = append(numbers, i)
			attached[i] = n
		}
	}

	read := map[int]bool{}
	for _, i := range dates {
		read[i] = true
	}
	ok := true
	amountAt := -1
	if len(numbers) == 1 {
		amountAt = numbers[0]
		read[amountAt] = true
		core, _, _ := number(tokens[amountAt])
		item.Amount, item.HasAmount = parseAmount(core)
	}
	marked := attached[amountAt]
	for _, i := range markers {
		if amountAt >= 0 && (i == amountAt-1 || i == amountAt+1) {
			read[i] = true
			marked++
		} else {
			ok = false // a stray currency marker
		}
	}
	if marked > 1 {
		ok = false
	}

	switch {
	case len(dates) == 1:
		item.OccurredOn = today.AddDays(-days)
	case len(dates) == 0 && textDateKnown:
		item.OccurredOn = textDate
	}

	var rest []string
	for i, tok := range tokens {
		if !read[i] {
			rest = append(rest, tok)
		}
	}
	merchant, err := transactionsdomain.NormalizeMerchant(strings.Join(rest, " "))
	if err == nil {
		item.Merchant = merchant
	}

	item.Complete = ok && item.HasAmount && item.Merchant != "" && !item.OccurredOn.IsZero()
	return item
}

// dateWord returns how many days before today tok names, when it is a date word.
func dateWord(tok string) (int, bool) {
	d, ok := dateWords[strings.ToLower(tok)]
	return d, ok
}

func isMarker(tok string) bool {
	tok = strings.ToLower(tok)
	for _, m := range currencyMarkers {
		if tok == m {
			return true
		}
	}
	return false
}

// number reports whether tok is a number: once at most one currency marker is cut from its start
// and one from its end, only digits (of any script), points and commas are left, with at least
// one digit. core is what is left and markers how many markers were cut. A number need not be a
// valid amount (145.505, ๖๐ and 0 are numbers); 1k, 60+20 and 3/10 are not numbers.
func number(tok string) (core string, markers int, ok bool) {
	core = strings.ToLower(tok)
	for _, m := range currencyMarkers {
		if rest, cut := strings.CutPrefix(core, m); cut {
			core, markers = rest, markers+1
			break
		}
	}
	for _, m := range currencyMarkers {
		if rest, cut := strings.CutSuffix(core, m); cut {
			core, markers = rest, markers+1
			break
		}
	}
	digit := false
	for _, r := range core {
		switch {
		case unicode.IsDigit(r):
			digit = true
		case r != '.' && r != ',':
			return "", 0, false
		}
	}
	return core, markers, digit
}

// parseAmount reads a number as an amount: ASCII digits, thousands marks only in groups of three,
// then the rules of transactionsdomain.ParseAmount (ADR-0071).
func parseAmount(core string) (transactionsdomain.Amount, bool) {
	if !amountPattern.MatchString(core) {
		return 0, false
	}
	a, err := transactionsdomain.ParseAmount(strings.ReplaceAll(core, ",", ""))
	return a, err == nil
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }
