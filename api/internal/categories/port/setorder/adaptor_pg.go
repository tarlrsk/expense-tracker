package setorder

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside a user transaction (ADR-0019).
func NewPG() Port { return pg{} }

// The ids go in as one uuid[] parameter (GORM would expand a Go slice into a list), and
// "with ordinality" numbers them from 1 in the order given.
const setOrderSQL = `
update categories c set sort_order = v.ord
from unnest(?::uuid[]) with ordinality as v(id, ord)
where c.owner_id = ? and c.id = v.id and c.sort_order <> v.ord`

func (pg) SetOrder(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) error {
	c, err := db.UserConn(ctx)
	if err != nil {
		return fmt.Errorf("set category order: %w", err)
	}
	if err := db.Err(c.Exec(setOrderSQL, uuidArray(ids), ownerID)); err != nil {
		return fmt.Errorf("set category order: %w", err)
	}
	return nil
}

// uuidArray renders ids as a Postgres array literal, {id,id,...}. A UUID's text has only hex
// digits and hyphens, so nothing needs quoting.
func uuidArray(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return "{" + strings.Join(parts, ",") + "}"
}
