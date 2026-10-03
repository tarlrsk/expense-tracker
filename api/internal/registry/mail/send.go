// Package mail wires the outside mail service (ADR-0028). app calls it once and puts the result
// in registry.Deps.
package mail

import (
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
)

// NewSend builds the SMTP mailer from the SMTP_* settings.
func NewSend(cfg config.Config) (send.Port, error) {
	p, err := send.NewSMTP(cfg.SMTP)
	if err != nil {
		return nil, fmt.Errorf("mailer: %w", err)
	}
	return p, nil
}
