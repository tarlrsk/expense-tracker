package invite

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcreateaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/createaccount"
	accountmakelinkproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/makelink"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send/sendtest"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
	"github.com/tarlrsk/expense-tracker/api/internal/tx/txtest"
)

type (
	createFn func(ctx context.Context, req accountcreateaccountproc.Request) (accountcreateaccountproc.Response, error)
	linkFn   func(ctx context.Context, req accountmakelinkproc.Request) (accountmakelinkproc.Response, error)
)

func (f createFn) Execute(ctx context.Context, r accountcreateaccountproc.Request) (accountcreateaccountproc.Response, error) {
	return f(ctx, r)
}

func (f linkFn) Execute(ctx context.Context, r accountmakelinkproc.Request) (accountmakelinkproc.Response, error) {
	return f(ctx, r)
}

func TestExecute(t *testing.T) {
	const email = "ann@example.test"
	token := domain.NewToken().Plain
	id := uuid.New()
	tests := []struct {
		name      string
		createErr error
		mailErr   error
		wantKind  apperr.Kind
		wantSent  bool
		wantMails int
		wantTx    txtest.Outcome
	}{
		{name: "sent", wantSent: true, wantMails: 1, wantTx: txtest.Committed},
		{name: "mail fails: still a success, email_sent false", mailErr: errors.New("smtp down"), wantTx: txtest.Committed},
		{
			name: "email taken: no link, no mail", createErr: apperr.New(apperr.Conflict, domain.EmailTakenMessage),
			wantKind: apperr.Conflict, wantTx: txtest.RolledBack,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, mailer := txtest.New(), sendtest.New()
			mailer.FailWith(tt.mailErr)
			var logs bytes.Buffer
			linked := false
			o := New(auth,
				createFn(func(ctx context.Context, _ accountcreateaccountproc.Request) (accountcreateaccountproc.Response, error) {
					if _, err := tx.Require(ctx, tx.RoleAuth); err != nil {
						t.Errorf("create outside the invite's transaction: %v", err)
					}
					return accountcreateaccountproc.Response{Account: domain.Account{ID: id, Email: email}}, tt.createErr
				}),
				linkFn(func(ctx context.Context, req accountmakelinkproc.Request) (accountmakelinkproc.Response, error) {
					if _, err := tx.Require(ctx, tx.RoleAuth); err != nil {
						t.Errorf("make link outside the invite's transaction: %v", err)
					}
					linked = req.UserID == id
					return accountmakelinkproc.Response{Email: email, Token: token}, nil
				}),
				mailer, "https://satang.example/", slog.New(slog.NewJSONHandler(&logs, nil)))

			resp, err := o.Execute(t.Context(), Request{Email: email})
			if apperr.KindOf(err) != tt.wantKind {
				t.Fatalf("error = %v, want kind %q", err, tt.wantKind)
			}
			if r := auth.Records(); len(r) != 1 || r[0].Outcome != tt.wantTx {
				t.Errorf("transactions = %+v, want one %s", r, tt.wantTx)
			}
			// The fake mailer refuses inside a transaction, so a sent mail proves the order.
			sent := mailer.Sent()
			if len(sent) != tt.wantMails {
				t.Fatalf("mails = %d, want %d", len(sent), tt.wantMails)
			}
			if err != nil {
				return
			}
			if !linked || resp.Account.ID != id || resp.EmailSent != tt.wantSent {
				t.Errorf("linked %v, response %+v", linked, resp)
			}
			if tt.wantMails == 1 {
				m := sent[0]
				link := "https://satang.example/set-password#token=" + token
				if m.To != email || m.Subject != domain.SetPasswordSubject || !strings.Contains(m.Text, link) {
					t.Errorf("mail = %+v", m)
				}
			}
			if tt.mailErr != nil {
				out := logs.String()
				if !strings.Contains(out, "set-password email not sent") || !strings.Contains(out, id.String()) {
					t.Errorf("failure not logged: %s", out)
				}
				if strings.Contains(out, email) || strings.Contains(out, token) {
					t.Errorf("log holds the address or the token: %s", out)
				}
			}
		})
	}
}
