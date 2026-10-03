package lock

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// The rows are locked in id order, so two lockers of one user never deadlock. Statements after
// the lock read a fresh snapshot (read committed), so they see what a writer that held the lock
// before committed.
const lockSQL = `select id from categories where owner_id = ? order by id for update`

func (pg) Lock(ctx context.Context, ownerID uuid.UUID) error {
	c, err := db.UserConn(ctx)
	if err != nil {
		return fmt.Errorf("lock categories: %w", err)
	}
	if err := db.Err(c.Exec(lockSQL, ownerID)); err != nil {
		return fmt.Errorf("lock categories: %w", err)
	}
	return nil
}
