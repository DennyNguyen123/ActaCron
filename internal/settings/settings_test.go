package settings_test

import (
	"path/filepath"
	"testing"

	"actacron/internal/settings"
	"actacron/internal/storage"
)

func TestAppSettings(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "settings.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	svc := settings.New(db, filepath.Join(tmpDir, ".env"))
	cfg, err := svc.Get()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}

	cfg.ThemeAccent = "#38BDF8"
	cfg.Density = "compact"
	cfg.EditorFontSize = 16
	if err := svc.Save(cfg); err != nil {
		t.Fatalf("save settings failed: %v", err)
	}

	reloaded, _ := svc.Get()
	if reloaded.ThemeAccent != "#38BDF8" || reloaded.Density != "compact" || reloaded.EditorFontSize != 16 {
		t.Fatalf("expected saved theme settings, got: %+v", reloaded)
	}
}
