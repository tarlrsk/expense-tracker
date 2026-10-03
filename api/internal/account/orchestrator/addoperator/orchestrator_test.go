package addoperator

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcreateaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/createaccount"
	accountmakelinkproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/makelink"
	accountpromoteproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/promote"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send/sendtest"
)

type (
	createFn  func(ctx context.Context, req accountcreateaccountproc.Request) (accountcreateaccountproc.Response, error)
	promoteFn func(ctx context.Context, req accountpromoteproc.Request) (accountpromoteproc.Response, error)
	linkFn    func(ctx context.Context, req accountmakelinkproc.Request) (accountmakelinkproc.Response, error)
)

func (f createFn) Execute(ctx context.Context, r accountcreateaccountproc.Request) (accountcreateaccountproc.Response, error) {
	return f(ctx, r)
}

func (f promoteFn) Execute(ctx context.Context, r accountpromoteproc.Request) (accountpromoteproc.Response, error) {
	return f(ctx, r)
}

func (f linkFn) Execute(ctx context.Context, r accountmakelinkproc.Request) (accountmakelinkproc.Response, error) {
	return f(ctx, r)
}

func TestExecute(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name        string
		exists      bool
		hasPassword bool
		wasOperator bool
		createErr   error
		mailErr     error
		want        Response
		wantErr     string
		wantCalls   []string
	}{
		{
			name: "new account", want: Response{Created: true, Promoted: true, EmailSent: true},
			wantCalls: []string{"promote", "create", "promote", "link"},
		},
		{
			name: "created at the same moment by another run", createErr: apperr.New(apperr.Conflict, ""),
			want:      Response{Promoted: true, EmailSent: true},
			wantCalls: []string{"promote", "create", "promote", "link"},
		},
		{
			name: "invited user", exists: true, want: Response{Promoted: true, EmailSent: true},
			wantCalls: []string{"promote", "link"},
		},
		{
			name: "user with a password: promoted, no email", exists: true, hasPassword: true, want: Response{Promoted: true},
			wantCalls: []string{"promote"},
		},
		{
			name: "operator without a password: a new link", exists: true, wasOperator: true, want: Response{EmailSent: true},
			wantCalls: []string{"promote", "link"},
		},
		{
			name: "operator with a password: nothing to do", exists: true, wasOperator: true, hasPassword: true,
			wantCalls: []string{"promote"},
		},
		{
			name: "bad email", createErr: apperr.New(apperr.InvalidInput, domain.InviteEmailMessage),
			wantErr: domain.InviteEmailMessage, wantCalls: []string{"promote", "create"},
		},
		{
			name: "mail fails: an error that says to run again", exists: true, mailErr: errors.New("smtp down"),
			wantErr: "run the command again", wantCalls: []string{"promote", "link"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mailer := sendtest.New()
			mailer.FailWith(tt.mailErr)
			var calls []string
			exists, role := tt.exists, domain.RoleUser
			if tt.wasOperator {
				role = domain.RoleOperator
			}
			o := New(
				createFn(func(context.Context, accountcreateaccountproc.Request) (accountcreateaccountproc.Response, error) {
					calls = append(calls, "create")
					if tt.createErr == nil || apperr.KindOf(tt.createErr) == apperr.Conflict {
						exists = true
					}
					return accountcreateaccountproc.Response{}, tt.createErr
				}),
				promoteFn(func(context.Context, accountpromoteproc.Request) (accountpromoteproc.Response, error) {
					calls = append(calls, "promote")
					if !exists {
						return accountpromoteproc.Response{}, apperr.New(apperr.NotFound, domain.NoSuchUserMessage)
					}
					changed := role != domain.RoleOperator
					role = domain.RoleOperator
					return accountpromoteproc.Response{UserID: id, Changed: changed, HasPassword: tt.hasPassword}, nil
				}),
				linkFn(func(_ context.Context, req accountmakelinkproc.Request) (accountmakelinkproc.Response, error) {
					calls = append(calls, "link")
					if req.UserID != id {
						t.Errorf("link for %s, want %s", req.UserID, id)
					}
					return accountmakelinkproc.Response{Email: "op@example.test", Token: "tok"}, nil
				}),
				mailer, "https://satang.example")
			resp, err := o.Execute(t.Context(), Request{Email: "op@example.test"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("error = %v", err)
			}
			if !slices.Equal(calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", calls, tt.wantCalls)
			}
			if tt.wantErr == "" && resp != tt.want {
				t.Errorf("response = %+v, want %+v", resp, tt.want)
			}
			wantMails := 0
			if tt.want.EmailSent {
				wantMails = 1
			}
			if sent := len(mailer.Sent()); sent != wantMails {
				t.Errorf("mails sent = %d, want %d", sent, wantMails)
			}
		})
	}
}
