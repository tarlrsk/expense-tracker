// Command api runs the Expense Tracker API.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/tarlrsk/expense-tracker/api/internal/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		slog.Error("api stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
