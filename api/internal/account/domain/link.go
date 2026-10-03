package domain

import (
	"strings"
	"time"
)

// LinkLifetime: a set-password link works for this long after it is made, once (ADR-0037).
const LinkLifetime = 7 * 24 * time.Hour

// SetPasswordSubject is the subject of the set-password email (ADR-0068).
const SetPasswordSubject = "Set your Satang password" //nolint:gosec // an email subject, not a credential

// SetPasswordLink is the link in the set-password email: <webBaseURL>/set-password#token=<token>.
// The token is in the fragment, which a browser never sends to a server, so it stays out of
// server and proxy logs (ADR-0068). A trailing slash on webBaseURL is ignored.
func SetPasswordLink(webBaseURL, token string) string {
	return strings.TrimRight(webBaseURL, "/") + "/set-password#token=" + token
}

// SetPasswordText is the plain-text body of the set-password email that carries link.
func SetPasswordText(link string) string {
	return "Hello,\n\n" +
		"Use this link to set your Satang password:\n\n" +
		link + "\n\n" +
		"The link is valid for 7 days and works once. If it has expired, ask for a new one.\n" +
		"If you did not expect this email, you can ignore it.\n"
}
