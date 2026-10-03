package list

import (
	"context"
	"fmt"

	categorieslistport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/list"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries list uses.
type Ports struct {
	List categorieslistport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the list-categories use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// Execute reads the list in a user transaction for the caller, so row-level security applies.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	var resp Response
	err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		var err error
		resp.Categories, err = p.ports.List.List(ctx, req.UserID)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("list categories: %w", err)
	}
	return resp, nil
}
