// Package authz is the single Go place that decides whether a caller may read or change a row
// of user data, given the row's owner (docs/05-roadmap.md "Groups / shared spending"
// guardrails, ADR-0071). Processors call it; no other layer compares user ids.
//
// Row-level security stays the real enforcement (ADR-0014, ADR-0019): every query runs inside
// WithUserTx, so a row of another user never even reaches Go. These checks are the second
// place, the one a later group rule would change. Today a caller may read and change only
// their own rows.
package authz

import "github.com/google/uuid"

// CanReadTransaction reports whether caller may see a transaction owned by owner.
func CanReadTransaction(caller, owner uuid.UUID) bool {
	return isOwner(caller, owner)
}

// CanChangeTransaction reports whether caller may update or delete a transaction owned by owner.
func CanChangeTransaction(caller, owner uuid.UUID) bool {
	return isOwner(caller, owner)
}

// isOwner is the one comparison of user ids. The nil id is nobody: it never matches.
func isOwner(caller, owner uuid.UUID) bool {
	return caller != uuid.Nil && caller == owner
}
