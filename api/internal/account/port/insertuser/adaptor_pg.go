package insertuser

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tarlrsk/expense-tracker/api/internal/db"
)

type pg struct{}

// NewPG returns the Postgres adaptor; it runs inside an auth transaction (ADR-0034).
func NewPG() Port { return pg{} }

const insertSQL = `insert into users (email) values (?::citext) returning id, created_at`

type row struct {
	ID        uuid.UUID `gorm:"column:id"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (pg) Insert(ctx context.Context, email string) (NewUser, error) {
	c, err := db.AuthConn(ctx)
	if err != nil {
		return NewUser{}, fmt.Errorf("insert user: %w", err)
	}
	var r row
	if err := db.Err(c.Raw(insertSQL, email).Scan(&r)); err != nil {
		// users has one unique key besides the generated primary key: the email.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return NewUser{}, ErrEmailTaken
		}
		return NewUser{}, fmt.Errorf("insert user: %w", err)
	}
	return NewUser(r), nil
}
