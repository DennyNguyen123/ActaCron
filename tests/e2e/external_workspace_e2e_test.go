package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/storage"
)

func TestExternalWorkspaceE2E(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	extFolder := filepath.Join(tmpDir, "my_standalone_scripts")
	os.MkdirAll(packagesDir, 0755)
	os.MkdirAll(extFolder, 0755)

	db, err := storage.New(filepath.Join(tmpDir, "app.db"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// 1. Create files in standalone folder
	os.WriteFile(filepath.Join(extFolder, ".env"), []byte("API_SECRET=supersecret123\n"), 0644)
	os.WriteFile(filepath.Join(extFolder, "runner.js"), []byte(`
/**
 * @name runTask
 */
function main(params) {
    return { status: "ok", secret: env("API_SECRET"), count: params.count + 1 };
}
`), 0644)

	// 2. Register external workspace in DB
	if err := db.AddExternalWorkspace("standalone", extFolder); err != nil {
		t.Fatalf("failed to register external ws: %v", err)
	}

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload failed: %v", err)
	}

	// 3. Verify files are NOT in packagesDir
	copiedTarget := filepath.Join(packagesDir, "standalone")
	if _, err := os.Stat(copiedTarget); !os.IsNotExist(err) {
		t.Fatalf("VIOLATION: files were copied into packages directory: %s", copiedTarget)
	}

	// 4. Verify function execution
	out, err := mgr.CallWithTrigger(context.Background(), "standalone/runTask", map[string]interface{}{"count": 5}, "manual")
	if err != nil {
		t.Fatalf("CallWithTrigger failed: %v", err)
	}
	resMap, ok := out.(map[string]interface{})
	if !ok || resMap["status"] != "ok" || resMap["secret"] != "supersecret123" || resMap["count"] != int64(6) {
		t.Fatalf("unexpected result: %v", resMap)
	}

	// 5. Unlink external workspace
	if err := db.DeleteExternalWorkspace("standalone"); err != nil {
		t.Fatalf("failed to delete external ws: %v", err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload after unlink failed: %v", err)
	}

	if mgr.GetPackage("standalone") != nil {
		t.Fatalf("expected package to be unlinked")
	}

	// Original folder must remain untouched on disk
	if _, err := os.Stat(filepath.Join(extFolder, "runner.js")); err != nil {
		t.Fatalf("original file was removed or corrupted: %v", err)
	}
}
