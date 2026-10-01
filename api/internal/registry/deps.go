// Package registry holds the dependencies and route groups handed to each
// module's Register. It never imports registry/<module>.
package registry

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
)

// Deps are the shared dependencies built once in app.
type Deps struct {
	Config config.Config
	Logger *slog.Logger
}

// Routes are the route groups a module registers its endpoints on.
type Routes struct {
	// Public is /api with no session check.
	// Authed and Operator are added in PLAN-0002 T5 together with the session middleware.
	Public gin.IRoutes
}
