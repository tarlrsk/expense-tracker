package insertprofile

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

func (pg) Insert(ctx context.Context, userID uuid.UUID, role domain.Role) error {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return fmt.Errorf("insert profile: %w", err)
	}
	if err := db.Err(c.Exec(`insert into profiles (id, role) values (?, ?)`, userID, string(role))); err != nil {
		return fmt.Errorf("insert profile: %w", err)
	}
	return nil
}
