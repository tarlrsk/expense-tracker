package findcredentials

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const findSQL = `select id, password_hash, disabled_at is not null as disabled from users where email = ?::citext`

type row struct {
	ID           uuid.UUID `gorm:"column:id"`
	PasswordHash string    `gorm:"column:password_hash"`
	Disabled     bool      `gorm:"column:disabled"`
}

func (pg) Find(ctx context.Context, email string) (Credentials, bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("find credentials: %w", err)
	}
	var r row
	res := c.Raw(findSQL, email).Scan(&r)
	if err := db.Err(res); err != nil {
		return Credentials{}, false, fmt.Errorf("find credentials: %w", err)
	}
	if res.RowsAffected == 0 {
		return Credentials{}, false, nil
	}
	return Credentials{UserID: r.ID, PasswordHash: r.PasswordHash, Disabled: r.Disabled}, true, nil
}
