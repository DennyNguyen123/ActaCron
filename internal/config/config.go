package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port             int
	Host             string
	DataDir          string
	PackagesDir      string
	TimeoutSeconds   int
	LogRetentionDays int
	AllowShell       bool
}

func Load() *Config {
	port := 8080
	if p, err := strconv.Atoi(os.Getenv("PORT")); err == nil && p > 0 {
		port = p
	}

	timeout := 30
	if t, err := strconv.Atoi(os.Getenv("TIMEOUT_SECONDS")); err == nil && t > 0 {
		timeout = t
	}

	retention := 7
	if r, err := strconv.Atoi(os.Getenv("LOG_RETENTION_DAYS")); err == nil && r > 0 {
		retention = r
	}

	return &Config{
		Port:             port,
		Host:             "127.0.0.1",
		DataDir:          "data",
		PackagesDir:      "packages",
		TimeoutSeconds:   timeout,
		LogRetentionDays: retention,
		AllowShell:       os.Getenv("ALLOW_SHELL") == "true",
	}
}
