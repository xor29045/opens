// Command worker runs the scheduled tasks (renewals, grace, reconciliation,
// publication recompute, purges).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"opensex/backend/internal/app"
	"opensex/backend/internal/config"
	"opensex/backend/internal/jobs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
	defer a.Close()
	a.Logger.Info("worker started")
	jobs.Scheduler(ctx, a.Svc, a.Logger)
}
