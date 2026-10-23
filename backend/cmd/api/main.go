// Command api runs the HTTP API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"learning-serbian-platform/backend/internal/config"
	"learning-serbian-platform/backend/internal/database"
	"learning-serbian-platform/backend/internal/httpapi"
	"learning-serbian-platform/backend/internal/identity"
	"learning-serbian-platform/backend/internal/learning"
	"learning-serbian-platform/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := store.New(pool)
	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			DB:           pool,
			Auth:         identity.NewService(queries, cfg.SessionTTL),
			Vocabulary:   learning.NewVocabularyService(queries),
			AppOrigin:    cfg.AppOrigin,
			CookieSecure: cfg.CookieSecure,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if !cfg.CookieSecure {
		slog.Warn("COOKIE_SECURE=false: session cookies are sent over plain HTTP; use only for local development")
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
