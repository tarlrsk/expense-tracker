package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountchecksessionproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/checksession"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/handler/httpx"
)

// Session checks the bearer token (Authorization: Bearer <token>) on every request of its route
// group and records the caller for handlers (httpx.CallerOf). A missing, malformed, unknown or
// expired token, or a disabled user, is 401 unauthenticated with one message (ADR-0025,
// ADR-0066).
//
// check is built by registry/account.NewCheckSession (ADR-0032). It runs on the request's own
// context and returns plain values; the context of its auth transaction is never kept.
func Session(check accountchecksessionproc.Processor) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.Request.Header.Values("Authorization"))
		if !ok {
			httpx.WriteError(c, apperr.New(apperr.Unauthenticated, domain.UnauthenticatedMessage))
			return
		}
		resp, err := check.Execute(c.Request.Context(), accountchecksessionproc.Request{Token: token})
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		httpx.SetCaller(c, httpx.Caller{UserID: resp.UserID, SessionID: resp.SessionID, Role: string(resp.Role)})
		c.Next()
	}
}

// RequireOperator lets only operators through; it runs after Session on the operator group
// (/api/admin). Anyone else logged in gets 403 forbidden (ADR-0019, ADR-0056).
func RequireOperator() gin.HandlerFunc {
	return func(c *gin.Context) {
		caller, ok := httpx.CallerOf(c)
		if !ok {
			httpx.WriteError(c, errors.New("middleware: RequireOperator without Session before it"))
			return
		}
		if caller.Role != string(domain.RoleOperator) {
			httpx.WriteError(c, apperr.New(apperr.Forbidden, "only an operator may do this"))
			return
		}
		c.Next()
	}
}

// bearerToken returns the token of the one Authorization header "Bearer <token>" (the scheme in
// any case, RFC 9110). Several headers, another scheme, or an empty or spaced token is not ok.
func bearerToken(values []string) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	scheme, token, ok := strings.Cut(values[0], " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t") {
		return "", false
	}
	return token, true
}
