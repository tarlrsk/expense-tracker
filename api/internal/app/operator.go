package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	accountaddoperatororch "github.com/tarlrsk/expense-tracker/api/internal/account/orchestrator/addoperator"
	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
	accountreg "github.com/tarlrsk/expense-tracker/api/internal/registry/account"
	mailreg "github.com/tarlrsk/expense-tracker/api/internal/registry/mail"
	"github.com/tarlrsk/expense-tracker/api/internal/tx"
)

// OperatorUsage is how the operator command is called.
const OperatorUsage = "usage: api operator -email <address>"

// RunOperator is the operator command (ADR-0035): `api operator -email <address>`, run by
// `make operator EMAIL=...`. It makes sure an account exists for the email, gives it the operator
// role and, when it has no password yet, emails it a new set-password link. It may be run again.
//
// It uses the API's own settings and wiring: config.Load, db.Open with the startup check (as
// app_login, ADR-0062), the SMTP mailer and the account registry. It prints one line saying what
// it did to stdout and never prints the link or token; logs go to stderr. A failed email is an
// error that says to run the command again.
func RunOperator(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("operator", flag.ContinueOnError)
	fs.SetOutput(stderr)
	email := fs.String("email", "", "the operator's email address")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w\n%s", err, OperatorUsage)
	}
	if fs.NArg() != 0 || *email == "" {
		return errors.New(OperatorUsage)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := openDB(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.LogAttrs(context.WithoutCancel(ctx), slog.LevelError, "close database", slog.String("error", err.Error()))
		}
	}()

	mailer, err := mailreg.NewSend(cfg)
	if err != nil {
		return err
	}

	// The command's work gets the same budget as a request (ADR-0043).
	ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()
	return runOperator(ctx, newDeps(cfg, logger, database, mailer), database, *email, stdout)
}

// runOperator is RunOperator's work with its dependencies passed in (the database as auth, the
// mailer and clock in deps), so tests can run it on the test database with a fake mailer.
func runOperator(ctx context.Context, deps registry.Deps, auth tx.Auth, email string, stdout io.Writer) error {
	resp, err := accountreg.NewAddOperator(deps, auth).Execute(ctx, accountaddoperatororch.Request{Email: email})
	if err != nil {
		return fmt.Errorf("operator: %w", err)
	}
	_, err = fmt.Fprintln(stdout, operatorSummary(resp.Created, resp.Promoted, resp.EmailSent))
	return err
}

// operatorSummary is the one line the command prints.
func operatorSummary(created, promoted, emailSent bool) string {
	var did string
	switch {
	case created:
		did = "Created a new operator account"
	case promoted:
		did = "Made the existing account an operator"
	default:
		did = "The account is already an operator"
	}
	if emailSent {
		return did + "; emailed it a new set-password link (valid for 7 days; earlier links no longer work)."
	}
	return did + "; it already has a password, so no email was sent."
}
