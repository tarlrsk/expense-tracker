package listusers

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

// The password hash is only compared, never selected. A user without a profile (which the one
// account-creation path never leaves behind) is still listed, with the defaults.
const listSQL = `
select u.id, u.email::text as email, coalesce(p.display_name, '') as display_name,
  coalesce(p.role, ?) as role, u.password_hash <> '' as has_password, u.created_at,
  (select max(s.last_used_at) from sessions s where s.user_id = u.id) as last_active_at
from users u
left join profiles p on p.id = u.id
order by u.created_at, u.id`

type row struct {
	ID           uuid.UUID  `gorm:"column:id"`
	Email        string     `gorm:"column:email"`
	DisplayName  string     `gorm:"column:display_name"`
	Role         string     `gorm:"column:role"`
	HasPassword  bool       `gorm:"column:has_password"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	LastActiveAt *time.Time `gorm:"column:last_active_at"`
}

func (pg) List(ctx context.Context) ([]domain.Account, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	var rows []row
	if err := db.Err(c.Raw(listSQL, string(domain.RoleUser)).Scan(&rows)); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := make([]domain.Account, 0, len(rows))
	for _, r := range rows {
		users = append(users, domain.Account{
			ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Role: domain.Role(r.Role),
			Status: domain.StatusOf(r.HasPassword), CreatedAt: r.CreatedAt, LastActiveAt: r.LastActiveAt,
		})
	}
	return users, nil
}
