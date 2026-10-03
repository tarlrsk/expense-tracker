package app

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/middleware"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	accountreg "github.com/tarlrsk/expense-tracker/api/internal/registry/account"
	categoriesreg "github.com/tarlrsk/expense-tracker/api/internal/registry/categories"
	healthreg "github.com/tarlrsk/expense-tracker/api/internal/registry/health"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// NewEngine builds the gin engine: middleware, route groups and every module's routes. auth is
// the auth transactor; it goes only to the session check and the account module (ADR-0032,
// ADR-0034).
func NewEngine(deps registry.Deps, auth tx.Auth) (*gin.Engine, error) {
	return newEngine(deps, auth)
}

// newEngine is NewEngine; tests pass extra registrations (a test-only operator route, for
// example) that run after every module's.
func newEngine(deps registry.Deps, auth tx.Auth, extra ...func(registry.Routes)) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	engine.Use(
		middleware.RequestLog(deps.Logger),
		middleware.Recover(deps.Logger),
		middleware.NoStore(),
		middleware.Deadline(deps.Config.RequestTimeout),
	)
	// A wrong method on a known path is a 404 too (HandleMethodNotAllowed stays off).
	engine.NoRoute(func(c *gin.Context) {
		httpx.WriteError(c, apperr.New(apperr.NotFound, "not found"))
	})

	// The session check runs on the request's own context; each authed request opens one auth
	// transaction for it, then the handler's own (ADR-0032).
	session := middleware.Session(accountreg.NewCheckSession(deps, auth))
	routes := registry.Routes{
		Public:   engine.Group("/api"),
		Authed:   engine.Group("/api", session),
		Operator: engine.Group("/api/admin", session, middleware.RequireOperator()),
	}
	healthreg.Register(deps, routes)
	accountreg.Register(deps, routes, auth)
	categoriesreg.Register(deps, routes)
	for _, register := range extra {
		register(routes)
	}

	return engine, nil
}
