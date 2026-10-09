// Package domain holds the export module's values (M8, ADR-0087, ADR-0092): whose data an export
// covers, its period and file name, the exported row and the CSV file.
package domain

import "github.com/google/uuid"

// Scope is whose data an export covers. The export query takes one and has it as its one
// ownership condition (the groups guardrail of docs/05-roadmap.md: "Dashboard/export SQL is
// written as WHERE <scope>"). Today a scope is always one user's own data; a group scope would be
// added here later, by a new ADR. Nothing for groups is built yet (ADR-0014).
type Scope struct {
	// OwnerID is the user whose transactions are exported; today always the caller.
	OwnerID uuid.UUID
}

// OwnerScope is the scope of one user's own data.
func OwnerScope(ownerID uuid.UUID) Scope { return Scope{OwnerID: ownerID} }
