package list

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/authz"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionslistport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/list"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries list uses.
type Ports struct {
	List transactionslistport.Port
}

type processor struct {
	user  tx.User
	ports Ports
}

// New returns the list-transactions use case.
func New(user tx.User, ports Ports) Processor {
	return &processor{user: user, ports: ports}
}

// errNotReadable: the list query returned a row authz does not let the caller read. Row-level
// security makes it impossible; it is a fault, never a filter.
var errNotReadable = errors.New("list transactions: a row the caller may not read was returned")

// Execute checks the parameters, then reads one row more than the page size in a user
// transaction for the caller (row-level security applies): the extra row only says that a next
// page exists. A category that is not the caller's simply matches nothing.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	f, err := filter(req)
	if err != nil {
		return Response{}, err
	}
	limit := f.Limit
	f.Limit++

	var rows []domain.Transaction
	err = p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		var err error
		rows, err = p.ports.List.List(ctx, req.UserID, f)
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("list transactions: %w", err)
	}
	for _, t := range rows {
		if !authz.CanReadTransaction(req.UserID, t.OwnerID) {
			return Response{}, errNotReadable
		}
	}
	resp := Response{Transactions: rows}
	if len(rows) > limit {
		resp.Transactions = rows[:limit]
		next := domain.CursorAfter(rows[limit-1])
		resp.Next = &next
	}
	return resp, nil
}

// filter applies the rules to the parameters.
func filter(req Request) (transactionslistport.Filter, error) {
	var f transactionslistport.Filter
	period, err := domain.ParsePeriod(req.Month, req.From, req.To)
	if err != nil {
		return f, err
	}
	f.From, f.To = period.From, period.To
	if req.Category != nil {
		id, err := domain.ParseCategoryFilter(*req.Category)
		if err != nil {
			return f, err
		}
		f.CategoryID = &id
	}
	if req.Source != nil {
		s, err := domain.ParseSourceFilter(*req.Source)
		if err != nil {
			return f, err
		}
		f.Source = &s
	}
	if f.Limit, err = domain.ParseLimit(req.Limit); err != nil {
		return f, err
	}
	if req.Cursor != nil {
		c, err := domain.ParseCursor(*req.Cursor)
		if err != nil {
			return f, err
		}
		f.After = &c
	}
	return f, nil
}
