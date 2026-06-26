// Package config loads runtime configuration from environment variables with
// sane defaults so the API runs out-of-the-box in a container.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
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

	// Authentication (token-based AAA). When AuthEnabled, /api/v1 routes require a
	// valid bearer token obtained from /api/v1/auth/login.
	AuthEnabled  bool
	AuthUsername string        // login username, default "admin"
	AuthPassword string        // login password, default "admin"
	JWTSecret    string        // HS256 signing secret; generated if empty
	TokenTTL     time.Duration // token lifetime, default 1h

	// Retention: events/history older than this many days are purged so the DB
	// does not grow unbounded.
	RetentionDays int
}

// Load reads configuration from the environment.
func Load() Config {
	return Config{
		Port:         getenv("NATIF_PORT", "8080"),
		DBPath:       getenv("NATIF_DB_PATH", "/data/natif.db"),
		DockerHost:   getenv("NATIF_DOCKER_HOST", ""),
		LogLevel:     getenv("NATIF_LOG_LEVEL", "info"),
		AuthEnabled:  getenvBool("NATIF_AUTH_ENABLED", true),
		AuthUsername: getenv("NATIF_AUTH_USERNAME", "admin"),
		AuthPassword: getenv("NATIF_AUTH_PASSWORD", "admin"),
		JWTSecret:     getenv("NATIF_JWT_SECRET", ""),
		TokenTTL:      getenvDuration("NATIF_TOKEN_TTL", time.Hour),
		RetentionDays: getenvInt("NATIF_RETENTION_DAYS", 30),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
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
