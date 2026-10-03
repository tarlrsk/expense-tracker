package updatepassword

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const (
	updateSQL   = `update users set password_hash = ? where id = ?`
	updateIfSQL = `update users set password_hash = ? where id = ? and password_hash = ?`
)

func (pg) Update(ctx context.Context, u Update) (bool, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return false, fmt.Errorf("update password: %w", err)
	}
	var res *gorm.DB
	if u.IfHash != nil {
		res = c.Exec(updateIfSQL, u.Hash, u.UserID, *u.IfHash)
	} else {
		res = c.Exec(updateSQL, u.Hash, u.UserID)
	}
	if err := db.Err(res); err != nil {
		return false, fmt.Errorf("update password: %w", err)
	}
	return res.RowsAffected > 0, nil
}
