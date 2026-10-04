// Package registry holds the dependencies and route groups handed to each
// module's Register. It never imports registry/<module>.
package registry

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/external/ai/parse"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// Deps are the shared dependencies built once in app.
//
// The auth transactor (tx.Auth) is deliberately not here: app hands it only to the account
// module's Register and to the session-check wiring (ADR-0032, ADR-0034).
type Deps struct {
	Config config.Config
	Logger *slog.Logger
	// UserTx opens transactions as app_user (WithUserTx). It is not a tx.Auth: a type
	// assertion to tx.Auth fails (db.DB.UserOnly).
	UserTx tx.User
	// Clock returns the current time; use cases read it instead of time.Now so expiry rules
	// can be tested. SQL defaults may still use now().
	Clock func() time.Time
	// Mailer sends email (ADR-0028); built by registry/mail.NewSend.
	Mailer send.Port
	// AIParse reads quick entry text with the AI (ADR-0009); built by registry/ai.NewParse.
	AIParse parse.Port
}

// Routes are the route groups a module registers its endpoints on.
type Routes struct {
	// Public is /api with no session check.
	Public gin.IRoutes
	// Authed is /api behind the session check; handlers read the caller with httpx.CallerOf.
	Authed gin.IRoutes
	// Operator is /api/admin behind the session check and the operator role check (403
	// otherwise).
	Operator gin.IRoutes
}
