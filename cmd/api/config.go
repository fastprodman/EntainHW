package main

import "fmt"

type apiConfig struct {
	Port     string
	Postgres PostgresConf
}

type PostgresConf struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

func readConfig() (*apiConfig, error) {
	// TODO: add config reading logic
	return &apiConfig{}, nil
}

func (pc PostgresConf) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		pc.User, pc.Password, pc.Host, pc.Port, pc.DBName,
	)
}
