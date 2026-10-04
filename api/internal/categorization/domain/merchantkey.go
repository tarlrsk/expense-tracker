package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// MerchantKey returns the key that merchant rules are stored and looked up by (ADR-0078): the
// merchant in Unicode NFKC, lowercased, keeping only letters, decimal digits and combining marks
// (Thai vowels and tone marks are marks). Spaces, hyphens, punctuation and symbols are dropped,
// so "7-Eleven", "7 eleven" and "7ELEVEN" all give "7eleven". There is no alias table: Thai and
// English names of one merchant have different keys.
//
// It returns "" when the merchant has no key: nothing is kept, or the key is longer than
// MaxLength characters (NFKC can lengthen text). Nothing is learnt or looked up for "".
//
// Stored keys never change, so changing these rules later means re-keying the stored rules.
func MerchantKey(merchant string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(norm.NFKC.String(merchant)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) {
			b.WriteRune(r)
		}
	}
	key := b.String()
	if utf8.RuneCountInString(key) > MaxLength {
		return ""
	}
	return key
}
