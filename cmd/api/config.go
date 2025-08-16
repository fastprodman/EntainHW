package main

type apiConfig struct {
	Port     string
	Postgres PostgresConf
}

type PostgresConf struct {
	DSN string
}

func readConfig() (*apiConfig, error) {
	// TODO: add config reading logic
	return &apiConfig{}, nil
}
