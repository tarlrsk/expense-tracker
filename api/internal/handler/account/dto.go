// Package account holds the gin handlers of the account module, one file per endpoint.
package account

import (
	"time"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountgetprofileport "github.com/tarlrsk/expense-tracker/api/internal/account/port/getprofile"
)

// sessionResponse is the body of a login and of a set-password: the bearer token, shown once,
// and when the session ends unless it is used again.
type sessionResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func newSessionResponse(token string, expiresAt time.Time) sessionResponse {
	return sessionResponse{Token: token, ExpiresAt: expiresAt.UTC()}
}

// profileResponse is the body of GET and PATCH /api/me.
type profileResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func newProfileResponse(p accountgetprofileport.Profile) profileResponse {
	return profileResponse{
		ID: p.ID, Email: p.Email, DisplayName: p.DisplayName, Role: string(p.Role), CreatedAt: p.CreatedAt.UTC(),
	}
}

// userItem is one account in the operator's list and in the invite's answer (ADR-0068). It has
// no password hash, token or financial data.
type userItem struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	// LastActiveAt is null for a user who has no session.
	LastActiveAt *time.Time `json:"last_active_at"`
}

func newUserItem(a domain.Account) userItem {
	item := userItem{
		ID: a.ID, Email: a.Email, DisplayName: a.DisplayName, Role: string(a.Role), Status: string(a.Status),
		CreatedAt: a.CreatedAt.UTC(),
	}
	if a.LastActiveAt != nil {
		t := a.LastActiveAt.UTC()
		item.LastActiveAt = &t
	}
	return item
}

// emailSentResponse is the body of POST /api/admin/users/{id}/set-password-link.
type emailSentResponse struct {
	EmailSent bool `json:"email_sent"`
}
