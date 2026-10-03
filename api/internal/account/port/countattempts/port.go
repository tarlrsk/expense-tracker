// Package countattempts is the query that counts recent failed logins for an email and an IP.
package countattempts

import (
	"context"
	"net/netip"
	"time"
)

// Counts are the failed attempts since a point in time.
type Counts struct {
	// Email: rows for the email (compared without case), from any address.
	Email int
	// IP: rows from the address, for any email.
	IP int
}

// Port counts failed logins.
type Port interface {
	// Count counts the login_attempts rows after since.
	Count(ctx context.Context, email string, ip netip.Addr, since time.Time) (Counts, error)
}
