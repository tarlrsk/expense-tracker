package domain

import (
	"github.com/google/uuid"

	txdomain "github.com/tarlrsk/expense-tracker/api/internal/transactions/domain"
)

// PageSize is how many rows the export reads per page, each page in its own user transaction,
// so no single statement or transaction grows with the history (ADR-0043, ADR-0092).
const PageSize = 1000

// Row is one exported transaction: the seven columns of the file (ADR-0087) and its id, which
// only orders the rows and is not written.
type Row struct {
	ID         uuid.UUID
	OccurredOn txdomain.Date
	// Amount is decimal text with two decimals, as stored, such as "145.00"; always above zero.
	Amount string
	// Kind is the category's kind: expense or income.
	Kind string
	// Category is the category's current name, also when it is archived.
	Category string
	// Description is the transaction's merchant column.
	Description string
	Note        string
	// Source is how the transaction was added: manual, text, scan or csv.
	Source string
}

// Cursor is the position after a row in the export's order: occurred_on, then id, both ascending.
type Cursor struct {
	OccurredOn txdomain.Date
	ID         uuid.UUID
}

// CursorAfter is the position just after r.
func CursorAfter(r Row) Cursor { return Cursor{OccurredOn: r.OccurredOn, ID: r.ID} }
