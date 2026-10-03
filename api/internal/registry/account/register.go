// Package account wires the account module's use cases and routes. It is the only module
// registration that receives the auth transactor (ADR-0032, ADR-0034).
package account

import (
	accounthandler "github.com/tarlrsk/expense-tracker/api/internal/handler/account"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Register adds the account module's routes.
func Register(deps registry.Deps, routes registry.Routes, auth tx.Auth) {
	routes.Public.POST("/auth/set-password", accounthandler.SetPassword(NewSetPassword(deps, auth)))
	routes.Public.POST("/auth/login", accounthandler.Login(NewLogin(deps, auth)))
	routes.Authed.POST("/auth/logout", accounthandler.Logout(NewLogout(deps, auth)))
	routes.Authed.POST("/me/password", accounthandler.ChangePassword(NewChangePassword(deps, auth)))
}
