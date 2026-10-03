package insertlink

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const insertSQL = `
insert into email_tokens (user_id, purpose, token_hash, expires_at, created_at)
values (?, 'set_password', ?, ?, ?)`

func (pg) Insert(ctx context.Context, l NewLink) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}
	if err := db.Err(c.Exec(insertSQL, l.UserID, l.TokenHash, l.ExpiresAt, l.Now)); err != nil {
		return fmt.Errorf("insert link: %w", err)
	}
	return nil
}
