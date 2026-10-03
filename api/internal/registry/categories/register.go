// Package categories wires the categories module's use cases and routes. Every use case runs as
// app_user through deps.UserTx; the module never gets the auth transactor (ADR-0032, ADR-0034).
package categories

import (
	categorieshandler "github.com/tarlrsk/expense-tracker/api/internal/handler/categories"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

// Register adds the categories module's routes (ADR-0069). There is no delete: categories are
// archived.
func Register(deps registry.Deps, routes registry.Routes) {
	routes.Authed.GET("/categories", categorieshandler.List(NewList(deps)))
	routes.Authed.POST("/categories", categorieshandler.Create(NewCreate(deps)))
	routes.Authed.PATCH("/categories/:id", categorieshandler.Update(NewUpdate(deps)))
	routes.Authed.PUT("/categories/order", categorieshandler.Reorder(NewReorder(deps)))
}
