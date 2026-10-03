// Command api runs the Expense Tracker API.
//
//	api                          serve the API (make run)
//	api operator -email <addr>   create or promote an operator (make operator EMAIL=...; ADR-0035)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/tarlrsk/expense-tracker/api/internal/app"
)

func main() {
	args := os.Args[1:]
	switch {
	case len(args) == 0:
		if err := app.Run(context.Background()); err != nil {
			slog.Error("api stopped", slog.String("error", err.Error()))
			os.Exit(1)
		}
	case args[0] == "operator":
		if err := app.RunOperator(context.Background(), args[1:], os.Stdout, os.Stderr); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command %q\nusage: api [operator -email <address>]\n", args[0])
		os.Exit(2)
	}
}
