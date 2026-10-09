// Package transactionscsv builds the caller's transactions as a CSV file, for all time or a
// period (GET /api/export.csv, M8, ADR-0087, ADR-0092).
package transactionscsv

import (
	"context"

	"github.com/google/uuid"
)

// Processor builds the export file.
type Processor interface {
	// Execute returns the file of the caller's transactions with occurred_on in the period, oldest
	// first; invalid_input for a malformed from or to, or from after to.
	Execute(ctx context.Context, req Request) (Response, error)
}

// Request is the caller and the period parameters as sent; a nil From or To was not sent.
type Request struct {
	UserID   uuid.UUID
	From, To *string
}

// Response is the file's name and its whole content.
type Response struct {
	FileName string
	Body     []byte
}
