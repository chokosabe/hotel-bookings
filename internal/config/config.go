// Package config loads the process configuration from environment variables.
package config

import (
	"fmt"
	"strings"
)

const (
	defaultPort         = "8080"
	defaultDatabasePath = "./data/hotel-bookings.db"
)

// Config contains the values required to run the API.
type Config struct {
	Port                string
	DatabasePath        string
	EnableTestEndpoints bool
}

// Load reads configuration through getenv, making parsing independently testable.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:                strings.TrimSpace(getenv("PORT")),
		DatabasePath:        strings.TrimSpace(getenv("DATABASE_PATH")),
		EnableTestEndpoints: true,
	}

	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = defaultDatabasePath
	}

	switch strings.ToLower(strings.TrimSpace(getenv("ENABLE_TEST_ENDPOINTS"))) {
	case "", "true", "1":
	case "false", "0":
		cfg.EnableTestEndpoints = false
	default:
		return Config{}, fmt.Errorf("ENABLE_TEST_ENDPOINTS must be true or false")
	}

	return cfg, nil
}
