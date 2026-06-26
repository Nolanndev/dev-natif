// Package config loads runtime configuration from environment variables with
// sane defaults so the API runs out-of-the-box in a container.
package config

import (
	"os"
	"strconv"
)

// Config holds all runtime settings.
type Config struct {
	// HTTP
	Port string // listen port, default "8080"

	// Persistence
	DBPath string // SQLite file path, default "/data/natif.db"

	// Docker
	DockerHost string // engine endpoint; empty => SDK env / default socket

	// Observability
	LogLevel string // "debug" | "info" | "warn" | "error", default "info"

	// Security (AAA hook). When APIKey is non-empty, the api-key middleware is
	// enforced on /api/v1 routes. Empty disables it (MVP default).
	APIKey string
}

// Load reads configuration from the environment.
func Load() Config {
	return Config{
		Port:       getenv("NATIF_PORT", "8080"),
		DBPath:     getenv("NATIF_DB_PATH", "/data/natif.db"),
		DockerHost: getenv("NATIF_DOCKER_HOST", ""),
		LogLevel:   getenv("NATIF_LOG_LEVEL", "info"),
		APIKey:     getenv("NATIF_API_KEY", ""),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// getenvInt is a small helper kept for future numeric settings.
func getenvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
