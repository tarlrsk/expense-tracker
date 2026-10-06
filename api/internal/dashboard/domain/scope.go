// Package domain holds the dashboard module's values (M6, ADR-0087, ADR-0088): whose data an
// aggregation covers, the calendar month it covers and the rule for the month parameter, money
// totals and the results of the dashboard queries.
package domain

import "github.com/google/uuid"

// Scope is whose data an aggregation covers. Every dashboard query takes one and has it as its
// one ownership condition (the groups guardrail of docs/05-roadmap.md: "Aggregations take a scope
// parameter"). Today a scope is always one user's own data; a group scope would be added here
// later, by a new ADR, and change only that one condition in each query. Nothing for groups is
// built yet (ADR-0014).
type Scope struct {
	// OwnerID is the user whose data is aggregated; today always the caller.
	OwnerID uuid.UUID
}

// OwnerScope is the scope of one user's own data.
func OwnerScope(ownerID uuid.UUID) Scope { return Scope{OwnerID: ownerID} }
