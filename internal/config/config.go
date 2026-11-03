// Package config reads Huginn's settings from environment variables.
package config

import (
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultAddr    = ":8080"
	defaultDataDir = "/data"
)

// Config holds the settings the server needs at start-up.
type Config struct {
	Addr         string // HUGINN_ADDR
	DataDir      string // HUGINN_DATA_DIR
	BlockPrivate bool   // HUGINN_BLOCK_PRIVATE_NETWORKS: refuse to fetch feeds from private networks
}

// Load reads the configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		Addr:    getenv("HUGINN_ADDR", defaultAddr),
		DataDir: getenv("HUGINN_DATA_DIR", defaultDataDir),

		BlockPrivate: getbool("HUGINN_BLOCK_PRIVATE_NETWORKS"),
	}
}

// DBPath is the location of the SQLite database file.
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "huginn.db")
}

// HealthURL is the address the healthcheck subcommand requests. It always
// targets the loopback interface, whatever host the server is bound to.
func (c Config) HealthURL() string {
	_, port, err := net.SplitHostPort(c.Addr)
	if err != nil || port == "" {
		port = "8080"
	}
	return "http://" + net.JoinHostPort("127.0.0.1", port) + "/healthz"
}

// getbool reads a switch: 1, true, yes or on turn it on, anything else is off.
func getbool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
