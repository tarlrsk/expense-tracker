package domain

import "github.com/tarlrsk/expense-tracker/api/internal/apperr"

// DescriptionStyle says how freely the AI may write the description of an entry it reads; the
// user chooses it in Settings and it is stored on the profile (ADR-0081, PLAN-0003 T7).
type DescriptionStyle string

// The description styles; the database allows only these (migration 0007).
const (
	// DescriptionStyleAsTyped keeps the user's own words.
	DescriptionStyleAsTyped DescriptionStyle = "as_typed"
	// DescriptionStyleLightTidy also fixes capital letters and obvious typos, without translating.
	DescriptionStyleLightTidy DescriptionStyle = "light_tidy"
	// DescriptionStyleCleanName lets the AI rewrite the description as a clean merchant name.
	DescriptionStyleCleanName DescriptionStyle = "clean_name"
)

// DefaultDescriptionStyle is the style of a new profile, and of one that cannot be read.
const DefaultDescriptionStyle = DescriptionStyleAsTyped

// DescriptionStyleRuleMessage: PATCH /api/me with a description style that is not one of the three.
const DescriptionStyleRuleMessage = "the description style must be as_typed, light_tidy or clean_name"

// ParseDescriptionStyle returns style as a DescriptionStyle, or an invalid_input error when it is
// not exactly one of the three (an empty one included).
func ParseDescriptionStyle(style string) (DescriptionStyle, error) {
	switch s := DescriptionStyle(style); s {
	case DescriptionStyleAsTyped, DescriptionStyleLightTidy, DescriptionStyleCleanName:
		return s, nil
	default:
		return "", apperr.New(apperr.InvalidInput, DescriptionStyleRuleMessage)
	}
}
