package send

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// smtpTimeout bounds one Send when ctx has no earlier deadline: connecting and each read or
// write of the conversation.
const smtpTimeout = 30 * time.Second

// smtpAdaptor sends each message over a new SMTP connection (github.com/wneessen/go-mail).
type smtpAdaptor struct {
	host string
	from string
	opts []mail.Option
}

// NewSMTP returns the SMTP adaptor for cfg (ADR-0028). It refuses TLS mode none together with
// credentials, so a password never goes over a clear connection, and an unknown TLS mode. Under
// starttls a server that does not offer STARTTLS is refused; there is no fallback to a clear
// connection.
func NewSMTP(cfg config.SMTP) (Port, error) {
	return newSMTP(cfg, nil)
}

// newSMTP is NewSMTP with an optional TLS configuration; tests pass one that trusts their own
// certificate.
func newSMTP(cfg config.SMTP, tlsConfig *tls.Config) (Port, error) {
	if cfg.Host == "" {
		return nil, errors.New("smtp: the host is empty")
	}
	hasUser, hasPassword := cfg.Username != "", cfg.Password != ""
	if hasUser != hasPassword {
		return nil, errors.New("smtp: set both the username and the password, or neither")
	}

	opts := []mail.Option{mail.WithPort(cfg.Port)}
	switch cfg.TLS {
	case config.SMTPTLSNone:
		if hasUser {
			return nil, errors.New("smtp: credentials are not sent over a clear connection; use starttls or tls")
		}
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	case config.SMTPTLSStartTLS:
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case config.SMTPTLSImplicit:
		opts = append(opts, mail.WithSSL())
	default:
		return nil, fmt.Errorf("smtp: unknown TLS mode %q", cfg.TLS)
	}
	if tlsConfig != nil {
		opts = append(opts, mail.WithTLSConfig(tlsConfig))
	}
	if hasUser {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(cfg.Username),
			mail.WithPassword(cfg.Password.Reveal()),
		)
	}

	// Check the options and the sender once, so a bad setting fails at startup, not at the
	// first email.
	if _, err := mail.NewClient(cfg.Host, opts...); err != nil {
		return nil, fmt.Errorf("smtp: %w", err)
	}
	if err := mail.NewMsg().From(cfg.From); err != nil {
		return nil, errors.New("smtp: the sender address is not valid")
	}
	return &smtpAdaptor{host: cfg.Host, from: cfg.From, opts: opts}, nil
}

// Send implements Port. Errors never quote the recipient's address or the message.
func (a *smtpAdaptor) Send(ctx context.Context, m Message) error {
	if err := tx.MustBeOutside(ctx); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	// go-mail bounds the connection by its own timeout, not by ctx's deadline, so the timeout is
	// cut to the time ctx has left (ADR-0043).
	timeout := smtpTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}
	if timeout <= 0 {
		return fmt.Errorf("send mail: %w", context.DeadlineExceeded)
	}

	msg := mail.NewMsg()
	if err := msg.From(a.from); err != nil {
		return errors.New("send mail: the sender address is not valid")
	}
	if err := msg.To(m.To); err != nil {
		return errors.New("send mail: the recipient address is not valid")
	}
	msg.Subject(m.Subject)
	msg.SetBodyString(mail.TypeTextPlain, m.Text)

	client, err := mail.NewClient(a.host, append(a.opts[:len(a.opts):len(a.opts)], mail.WithTimeout(timeout))...)
	if err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		err = withoutRecipients(err)
		if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
			return fmt.Errorf("send mail: %w (context: %w)", err, ctxErr)
		}
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// withoutRecipients replaces a go-mail delivery error, whose text lists the recipients and the
// server's replies (which may quote the address), by its reason and SMTP codes.
func withoutRecipients(err error) error {
	var sendErr *mail.SendError
	if !errors.As(err, &sendErr) {
		return err
	}
	return fmt.Errorf("%s (SMTP code %d, enhanced %q, temporary %t)",
		sendErr.Reason, sendErr.ErrorCode(), sendErr.EnhancedStatusCode(), sendErr.IsTemp())
}
