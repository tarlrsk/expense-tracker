package addoperator

import (
	"context"
	"fmt"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountcreateaccountproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/createaccount"
	accountmakelinkproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/makelink"
	accountpromoteproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/promote"
	"github.com/tarlrsk/expense-tracker/api/internal/apperr"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
)

type orchestrator struct {
	create     accountcreateaccountproc.Processor
	promote    accountpromoteproc.Processor
	link       accountmakelinkproc.Processor
	mailer     send.Port
	webBaseURL string
}

// New returns the add-operator use case. webBaseURL is WEB_BASE_URL, the start of the emailed
// link.
func New(create accountcreateaccountproc.Processor, promote accountpromoteproc.Processor,
	link accountmakelinkproc.Processor, mailer send.Port, webBaseURL string,
) Orchestrator {
	return &orchestrator{create: create, promote: promote, link: link, mailer: mailer, webBaseURL: webBaseURL}
}

// Execute promotes the account; when there is none, it creates it through the one
// account-creation processor and promotes it then. Each step commits on its own, so a run that
// stops half way is completed by running it again. The email is sent last, outside any
// transaction (ADR-0032).
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	var resp Response
	promoted, err := o.promote.Execute(ctx, accountpromoteproc.Request{Email: req.Email})
	if apperr.KindOf(err) == apperr.NotFound {
		_, err = o.create.Execute(ctx, accountcreateaccountproc.Request{Email: req.Email})
		switch {
		case err == nil:
			resp.Created = true
		case apperr.KindOf(err) != apperr.Conflict: // conflict: made by a run at the same moment
			return Response{}, fmt.Errorf("add operator: %w", err)
		}
		promoted, err = o.promote.Execute(ctx, accountpromoteproc.Request{Email: req.Email})
	}
	if err != nil {
		return Response{}, fmt.Errorf("add operator: %w", err)
	}
	resp.Promoted = promoted.Changed
	if promoted.HasPassword {
		return resp, nil
	}

	link, err := o.link.Execute(ctx, accountmakelinkproc.Request{UserID: promoted.UserID})
	if err != nil {
		return resp, fmt.Errorf("add operator: %w", err)
	}
	err = o.mailer.Send(ctx, send.Message{
		To:      link.Email,
		Subject: domain.SetPasswordSubject,
		Text:    domain.SetPasswordText(domain.SetPasswordLink(o.webBaseURL, link.Token)),
	})
	if err != nil {
		return resp, fmt.Errorf("add operator: the account is ready but the set-password email was not sent; "+
			"run the command again to send a new link: %w", err)
	}
	resp.EmailSent = true
	return resp, nil
}
