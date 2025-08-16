package main

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/fastprodman/EntainHW/internal/infra/logging"
	"github.com/fastprodman/EntainHW/pkg/envconf"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var baseFS embed.FS

//go:embed test_data/*.sql
var devFS embed.FS

type migratorConfig struct {
	DSN      string     `env:"PG_DSN"`
	LogLevel slog.Level `env:"APP_LOG_LEVEL"`
	AppEnv   string     `enc:"APP_ENV"`
}

func main() {
	err := migrateAll()
	if err != nil {
		slog.Error("migration run failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migration run finished successfully")
}

func migrateAll() error {
	cfg := new(migratorConfig)

	err := envconf.Load(cfg)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logging.SetupJSON(cfg.LogLevel)

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	//nolint:errcheck
	defer db.Close()

	err = db.Ping()
	if err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("init postgres driver: %w", err)
	}

	err = runMigrations(driver, baseFS, "migrations")
	if err != nil {
		return fmt.Errorf("base migrations failed: %w", err)
	}

	slog.Info("base migrations applied")

	if cfg.AppEnv == "DEV" {
		err = runMigrations(driver, devFS, "test_data")
		if err != nil {
			return fmt.Errorf("dev seed migrations failed: %w", err)
		}

		slog.Info("dev seed migrations applied")
	}

	return nil
}

func runMigrations(driver database.Driver, fsys embed.FS, dir string) error {
	src, err := iofs.New(fsys, dir)
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("m.Up: %w", err)
	}

	return nil
}
