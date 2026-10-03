package insertattempt

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const insertSQL = `insert into login_attempts (email, ip, attempted_at) values (?::citext, ?::inet, ?) returning id`

// idRow receives the returned id (GORM scans a uuid.UUID only as a struct field).
type idRow struct {
	ID uuid.UUID `gorm:"column:id"`
}

func (pg) Insert(ctx context.Context, email string, ip netip.Addr, at time.Time) (uuid.UUID, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert login attempt: %w", err)
	}
	var r idRow
	if err := db.Err(c.Raw(insertSQL, email, ip.String(), at).Scan(&r)); err != nil {
		return uuid.Nil, fmt.Errorf("insert login attempt: %w", err)
	}
	return r.ID, nil
}
