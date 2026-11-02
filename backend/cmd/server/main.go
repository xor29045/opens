// Command server runs the HTTP API. The bootstrap administrators are
// ensured before the port is opened (RM-02b).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"opensex/backend/internal/app"
	"opensex/backend/internal/config"
	"opensex/backend/internal/database"
	"opensex/backend/internal/routes"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.IsDev() && os.Getenv("AUTO_MIGRATE") == "true" {
		if err := database.MigrateUp(cfg.DatabaseURL); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer a.Close()
	if err := a.Svc.Bootstrap.EnsureBootstrapAdmins(ctx); err != nil {
		return err
	}
	engine, _ := routes.New(routes.Deps{Cfg: cfg, Svc: a.Svc, Logger: a.Logger, Limiter: a.Limiter})
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.APIPort), Handler: engine,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	errCh := make(chan error, 1)
	go func() {
		a.Logger.Info("api listening", "port", cfg.APIPort, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}
