package domain

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMerchantKey(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"hyphen dropped", "7-Eleven", "7eleven"},
		{"space dropped", "7 eleven", "7eleven"},
		{"upper case", "7ELEVEN", "7eleven"},
		{"plain", "Grab", "grab"},
		{"spaces around", " grab ", "grab"},
		{"all caps", "GRAB", "grab"},
		{"full-width letters (NFKC)", "ＧＲＡＢ", "grab"},
		{"full-width digit and hyphen (NFKC)", "７－ELEVEN", "7eleven"},
		{"Thai with vowels and tone marks kept", "ข้าวมันไก่", "ข้าวมันไก่"},
		{"Thai with spaces", "ข้าว มัน ไก่", "ข้าวมันไก่"},
		{"Thai name of 7-Eleven is its own key", "เซเว่น", "เซเว่น"},
		{"Thai digits are digits, not mapped to ASCII", "๗-eleven", "๗eleven"},
		{"Thai sara am (NFKC splits it into nikhahit and sara aa)", "น้ำ", "น้ํา"},
		{"one word", "Amazon", "amazon"},
		{"two words", "Cafe Amazon", "cafeamazon"},
		{"composed accent", "Café", "café"},
		{"combining accent (NFKC composes it)", "Café", "café"},
		{"punctuation and symbols dropped", "McDonald's® (Siam) ☕", "mcdonaldssiam"},
		{"tab and line break dropped", "Big\tC\n", "bigc"},
		{"only punctuation: no key", "---", ""},
		{"empty: no key", "", ""},
		{"only spaces: no key", "   ", ""},
		{"100 characters", strings.Repeat("a", 100), strings.Repeat("a", 100)},
		{"101 characters: no key", strings.Repeat("a", 101), ""},
		{"NFKC lengthens past 100: no key", strings.Repeat("ﷺ", 7), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MerchantKey(tt.in); got != tt.want {
				t.Errorf("MerchantKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMerchantKeyEquivalence(t *testing.T) {
	tests := []struct {
		a, b string
		same bool
	}{
		{"7-Eleven", "7 eleven", true},
		{"7-Eleven", "7ELEVEN", true},
		{"Grab", " grab ", true},
		{"Grab", "ＧＲＡＢ", true},
		{"ข้าวมันไก่", "ข้าว มัน ไก่", true},
		{"Big C", "BigC", true},
		{"น้ำ", "น้ํา", true},
		{"เซเว่น", "7-Eleven", false},
		{"Amazon", "Cafe Amazon", false},
		{"ข้าวมันไก่", "ข้าวมันไก", false}, // a tone mark is part of the key
	}
	for _, tt := range tests {
		ka, kb := MerchantKey(tt.a), MerchantKey(tt.b)
		if ka == "" || kb == "" {
			t.Errorf("no key for %q (%q) or %q (%q)", tt.a, ka, tt.b, kb)
		}
		if (ka == kb) != tt.same {
			t.Errorf("MerchantKey(%q) = %q, MerchantKey(%q) = %q; same = %v, want %v", tt.a, ka, tt.b, kb, ka == kb, tt.same)
		}
	}
}

// A key is what merchant_rules accepts: trimmed, not empty, at most MaxLength characters.
func TestMerchantKeyFitsTheTable(t *testing.T) {
	for _, in := range []string{"7-Eleven", " ข้าว มัน ไก่ ", strings.Repeat("ab ", 60), strings.Repeat("ﷺ", 6)} {
		k := MerchantKey(in)
		if k == "" {
			continue
		}
		if k != strings.TrimSpace(k) || utf8.RuneCountInString(k) > MaxLength {
			t.Errorf("MerchantKey(%q) = %q does not fit merchant_rules.merchant_key", in, k)
		}
	}
}
