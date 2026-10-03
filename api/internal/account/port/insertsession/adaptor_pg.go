package insertsession

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const insertSQL = `
insert into sessions (user_id, token_hash, created_at, last_used_at, expires_at)
values (?, ?, ?, ?, ?)
returning id`

// idRow receives the returned id (GORM scans a uuid.UUID only as a struct field).
type idRow struct {
	ID uuid.UUID `gorm:"column:id"`
}

func (pg) Insert(ctx context.Context, s NewSession) (uuid.UUID, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert session: %w", err)
	}
	var r idRow
	if err := db.Err(c.Raw(insertSQL, s.UserID, s.TokenHash, s.Now, s.Now, s.ExpiresAt).Scan(&r)); err != nil {
		return uuid.Nil, fmt.Errorf("insert session: %w", err)
	}
	return r.ID, nil
}
