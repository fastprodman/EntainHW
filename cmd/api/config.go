package main

import (
	"log/slog"
	"time"

	"github.com/fastprodman/EntainHW/internal/config"
)

type apiConfig struct {
	Port            uint16        `env:"API_PORT"`
	ShutdownTimeout time.Duration `env:"API_SHUTDOWN_TIMEOUT"`
	LogLevel        slog.Level    `env:"APP_LOG_LEVEL"`
	Postgres        *config.PostgresConfig
}
