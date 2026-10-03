package create

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/authz"
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionsfindport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/find"
	transactionsinsertport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/insert"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries create uses. Category is the categories module's read port (ADR-0032
// lets a processor use read ports of earlier modules).
type Ports struct {
	Find     transactionsfindport.Port
	Insert   transactionsinsertport.Port
	Category categoriesfindport.Port
}

type processor struct {
	user  tx.User
	now   func() time.Time
	zone  *time.Location
	ports Ports
}

// New returns the create-transaction use case. now is the clock and zone the app time zone;
// together they say which day is today (ADR-0042).
func New(user tx.User, now func() time.Time, zone *time.Location, ports Ports) Processor {
	return &processor{user: user, now: now, zone: zone, ports: ports}
}

// Execute checks the input, then in one user transaction:
//  1. returns the caller's transaction with the id when there is one, whatever the other fields
//     say (a retry, ADR-0040); the category is not checked again, so a retry after the category
//     was archived still gets its transaction;
//  2. otherwise checks the category is one of the caller's non-archived ones (ADR-0071);
//  3. inserts. When the id turns out to be taken, a request of the caller that won a race
//     with this one has just committed it (it is returned like in 1), or another user has it:
//     row-level security hides that row, and the answer is the invalid_input of a malformed id,
//     with nothing that tells the two apart (ADR-0040).
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	nt, err := p.check(req)
	if err != nil {
		return Response{}, err
	}

	var (
		resp       Response
		idTaken    bool
		noCategory bool
	)
	err = p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		existing, found, err := p.ports.Find.Find(ctx, req.UserID, nt.ID)
		if err != nil {
			return err
		}
		if found {
			resp.Transaction, idTaken = existing, !authz.CanReadTransaction(req.UserID, existing.OwnerID)
			return nil
		}
		cat, found, err := p.ports.Category.Find(ctx, req.UserID, nt.CategoryID)
		if err != nil {
			return err
		}
		if !found || cat.Archived {
			noCategory = true
			return nil
		}
		resp.Transaction, resp.Created, err = p.ports.Insert.Insert(ctx, nt)
		if err != nil || resp.Created {
			return err
		}
		// The id is taken: by a create of the caller that committed meanwhile, or by another
		// user. A new statement sees the committed row of the caller; another user's stays hidden.
		existing, found, err = p.ports.Find.Find(ctx, req.UserID, nt.ID)
		if err != nil {
			return err
		}
		resp.Transaction, idTaken = existing, !found || !authz.CanReadTransaction(req.UserID, existing.OwnerID)
		return nil
	})
	switch {
	case errors.Is(err, transactionsinsertport.ErrCategory):
		return Response{}, apperr.Wrap(apperr.InvalidInput, domain.CategoryMessage, err)
	case err != nil:
		return Response{}, fmt.Errorf("create transaction: %w", err)
	case noCategory:
		return Response{}, apperr.New(apperr.InvalidInput, domain.CategoryMessage)
	case idTaken:
		return Response{}, apperr.New(apperr.InvalidInput, domain.IDRuleMessage)
	}
	return resp, nil
}

// check applies the rules to the request and returns the transaction to insert for the caller.
func (p *processor) check(req Request) (transactionsinsertport.NewTransaction, error) {
	nt := transactionsinsertport.NewTransaction{OwnerID: req.UserID, Currency: domain.CurrencyTHB, Source: domain.SourceManual}
	var err error
	if nt.ID, err = domain.ParseNewID(req.ID); err != nil {
		return nt, err
	}
	if nt.Amount, err = domain.ParseAmount(req.Amount); err != nil {
		return nt, err
	}
	if nt.OccurredOn, err = domain.ParseOccurredOn(req.OccurredOn, domain.Today(p.now(), p.zone)); err != nil {
		return nt, err
	}
	if nt.CategoryID, err = domain.ParseCategoryID(req.CategoryID); err != nil {
		return nt, err
	}
	if nt.Merchant, err = domain.NormalizeMerchant(req.Merchant); err != nil {
		return nt, err
	}
	if nt.Note, err = domain.NormalizeNote(req.Note); err != nil {
		return nt, err
	}
	if req.Currency != nil {
		if nt.Currency, err = domain.ParseCurrency(*req.Currency); err != nil {
			return nt, err
		}
	}
	if req.Source != nil {
		if nt.Source, err = domain.ParseCreateSource(*req.Source); err != nil {
			return nt, err
		}
	}
	return nt, nil
}
