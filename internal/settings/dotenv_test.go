package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/settings"
)

func TestDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	initialContent := "API_KEY=secret_123\nPORT=8080\n# comment\nDEBUG=true\n"
	os.WriteFile(envPath, []byte(initialContent), 0644)

	envMap, err := settings.ReadEnv(envPath)
	if err != nil {
		t.Fatalf("read env failed: %v", err)
	}
	if envMap["API_KEY"] != "secret_123" || envMap["PORT"] != "8080" {
		t.Fatalf("unexpected env map: %v", envMap)
	}

	envMap["API_KEY"] = "updated_token"
	envMap["NEW_VAR"] = "val"
	if err := settings.WriteEnv(envPath, envMap); err != nil {
		t.Fatalf("write env failed: %v", err)
	}

	updated, _ := settings.ReadEnv(envPath)
	if updated["API_KEY"] != "updated_token" || updated["NEW_VAR"] != "val" {
		t.Fatalf("expected updated env, got: %v", updated)
	}
}
