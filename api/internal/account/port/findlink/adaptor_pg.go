package findlink

import (
	"context"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const usableSQL = `
select exists (
  select from email_tokens t join users u on u.id = t.user_id
  where t.token_hash = ? and t.purpose = 'set_password' and t.used_at is null and t.expires_at > ?
    and u.disabled_at is null)`

func (pg) Usable(ctx context.Context, tokenHash []byte, now time.Time) (bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return false, fmt.Errorf("find link: %w", err)
	}
	var ok bool
	if err := db.Err(c.Raw(usableSQL, tokenHash, now).Scan(&ok)); err != nil {
		return false, fmt.Errorf("find link: %w", err)
	}
	return ok, nil
}
