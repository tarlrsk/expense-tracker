// Package registry holds the dependencies and route groups handed to each
// module's Register. It never imports registry/<module>.
package registry

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Deps are the shared dependencies built once in app.
//
// The auth transactor (tx.Auth) is deliberately not here: app hands it only to the account
// module's Register (ADR-0032, ADR-0034).
type Deps struct {
	Config config.Config
	Logger *slog.Logger
	// UserTx opens transactions as app_user (WithUserTx).
	UserTx tx.User
}

// Routes are the route groups a module registers its endpoints on.
type Routes struct {
	// Public is /api with no session check.
	// Authed and Operator are added in PLAN-0002 T5 together with the session middleware.
	Public gin.IRoutes
}
