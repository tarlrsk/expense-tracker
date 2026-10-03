package invite

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcreateaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/createaccount"
	accountmakelinkproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/makelink"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

type orchestrator struct {
	auth       tx.Auth
	create     accountcreateaccountproc.Processor
	link       accountmakelinkproc.Processor
	mailer     send.Port
	webBaseURL string
	logger     *slog.Logger
}

// New returns the invite use case. webBaseURL is WEB_BASE_URL, the start of the emailed link.
func New(auth tx.Auth, create accountcreateaccountproc.Processor, link accountmakelinkproc.Processor,
	mailer send.Port, webBaseURL string, logger *slog.Logger,
) Orchestrator {
	return &orchestrator{auth: auth, create: create, link: link, mailer: mailer, webBaseURL: webBaseURL, logger: logger}
}

// Execute creates the account and its link in one auth transaction, so an invite leaves either
// both or nothing, and sends the email only after that transaction has committed (ADR-0032). A
// failed email is logged without the address, token or link.
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	var (
		account domain.Account
		link    accountmakelinkproc.Response
	)
	err := o.auth.WithAuthTx(ctx, func(ctx context.Context) error {
		created, err := o.create.Execute(ctx, accountcreateaccountproc.Request{Email: req.Email})
		if err != nil {
			return err
		}
		account = created.Account
		link, err = o.link.Execute(ctx, accountmakelinkproc.Request{UserID: account.ID})
		return err
	})
	if err != nil {
		return Response{}, fmt.Errorf("invite: %w", err)
	}

	err = o.mailer.Send(ctx, send.Message{
		To:      link.Email,
		Subject: domain.SetPasswordSubject,
		Text:    domain.SetPasswordText(domain.SetPasswordLink(o.webBaseURL, link.Token)),
	})
	if err != nil {
		o.logger.LogAttrs(ctx, slog.LevelError, "set-password email not sent",
			slog.String("use_case", "invite"), slog.String("user_id", account.ID.String()), slog.String("error", err.Error()))
		return Response{Account: account, EmailSent: false}, nil
	}
	return Response{Account: account, EmailSent: true}, nil
}
