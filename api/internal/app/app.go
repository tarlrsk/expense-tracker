// Package app builds the API from its parts and runs it.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tarlrsk/expense-tracker/api/internal/config"
	"github.com/tarlrsk/expense-tracker/api/internal/db"
	"github.com/tarlrsk/expense-tracker/api/internal/registry"
)

const (
	readHeaderTimeout = 10 * time.Second
	writeTimeoutExtra = 10 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
	// dbStartupTimeout covers connecting and the startup check, including a cold Neon start.
	dbStartupTimeout = 30 * time.Second
)

// Run opens the database, starts the HTTP server and blocks until ctx is done, SIGINT or SIGTERM
// arrives, or the server fails. It then shuts down gracefully and closes the database.
// It refuses to start when DATABASE_URL is missing, the database cannot be reached, or the role
// it logs in as could read data outside WithUserTx / WithAuthTx (db.Check).
func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

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

	// database is also the auth transactor (tx.Auth). It is not in Deps: it goes only to the
	// account module's Register, from PLAN-0002 T5 on (ADR-0032, ADR-0034).
	engine, err := NewEngine(registry.Deps{Config: cfg, Logger: logger, UserTx: database})
	if err != nil {
		return fmt.Errorf("build engine: %w", err)
	}

	srv := &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      cfg.RequestTimeout + writeTimeoutExtra,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.LogAttrs(ctx, slog.LevelInfo, "listening", slog.String("addr", ln.Addr().String()))

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	stop()

	logger.LogAttrs(ctx, slog.LevelInfo, "shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// openDB connects to DATABASE_URL and runs the startup check. /api/healthz never uses it
// (ADR-0048).
func openDB(ctx context.Context, cfg config.Config, logger *slog.Logger) (*db.DB, error) {
	ctx, cancel := context.WithTimeout(ctx, dbStartupTimeout)
	defer cancel()
	database, err := db.Open(ctx, db.Config{
		URL:              cfg.DatabaseURL.Reveal(),
		StatementTimeout: cfg.DBStatementTimeout,
		MaxOpenConns:     cfg.DBMaxOpenConns,
		Logger:           logger,
	})
	if err != nil {
		return nil, fmt.Errorf("open the database: %w", err)
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "database ready")
	return database, nil
}
