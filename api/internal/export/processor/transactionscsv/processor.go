package transactionscsv

import (
	"context"
	"errors"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/authz"
	"github.com/tarlrsk/expense-tracker/api/internal/export/domain"
	exportlistport "github.com/tarlrsk/expense-tracker/api/internal/export/port/list"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries transactionscsv uses.
type Ports struct {
	List exportlistport.Port
}

type processor struct {
	user     tx.User
	pageSize int
	ports    Ports
}

// New returns the export use case; pageSize is the number of rows read per user transaction
// (domain.PageSize in the API).
func New(user tx.User, pageSize int, ports Ports) Processor {
	return &processor{user: user, pageSize: pageSize, ports: ports}
}

// errNotReadable: authz does not let the caller read the transactions of their own scope. Today
// it cannot happen (the scope is always the caller's own data); it is a fault, never a filter.
var errNotReadable = errors.New("export: the caller may not read the transactions of this scope")

// Execute checks the period, then reads the caller's rows page by page, each page in its own
// short user transaction (row-level security applies), and adds each page to a file built in
// memory, which is returned whole (ADR-0092). A failure on any page fails the request, so a
// client never receives a cut-off file. The pages are not one snapshot: a transaction added,
// deleted or moved to another date while the file is built may be in it or not.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	period, err := domain.ParsePeriod(req.From, req.To)
	if err != nil {
		return Response{}, err
	}
	scope := domain.OwnerScope(req.UserID)
	if !authz.CanReadTransaction(req.UserID, scope.OwnerID) {
		return Response{}, errNotReadable
	}
	file, err := domain.NewFile()
	if err != nil {
		return Response{}, fmt.Errorf("export: %w", err)
	}
	filter := exportlistport.Filter{From: period.From, To: period.To, Limit: p.pageSize}
	for {
		var rows []domain.Row
		err := p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
			var err error
			rows, err = p.ports.List.List(ctx, scope, filter)
			return err
		})
		if err != nil {
			return Response{}, fmt.Errorf("export: %w", err)
		}
		if err := file.Add(rows); err != nil {
			return Response{}, fmt.Errorf("export: %w", err)
		}
		if len(rows) < p.pageSize {
			break
		}
		after := domain.CursorAfter(rows[len(rows)-1])
		filter.After = &after
	}
	body, err := file.Bytes()
	if err != nil {
		return Response{}, fmt.Errorf("export: %w", err)
	}
	return Response{FileName: domain.FileName(period), Body: body}, nil
}
