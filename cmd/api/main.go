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

	"github.com/fastprodman/EntainHW/internal/api"
	"github.com/fastprodman/EntainHW/internal/infra/logging"
	"github.com/fastprodman/EntainHW/internal/infra/pgutils"
	"github.com/fastprodman/EntainHW/internal/services/balance"
	"github.com/fastprodman/EntainHW/pkg/envconf"
	"github.com/fastprodman/EntainHW/pkg/shutdownqueue"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running api: %v", err)
		//nolint:gocritic
		os.Exit(1)
	}
}

func run(ctx context.Context) (retErr error) {
	cfg := new(apiConfig)

	err := envconf.Load(cfg)
	if err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	logging.SetupJSON(cfg.LogLevel)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		serr := shutdownqueue.Shutdown(shutdownCtx)
		if serr != nil {
			retErr = errors.Join(retErr, serr)
		}
	}()

	// --- Infra ---
	dbConns, err := pgutils.OpenDB(ctx, cfg.Postgres)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	balanceSrv := balance.New(dbConns)

	// --- HTTP server ---
	srv := api.NewServer(cfg.Port, balanceSrv)

	// Register HTTP server graceful shutdown
	shutdownqueue.Add(func(c context.Context) error {
		slog.Info("Shut down server")

		err := srv.Shutdown(c)
		if err != nil {
			return fmt.Errorf("shutdown srv: %w", err)
		}

		return nil
	})

	// Run server
	errCh := make(chan error, 1)

	go func() {
		serr := srv.ListenAndServe()
		// http.ErrServerClosed is the normal path during Shutdown
		if serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			errCh <- serr
			return
		}

		errCh <- nil
	}()

	slog.Info("API started")

	// --- Wait until either context cancels or server errors out ---
	select {
	case <-ctx.Done():
		// graceful path; deferred shutdownqueue.Shutdown will run
		return nil
	case serr := <-errCh:
		if serr != nil {
			return fmt.Errorf("server error: %w", serr)
		}

		return nil
	}
}
