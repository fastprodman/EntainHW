package pgutils

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/fastprodman/EntainHW/internal/config"
	"github.com/fastprodman/EntainHW/pkg/shutdownqueue"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenDB(ctx context.Context, pgConfig *config.PostgresConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", pgConfig.DSN)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}

	db.SetMaxOpenConns(pgConfig.MaxOpenConns)
	db.SetMaxIdleConns(pgConfig.MaxIdleConns)
	db.SetConnMaxIdleTime(pgConfig.ConnMaxIdleTime)
	db.SetConnMaxLifetime(pgConfig.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err = db.PingContext(pingCtx)
	if err != nil {
		//nolint:errcheck
		_ = db.Close()

		return nil, fmt.Errorf("ping database: %w", err)
	}

	shutdownqueue.Add(func(ctx context.Context) error {
		err := db.Close()
		if err != nil {
			return fmt.Errorf("close postgres: %w", err)
		}

		return nil
	})

	return db, nil
}
