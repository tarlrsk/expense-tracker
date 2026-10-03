package countattempts

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const countSQL = `
select
  (select count(*) from login_attempts where email = ?::citext and attempted_at > ?) as email,
  (select count(*) from login_attempts where ip = ?::inet and attempted_at > ?) as ip`

type row struct {
	Email int `gorm:"column:email"`
	IP    int `gorm:"column:ip"`
}

func (pg) Count(ctx context.Context, email string, ip netip.Addr, since time.Time) (Counts, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return Counts{}, fmt.Errorf("count login attempts: %w", err)
	}
	var r row
	if err := db.Err(c.Raw(countSQL, email, since, ip.String(), since).Scan(&r)); err != nil {
		return Counts{}, fmt.Errorf("count login attempts: %w", err)
	}
	return Counts(r), nil
}
