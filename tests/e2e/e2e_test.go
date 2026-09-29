package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/settings"
	"actacron/internal/storage"
)

func TestEndToEndPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "actacron.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	// Test settings & dotenv
	envPath := filepath.Join(tmpDir, ".env")
	os.WriteFile(envPath, []byte("GREET_PREFIX=Hello"), 0644)
	settingsSvc := settings.New(db, envPath)

	cfg, _ := settingsSvc.Get()
	cfg.ThemeAccent = "#22C55E"
	if err := settingsSvc.Save(cfg); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	// Create a test function with cron & mcp metadata
	pkgDir := filepath.Join(tmpDir, "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	scriptContent := `/**
 * @name greet_task
 * @cron * * * * *
 * @mcp true
 * @param {string} name - Target name
 */
function main(params) {
	const prefix = env("GREET_PREFIX") || "Hi";
	const res = prefix + " " + params.name;
	console.log("Greeting generated:", res);
	storage.set("last_greet", res);
	return res;
}`
	if err := os.WriteFile(filepath.Join(pkgDir, "greet.js"), []byte(scriptContent), 0644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	fn := mgr.GetFunction("testpkg/greet_task")
	if fn == nil || !fn.IsMCP || fn.CronExpr != "* * * * *" {
		t.Fatalf("function metadata not parsed correctly: %+v", fn)
	}

	// Execute manual run
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := mgr.Call(ctx, "testpkg/greet_task", map[string]interface{}{"name": "Antigravity"})
	if err != nil || res != "Hello Antigravity" {
		t.Fatalf("unexpected call result: %v, err: %v", res, err)
	}

	// Verify log in SQLite
	_, total, err := db.QueryLogs("greet_task", "", 10, 0)
	if err != nil || total < 1 {
		t.Fatalf("expected logged execution in SQLite, got %d, err: %v", total, err)
	}

	// Verify KV state
	stored, err := db.GetKV("testpkg", "last_greet")
	if err != nil || stored != "Hello Antigravity" {
		t.Fatalf("expected KV 'Hello Antigravity', got '%s'", stored)
	}

	// Verify settings persisted
	reloadedCfg, _ := settingsSvc.Get()
	if reloadedCfg.ThemeAccent != "#22C55E" {
		t.Fatalf("expected accent #22C55E, got %s", reloadedCfg.ThemeAccent)
	}
}
