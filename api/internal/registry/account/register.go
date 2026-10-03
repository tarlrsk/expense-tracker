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
	routes.Authed.GET("/me", accounthandler.GetMe(NewGetProfile(deps, auth)))
	routes.Authed.PATCH("/me", accounthandler.UpdateMe(NewUpdateProfile(deps, auth)))
	routes.Authed.DELETE("/me", accounthandler.DeleteMe(NewDeleteMe(deps, auth)))

	// /api/admin: operators only (ADR-0019, ADR-0068).
	routes.Operator.GET("/users", accounthandler.ListUsers(NewListUsers(deps, auth)))
	routes.Operator.POST("/invites", accounthandler.Invite(NewInvite(deps, auth)))
	routes.Operator.POST("/users/:id/set-password-link", accounthandler.SendLink(NewSendLink(deps, auth)))
	routes.Operator.DELETE("/users/:id", accounthandler.RemoveUser(NewRemoveAccount(deps, auth)))
}
