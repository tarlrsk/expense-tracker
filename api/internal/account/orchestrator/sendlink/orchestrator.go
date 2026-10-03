package sendlink

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tarlrsk/expense-tracker/api/internal/account/domain"
	accountmakelinkproc "github.com/tarlrsk/expense-tracker/api/internal/account/processor/makelink"
	"github.com/tarlrsk/expense-tracker/api/internal/external/mail/send"
)

type orchestrator struct {
	link       accountmakelinkproc.Processor
	mailer     send.Port
	webBaseURL string
	logger     *slog.Logger
}

// New returns the send-link use case. webBaseURL is WEB_BASE_URL, the start of the emailed link.
func New(link accountmakelinkproc.Processor, mailer send.Port, webBaseURL string, logger *slog.Logger) Orchestrator {
	return &orchestrator{link: link, mailer: mailer, webBaseURL: webBaseURL, logger: logger}
}

// Execute makes the link (its own auth transaction commits first), then emails it (ADR-0032). A
// failed email is logged without the address, token or link.
func (o *orchestrator) Execute(ctx context.Context, req Request) (Response, error) {
	link, err := o.link.Execute(ctx, accountmakelinkproc.Request{UserID: req.UserID})
	if err != nil {
		return Response{}, fmt.Errorf("send link: %w", err)
	}
	err = o.mailer.Send(ctx, send.Message{
		To:      link.Email,
		Subject: domain.SetPasswordSubject,
		Text:    domain.SetPasswordText(domain.SetPasswordLink(o.webBaseURL, link.Token)),
	})
	if err != nil {
		o.logger.LogAttrs(ctx, slog.LevelError, "set-password email not sent",
			slog.String("use_case", "send link"), slog.String("user_id", req.UserID.String()), slog.String("error", err.Error()))
		return Response{EmailSent: false}, nil
	}
	return Response{EmailSent: true}, nil
}
