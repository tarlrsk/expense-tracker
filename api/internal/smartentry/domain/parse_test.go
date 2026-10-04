package domain

import (
	"strings"
	"testing"

	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// want is one expected item: amount "" means none, date "" means undecided.
type want struct {
	text, amount, date, merchant string
	complete                     bool
}

func TestParse(t *testing.T) {
	today, _ := transactionsdomain.ParseDate("2026-10-04")
	const (
		d0 = "2026-10-04" // today
		d1 = "2026-10-03" // yesterday
		d2 = "2026-10-02" // the day before yesterday
	)
	long := strings.Repeat("a", 101)
	tests := []struct {
		name string
		in   string
		want []want
	}{
		// One item.
		{"English", "grab 145", []want{{"grab 145", "145.00", d0, "grab", true}}},
		{"Thai", "ข้าวมันไก่ 60", []want{{"ข้าวมันไก่ 60", "60.00", d0, "ข้าวมันไก่", true}}},
		{"amount first", "60 coffee", []want{{"60 coffee", "60.00", d0, "coffee", true}}},
		{"amount in the middle", "cafe 60 amazon", []want{{"cafe 60 amazon", "60.00", d0, "cafe amazon", true}}},
		{"case kept", "Cafe Amazon 75", []want{{"Cafe Amazon 75", "75.00", d0, "Cafe Amazon", true}}},
		{"spaces collapsed", "  cafe \t  amazon   75  ", []want{{"cafe \t  amazon   75", "75.00", d0, "cafe amazon", true}}},
		{"hyphenated name with a digit", "7-eleven 60", []want{{"7-eleven 60", "60.00", d0, "7-eleven", true}}},
		{"decimals", "grab 145.50", []want{{"grab 145.50", "145.50", d0, "grab", true}}},
		{"one decimal", "grab 145.5", []want{{"grab 145.5", "145.50", d0, "grab", true}}},
		{"thousands mark", "rent 1,200", []want{{"rent 1,200", "1200.00", d0, "rent", true}}},
		{"thousands mark and decimals", "rent 12,000.50", []want{{"rent 12,000.50", "12000.50", d0, "rent", true}}},
		{"two thousands marks", "car 1,250,000", []want{{"car 1,250,000", "1250000.00", d0, "car", true}}},
		{"the maximum", "x 9999999999.99", []want{{"x 9999999999.99", "9999999999.99", d0, "x", true}}},
		{"a leftover token stays in the merchant", "coffee 60 3/10", []want{{"coffee 60 3/10", "60.00", d0, "coffee 3/10", true}}},
		{"a written date is not read", "coffee 60 2026-10-03", []want{{"coffee 60 2026-10-03", "60.00", d0, "coffee 2026-10-03", true}}},

		// Currency markers.
		{"บาท attached after", "ข้าว 60บาท", []want{{"ข้าว 60บาท", "60.00", d0, "ข้าว", true}}},
		{"฿ attached before", "coffee ฿60", []want{{"coffee ฿60", "60.00", d0, "coffee", true}}},
		{"baht after", "coffee 60 baht", []want{{"coffee 60 baht", "60.00", d0, "coffee", true}}},
		{"THB before", "coffee THB 60", []want{{"coffee THB 60", "60.00", d0, "coffee", true}}},
		{"บาท as a token", "ข้าว 60 บาท", []want{{"ข้าว 60 บาท", "60.00", d0, "ข้าว", true}}},
		{"Baht attached, any case", "coffee 60Baht", []want{{"coffee 60Baht", "60.00", d0, "coffee", true}}},
		{"thb attached before thousands", "rent thb1,200", []want{{"rent thb1,200", "1200.00", d0, "rent", true}}},
		{"฿ as a token", "coffee ฿ 60", []want{{"coffee ฿ 60", "60.00", d0, "coffee", true}}},
		{"a marker away from the amount", "baht coffee 60", []want{{"baht coffee 60", "60.00", d0, "baht coffee", false}}},
		{"two markers", "coffee ฿60 baht", []want{{"coffee ฿60 baht", "60.00", d0, "coffee", false}}},
		{"markers on both sides", "coffee thb 60 baht", []want{{"coffee thb 60 baht", "60.00", d0, "coffee", false}}},
		{"two attached markers", "coffee ฿60บาท", []want{{"coffee ฿60บาท", "60.00", d0, "coffee", false}}},
		{"a marker without an amount", "coffee baht", []want{{"coffee baht", "", d0, "coffee baht", false}}},

		// Not amounts: never guessed.
		{"1k", "rent 1k", []want{{"rent 1k", "", d0, "rent 1k", false}}},
		{"1.5k", "rent 1.5k", []want{{"rent 1.5k", "", d0, "rent 1.5k", false}}},
		{"Thai digits", "ข้าว ๖๐", []want{{"ข้าว ๖๐", "", d0, "ข้าว", false}}},
		{"a sum", "coffee 60+20", []want{{"coffee 60+20", "", d0, "coffee 60+20", false}}},
		{"three decimals", "grab 145.505", []want{{"grab 145.505", "", d0, "grab", false}}},
		{"zero", "grab 0", []want{{"grab 0", "", d0, "grab", false}}},
		{"over the maximum", "x 10000000000", []want{{"x 10000000000", "", d0, "x", false}}},
		{"over the maximum with thousands marks", "x 10,000,000,000", []want{{"x 10,000,000,000", "", d0, "x", false}}},
		{"a negative number", "grab -60", []want{{"grab -60", "", d0, "grab -60", false}}},
		{"a trailing point", "grab 60.", []want{{"grab 60.", "", d0, "grab", false}}},
		{"a thousands mark after a point", "grab 1.5,000", []want{{"grab 1.5,000", "", d0, "grab", false}}},
		{"full-width digits", "grab ６０", []want{{"grab ６０", "", d0, "grab", false}}},

		// Not one amount.
		{"two numbers", "7 eleven 60", []want{{"7 eleven 60", "", d0, "7 eleven 60", false}}},
		{"a number in the trailing words", "grab 145 2 days ago", []want{{"grab 145 2 days ago", "", d0, "grab 145 2 days ago", false}}},
		{"a valid and an invalid number", "coffee 60 145.505", []want{{"coffee 60 145.505", "", d0, "coffee 60 145.505", false}}},
		{"no amount", "coffee", []want{{"coffee", "", d0, "coffee", false}}},
		{"amount only", "60", []want{{"60", "60.00", d0, "", false}}},
		{"amount and marker only", "฿60", []want{{"฿60", "60.00", d0, "", false}}},

		// Merchant rules.
		{"a merchant of 100 characters", strings.Repeat("a", 100) + " 60", []want{{strings.Repeat("a", 100) + " 60", "60.00", d0, strings.Repeat("a", 100), true}}},
		{"a merchant over 100 characters", long + " 60", []want{{long + " 60", "60.00", d0, "", false}}},
		{"a control character in the merchant", "cof\x00fee 60", []want{{"cof\x00fee 60", "60.00", d0, "", false}}},

		// Several items.
		{"comma", "coffee 60, grab 145", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 145", "145.00", d0, "grab", true},
		}},
		{"comma without a space", "coffee 60,grab 145", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 145", "145.00", d0, "grab", true},
		}},
		{"a comma before two digits splits", "coffee 1,20 grab", []want{
			{"coffee 1", "1.00", d0, "coffee", true}, {"20 grab", "20.00", d0, "grab", true},
		}},
		{"a comma before four digits splits", "coffee 1,2000 grab", []want{
			{"coffee 1", "1.00", d0, "coffee", true}, {"2000 grab", "2000.00", d0, "grab", true},
		}},
		{"a comma after four digits splits", "coffee 1234,567 grab", []want{
			{"coffee 1234", "1234.00", d0, "coffee", true}, {"567 grab", "567.00", d0, "grab", true},
		}},
		{"a comma and a space split", "coffee 1, 200 grab", []want{
			{"coffee 1", "1.00", d0, "coffee", true}, {"200 grab", "200.00", d0, "grab", true},
		}},
		{"semicolon", "coffee 60; grab 145", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 145", "145.00", d0, "grab", true},
		}},
		{"line breaks", "coffee 60\ngrab 145\r\nข้าวมันไก่ 50", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 145", "145.00", d0, "grab", true},
			{"ข้าวมันไก่ 50", "50.00", d0, "ข้าวมันไก่", true},
		}},
		{"trailing and doubled separators", ",coffee 60,, ;grab 145;\n\n", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 145", "145.00", d0, "grab", true},
		}},
		{"rent and a thousands mark beside an item comma", "rent 12,000.50, water 1,200", []want{
			{"rent 12,000.50", "12000.50", d0, "rent", true}, {"water 1,200", "1200.00", d0, "water", true},
		}},
		{"one complete and one not", "coffee 60, grab 1k", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 1k", "", d0, "grab 1k", false},
		}},

		// Dates.
		{"yesterday", "grab 145 yesterday", []want{{"grab 145 yesterday", "145.00", d1, "grab", true}}},
		{"เมื่อวาน", "ข้าวมันไก่ 60 เมื่อวาน", []want{{"ข้าวมันไก่ 60 เมื่อวาน", "60.00", d1, "ข้าวมันไก่", true}}},
		{"เมื่อวานนี้", "ข้าวมันไก่ 60 เมื่อวานนี้", []want{{"ข้าวมันไก่ 60 เมื่อวานนี้", "60.00", d1, "ข้าวมันไก่", true}}},
		{"เมื่อวานซืน", "กาแฟ 60 เมื่อวานซืน", []want{{"กาแฟ 60 เมื่อวานซืน", "60.00", d2, "กาแฟ", true}}},
		{"วันนี้", "วันนี้ กาแฟ 60", []want{{"วันนี้ กาแฟ 60", "60.00", d0, "กาแฟ", true}}},
		{"today, any case", "Coffee 60 TODAY", []want{{"Coffee 60 TODAY", "60.00", d0, "Coffee", true}}},
		{"Yesterday first", "Yesterday coffee 60", []want{{"Yesterday coffee 60", "60.00", d1, "coffee", true}}},
		{"a date word only as a whole token", "coffee 60 yesterday's", []want{{"coffee 60 yesterday's", "60.00", d0, "coffee yesterday's", true}}},
		{"a date word inside the item", "coffee yesterday 60", []want{{"coffee yesterday 60", "60.00", d1, "coffee", true}}},
		{"the last date word applies to every item", "coffee 60, grab 145 yesterday", []want{
			{"coffee 60", "60.00", d1, "coffee", true}, {"grab 145 yesterday", "145.00", d1, "grab", true},
		}},
		{"the first date word applies to every item", "เมื่อวาน ข้าว 50, กาแฟ 60", []want{
			{"เมื่อวาน ข้าว 50", "50.00", d1, "ข้าว", true}, {"กาแฟ 60", "60.00", d1, "กาแฟ", true},
		}},
		{"a date word in a middle item is its own only", "coffee 60, grab 145 yesterday, rice 50", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"grab 145 yesterday", "145.00", d1, "grab", true},
			{"rice 50", "50.00", d0, "rice", true},
		}},
		{"an item's own date word beats the text's", "coffee 60 today, grab 145 yesterday", []want{
			{"coffee 60 today", "60.00", d0, "coffee", true}, {"grab 145 yesterday", "145.00", d1, "grab", true},
		}},
		{"a date-only part at the start", "yesterday, coffee 60, grab 145", []want{
			{"coffee 60", "60.00", d1, "coffee", true}, {"grab 145", "145.00", d1, "grab", true},
		}},
		{"a date-only line at the end", "coffee 60\ngrab 145\nเมื่อวานซืน", []want{
			{"coffee 60", "60.00", d2, "coffee", true}, {"grab 145", "145.00", d2, "grab", true},
		}},
		{"a date-only part in the middle is an item", "coffee 60, yesterday, grab 145", []want{
			{"coffee 60", "60.00", d0, "coffee", true}, {"yesterday", "", d1, "", false}, {"grab 145", "145.00", d0, "grab", true},
		}},
		{"start and end words that differ", "yesterday coffee 60, grab 145, rice 50 today", []want{
			{"yesterday coffee 60", "60.00", d1, "coffee", true}, {"grab 145", "145.00", "", "grab", false},
			{"rice 50 today", "50.00", d0, "rice", true},
		}},
		{"start and end words that differ, date-only parts", "yesterday, coffee 60, today", []want{
			{"coffee 60", "60.00", "", "coffee", false},
		}},
		{"start and end words of the same day agree", "yesterday, coffee 60, grab 145 เมื่อวาน", []want{
			{"coffee 60", "60.00", d1, "coffee", true}, {"grab 145 เมื่อวาน", "145.00", d1, "grab", true},
		}},
		{"two date words in one item", "coffee 60 yesterday today", []want{{"coffee 60 yesterday today", "60.00", "", "coffee", false}}},
		{"two date words at the start and end of one item", "yesterday coffee 60 today", []want{{"yesterday coffee 60 today", "60.00", "", "coffee", false}}},

		// No items.
		{"empty", "", nil},
		{"spaces only", "  \t ", nil},
		{"separators only", " , ;\n\r\n", nil},
		{"a date word only", "เมื่อวานซืน", nil},
		{"date words only", "yesterday, today", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.in, today)
			if len(got) != len(tt.want) {
				t.Fatalf("Parse(%q) = %d items %+v, want %d", tt.in, len(got), got, len(tt.want))
			}
			for i, w := range tt.want {
				g := got[i]
				amount := ""
				if g.HasAmount {
					amount = g.Amount.String()
				} else if g.Amount != 0 {
					t.Errorf("item %d: amount %s without HasAmount", i, g.Amount)
				}
				if g.Text != w.text || amount != w.amount || g.OccurredOn.String() != w.date || g.Merchant != w.merchant ||
					g.Complete != w.complete {
					t.Errorf("Parse(%q) item %d = {%q %q %q %q %v}, want %+v", tt.in, i, g.Text, amount, g.OccurredOn, g.Merchant, g.Complete, w)
				}
			}
		})
	}
}

// Today is whatever day the caller says: the parser reads no clock.
func TestParseDaysAcrossMonthAndYear(t *testing.T) {
	newYear, _ := transactionsdomain.ParseDate("2027-01-01")
	got := Parse("coffee 60 เมื่อวานซืน, grab 145 yesterday, rice 50", newYear)
	dates := []string{"2026-12-30", "2026-12-31", "2027-01-01"}
	if len(got) != len(dates) {
		t.Fatalf("got %d items", len(got))
	}
	for i, d := range dates {
		if got[i].OccurredOn.String() != d {
			t.Errorf("item %d on %s, want %s", i, got[i].OccurredOn, d)
		}
	}
}
