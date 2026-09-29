package config_test

import (
	"os"
	"testing"

	"actacron/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Load()
	if cfg.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Fatalf("expected timeout 30, got %d", cfg.TimeoutSeconds)
	}
	if cfg.LogRetentionDays != 7 {
		t.Fatalf("expected log retention 7, got %d", cfg.LogRetentionDays)
	}
}

func TestEnvOverrideConfig(t *testing.T) {
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")

	cfg := config.Load()
	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}
}
