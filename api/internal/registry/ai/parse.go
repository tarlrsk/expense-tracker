// Package ai wires the outside AI service (ADR-0009). app calls it once and puts the result in
// registry.Deps.
package ai

import (
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
)

// NewParse builds the Anthropic parser from the ANTHROPIC_* and AI_TIMEOUT settings. Without a
// key it still succeeds; the parser then answers parse.ErrNotConfigured.
func NewParse(cfg config.Config) (parse.Port, error) {
	p, err := parse.NewAnthropic(cfg.AI)
	if err != nil {
		return nil, fmt.Errorf("ai parser: %w", err)
	}
	return p, nil
}
