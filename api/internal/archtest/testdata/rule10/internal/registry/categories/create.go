package categories

import (
	"example.com/fx/internal/registry"
	"example.com/fx/internal/tx"
)

// Violation: a type assertion from the user transactor.
func NewCreate(deps registry.Deps) { _, _ = deps.UserTx.(tx.Auth) }
