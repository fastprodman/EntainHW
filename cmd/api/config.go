package main

import (
	"time"

	"github.com/fastprodman/EntainHW/internal/config"
)

type apiConfig struct {
	Port            uint16        `env:"API_PORT"`
	ShutdownTimeout time.Duration `env:"API_SHUTDOWN_TIMEOUT"`
	Postgres        *config.PostgresConfig
}
