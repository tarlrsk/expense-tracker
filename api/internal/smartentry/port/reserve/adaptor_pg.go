package reserve

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
	transactionsdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// reserveSQL is one statement. The select gives no row when the limit is 0 or less, so nothing is
// inserted; a new day's row starts at 1; an existing row is counted up only while it is below the
// limit (the where of do update), otherwise nothing is written and no row is returned. A
// concurrent reservation of the same row waits for the first to commit, then sees its count.
const reserveSQL = `
insert into ai_usage (owner_id, day, parse_count)
select ?, ?::date, 1
where ?::integer > 0
on conflict (owner_id, day) do update set parse_count = ai_usage.parse_count + 1
where ai_usage.parse_count < ?::integer
returning parse_count`

type row struct {
	ParseCount int `gorm:"column:parse_count"`
}

func (pg) Reserve(ctx context.Context, ownerID uuid.UUID, day transactionsdomain.Date, limit int) (int, bool, error) {
	c, err := db.UserConn(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("reserve an AI parse: %w", err)
	}
	if day.IsZero() {
		return 0, false, errors.New("reserve an AI parse: no day")
	}
	var r row
	res := c.Raw(reserveSQL, ownerID, day.String(), limit, limit).Scan(&r)
	if err := db.Err(res); err != nil {
		return 0, false, fmt.Errorf("reserve an AI parse: %w", err)
	}
	if res.RowsAffected == 0 {
		return 0, false, nil
	}
	return r.ParseCount, true, nil
}
