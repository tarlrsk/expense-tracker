package update

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/authz"
	categoriesfindport "github.com/tarlrsk/expense-tracker/api/internal/categories/port/find"
	"github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
	transactionslockport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/lock"
	transactionsupdateport "github.com/tarlrsk/expense-tracker/api/internal/transactions/port/update"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Ports are the queries update uses. Category is the categories module's read port (ADR-0032
// lets a processor use read ports of earlier modules).
type Ports struct {
	Lock     transactionslockport.Port
	Update   transactionsupdateport.Port
	Category categoriesfindport.Port
}

type processor struct {
	user  tx.User
	now   func() time.Time
	zone  *time.Location
	ports Ports
}

// New returns the update-transaction use case. now is the clock and zone the app time zone;
// together they say which day is today (ADR-0042).
func New(user tx.User, now func() time.Time, zone *time.Location, ports Ports) Processor {
	return &processor{user: user, now: now, zone: zone, ports: ports}
}

// Execute checks the input, then in one user transaction locks the transaction, asks authz
// whether the caller may change it, and writes only what differs from it:
//   - a value equal to the stored one is not a change, so a PATCH that changes nothing writes
//     nothing and returns the transaction;
//   - a new category must be one of the caller's non-archived ones (ADR-0071); keeping an
//     archived category the transaction already has is not a change and is allowed.
//
// Another user's id is not found, like an unknown one: row-level security hides the row.
func (p *processor) Execute(ctx context.Context, req Request) (Response, error) {
	want, err := p.check(req)
	if err != nil {
		return Response{}, err
	}

	var (
		t          domain.Transaction
		found      bool
		noCategory bool
	)
	err = p.user.WithUserTx(ctx, req.UserID, func(ctx context.Context) error {
		var err error
		t, found, err = p.ports.Lock.Lock(ctx, req.UserID, req.ID)
		if err != nil || !found {
			return err
		}
		if !authz.CanChangeTransaction(req.UserID, t.OwnerID) {
			found = false
			return nil
		}
		ch, changed := changes(t, want)
		if !changed {
			return nil
		}
		if ch.CategoryID != nil {
			cat, ok, err := p.ports.Category.Find(ctx, req.UserID, *ch.CategoryID)
			if err != nil {
				return err
			}
			if !ok || cat.Archived {
				noCategory = true
				return nil
			}
		}
		t, found, err = p.ports.Update.Update(ctx, req.UserID, req.ID, ch)
		return err
	})
	switch {
	case errors.Is(err, transactionsupdateport.ErrCategory):
		return Response{}, apperr.Wrap(apperr.InvalidInput, domain.CategoryMessage, err)
	case err != nil:
		return Response{}, fmt.Errorf("update transaction: %w", err)
	case !found:
		return Response{}, apperr.New(apperr.NotFound, domain.NotFoundMessage)
	case noCategory:
		return Response{}, apperr.New(apperr.InvalidInput, domain.CategoryMessage)
	}
	return Response{Transaction: t}, nil
}

// check applies the rules to every field that is set and returns them as values to compare and
// write.
func (p *processor) check(req Request) (transactionsupdateport.Changes, error) {
	var ch transactionsupdateport.Changes
	if req.Amount == nil && req.OccurredOn == nil && req.CategoryID == nil && req.Merchant == nil &&
		req.Note == nil && req.Currency == nil {
		return ch, apperr.New(apperr.InvalidInput, domain.NoChangeMessage)
	}
	if req.Amount != nil {
		a, err := domain.ParseAmount(*req.Amount)
		if err != nil {
			return ch, err
		}
		ch.Amount = &a
	}
	if req.OccurredOn != nil {
		d, err := domain.ParseOccurredOn(*req.OccurredOn, domain.Today(p.now(), p.zone))
		if err != nil {
			return ch, err
		}
		ch.OccurredOn = &d
	}
	if req.CategoryID != nil {
		id, err := domain.ParseCategoryID(*req.CategoryID)
		if err != nil {
			return ch, err
		}
		ch.CategoryID = &id
	}
	if req.Merchant != nil {
		m, err := domain.NormalizeMerchant(*req.Merchant)
		if err != nil {
			return ch, err
		}
		ch.Merchant = &m
	}
	if req.Note != nil {
		n, err := domain.NormalizeNote(*req.Note)
		if err != nil {
			return ch, err
		}
		ch.Note = &n
	}
	if req.Currency != nil {
		c, err := domain.ParseCurrency(*req.Currency)
		if err != nil {
			return ch, err
		}
		ch.Currency = &c
	}
	return ch, nil
}

// changes keeps the fields of want that differ from cur, and says whether any does.
func changes(cur domain.Transaction, want transactionsupdateport.Changes) (transactionsupdateport.Changes, bool) {
	var ch transactionsupdateport.Changes
	if want.Amount != nil && *want.Amount != cur.Amount {
		ch.Amount = want.Amount
	}
	if want.Currency != nil && *want.Currency != cur.Currency {
		ch.Currency = want.Currency
	}
	if want.OccurredOn != nil && want.OccurredOn.Compare(cur.OccurredOn) != 0 {
		ch.OccurredOn = want.OccurredOn
	}
	if want.Merchant != nil && *want.Merchant != cur.Merchant {
		ch.Merchant = want.Merchant
	}
	if want.CategoryID != nil && *want.CategoryID != cur.CategoryID {
		ch.CategoryID = want.CategoryID
	}
	if want.Note != nil && *want.Note != cur.Note {
		ch.Note = want.Note
	}
	changed := ch.Amount != nil || ch.Currency != nil || ch.OccurredOn != nil || ch.Merchant != nil ||
		ch.CategoryID != nil || ch.Note != nil
	return ch, changed
}
