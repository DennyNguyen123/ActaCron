package manager_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/storage"
)

func TestEffectiveTimeoutHierarchy(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_timeout.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 30, false)
	mgr := manager.New(tmpDir, runner, db)

	// Case 1: Workspace with workspace.json (timeout = 45s)
	pkgWithWsTimeout := filepath.Join(tmpDir, "pkgCustom")
	os.MkdirAll(pkgWithWsTimeout, 0755)
	os.WriteFile(filepath.Join(pkgWithWsTimeout, "workspace.json"), []byte(`{"timeout_seconds": 45}`), 0644)
	os.WriteFile(filepath.Join(pkgWithWsTimeout, "default_fn.js"), []byte(`function main() { return 1; }`), 0644)

	// Case 2: Function with explicit @timeout 120s overriding workspace.json
	os.WriteFile(filepath.Join(pkgWithWsTimeout, "long_fn.js"), []byte(`
/**
 * @name long_task
 * @timeout 120
 */
function main() { return 2; }
`), 0644)

	// Case 3: Workspace without workspace.json or timeout
	pkgDefault := filepath.Join(tmpDir, "pkgDefault")
	os.MkdirAll(pkgDefault, 0755)
	os.WriteFile(filepath.Join(pkgDefault, "plain.js"), []byte(`function main() { return 3; }`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	// Test Case 1: Uses workspace timeout (45s)
	t1 := mgr.GetEffectiveTimeout("pkgCustom", "default_fn")
	if t1 != 45*time.Second {
		t.Errorf("expected 45s for pkgCustom/default_fn, got %v", t1)
	}

	// Test Case 2: Function @timeout overrides workspace timeout (120s)
	t2 := mgr.GetEffectiveTimeout("pkgCustom", "long_task")
	if t2 != 120*time.Second {
		t.Errorf("expected 120s for pkgCustom/long_task, got %v", t2)
	}

	// Test Case 3: Fallback to global runner default (30s)
	t3 := mgr.GetEffectiveTimeout("pkgDefault", "plain")
	if t3 != 30*time.Second {
		t.Errorf("expected 30s for pkgDefault/plain, got %v", t3)
	}
}
