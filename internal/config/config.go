package config

import (
	"log/slog"
	"time"
)

type PostgresConfig struct {
	DSN             string        `env:"PG_DSN"`
	MaxOpenConns    int           `env:"PG_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `env:"PG_MAX_IDLE_CONNS"`
	ConnMaxIdleTime time.Duration `env:"PG_CONN_MAX_IDLE_TIME"`
	ConnMaxLifetime time.Duration `env:"PG_CONN_MAX_LIFETIME"`
}

type LoggerConfig struct {
	LogLevel slog.Level `env:"APP_LOG_LEVEL"`
}
