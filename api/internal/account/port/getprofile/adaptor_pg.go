package getprofile

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

const getSQL = `
select u.id, u.email::text as email, p.display_name, p.role, u.created_at
from users u
join profiles p on p.id = u.id
where u.id = ?`

type row struct {
	ID          uuid.UUID `gorm:"column:id"`
	Email       string    `gorm:"column:email"`
	DisplayName string    `gorm:"column:display_name"`
	Role        string    `gorm:"column:role"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (pg) Get(ctx context.Context, userID uuid.UUID) (Profile, bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return Profile{}, false, fmt.Errorf("get profile: %w", err)
	}
	var r row
	res := c.Raw(getSQL, userID).Scan(&r)
	if err := db.Err(res); err != nil {
		return Profile{}, false, fmt.Errorf("get profile: %w", err)
	}
	if res.RowsAffected == 0 {
		return Profile{}, false, nil
	}
	return Profile{ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Role: domain.Role(r.Role), CreatedAt: r.CreatedAt}, true, nil
}
