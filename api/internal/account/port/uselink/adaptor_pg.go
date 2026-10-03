package uselink

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const useSQL = `
update email_tokens t set used_at = ?
from users u
where t.token_hash = ? and t.purpose = 'set_password' and t.used_at is null and t.expires_at > ?
  and u.id = t.user_id and u.disabled_at is null
returning t.user_id`

type row struct {
	UserID uuid.UUID `gorm:"column:user_id"`
}

func (pg) Use(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("use link: %w", err)
	}
	var r row
	res := c.Raw(useSQL, now, tokenHash, now).Scan(&r)
	if err := db.Err(res); err != nil {
		return uuid.Nil, false, fmt.Errorf("use link: %w", err)
	}
	return r.UserID, res.RowsAffected > 0, nil
}
