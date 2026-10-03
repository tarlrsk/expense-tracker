// Package account holds the gin handlers of the account module, one file per endpoint.
package account

import "time"

// sessionResponse is the body of a login and of a set-password: the bearer token, shown once,
// and when the session ends unless it is used again.
type sessionResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func newSessionResponse(token string, expiresAt time.Time) sessionResponse {
	return sessionResponse{Token: token, ExpiresAt: expiresAt.UTC()}
}
