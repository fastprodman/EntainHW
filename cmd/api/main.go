package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fastprodman/EntainHW/internal/infra/pgutils"
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

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			cfg.ShutdownTimeout,
		)
		defer cancel()

		err := shutdownqueue.Shutdown(shutdownCtx)
		if err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	_, err = pgutils.OpenDB(ctx, cfg.Postgres)

	return nil
}
