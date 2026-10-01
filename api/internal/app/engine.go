package app

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
	"github.com/tarlrsk/expense-tracker/api/internal/middleware"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	healthreg "github.com/tarlrsk/expense-tracker/api/internal/registry/health"
)

// NewEngine builds the gin engine: middleware, route groups and every module's routes.
func NewEngine(deps registry.Deps) (*gin.Engine, error) {
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

	routes := registry.Routes{
		Public: engine.Group("/api"),
	}
	healthreg.Register(deps, routes)

	return engine, nil
}
