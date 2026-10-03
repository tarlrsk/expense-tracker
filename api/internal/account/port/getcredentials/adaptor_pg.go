package getcredentials

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

type row struct {
	Email        string `gorm:"column:email"`
	PasswordHash string `gorm:"column:password_hash"`
}

func (pg) Get(ctx context.Context, userID uuid.UUID) (Credentials, bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return Credentials{}, false, fmt.Errorf("get credentials: %w", err)
	}
	var r row
	res := c.Raw(`select email::text as email, password_hash from users where id = ?`, userID).Scan(&r)
	if err := db.Err(res); err != nil {
		return Credentials{}, false, fmt.Errorf("get credentials: %w", err)
	}
	if res.RowsAffected == 0 {
		return Credentials{}, false, nil
	}
	return Credentials(r), true, nil
}
