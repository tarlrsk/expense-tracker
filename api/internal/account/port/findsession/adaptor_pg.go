package findsession

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const findSQL = `
select s.id, s.user_id, p.role, s.last_used_at
from sessions s
join users u on u.id = s.user_id
join profiles p on p.id = s.user_id
where s.token_hash = ? and s.expires_at > ? and u.disabled_at is null`

type row struct {
	ID         uuid.UUID `gorm:"column:id"`
	UserID     uuid.UUID `gorm:"column:user_id"`
	Role       string    `gorm:"column:role"`
	LastUsedAt time.Time `gorm:"column:last_used_at"`
}

func (pg) Find(ctx context.Context, tokenHash []byte, now time.Time) (Session, bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return Session{}, false, fmt.Errorf("find session: %w", err)
	}
	var r row
	res := c.Raw(findSQL, tokenHash, now).Scan(&r)
	if err := db.Err(res); err != nil {
		return Session{}, false, fmt.Errorf("find session: %w", err)
	}
	if res.RowsAffected == 0 {
		return Session{}, false, nil
	}
	return Session{ID: r.ID, UserID: r.UserID, Role: domain.Role(r.Role), LastUsedAt: r.LastUsedAt}, true, nil
}
