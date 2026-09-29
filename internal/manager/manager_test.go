package manager_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/storage"
)

func TestCrossFunctionCall(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	// Create package A with helper
	pkgA := filepath.Join(tmpDir, "pkgA")
	os.MkdirAll(pkgA, 0755)
	os.WriteFile(filepath.Join(pkgA, "double.js"), []byte(`
		function main(params) { return params.n * 2; }
	`), 0644)

	// Create package B that calls package A
	pkgB := filepath.Join(tmpDir, "pkgB")
	os.MkdirAll(pkgB, 0755)
	os.WriteFile(filepath.Join(pkgB, "calc.js"), []byte(`
		function main(params) {
			const res = call("pkgA/double", { n: params.x });
			return res + 1;
		}
	`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := mgr.Call(ctx, "pkgB/calc", map[string]interface{}{"x": 5})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	var numVal float64
	switch v := result.(type) {
	case int64:
		numVal = float64(v)
	case float64:
		numVal = v
	default:
		t.Fatalf("unexpected type %T, val %v", result, result)
	}

	if numVal != 11 {
		t.Fatalf("expected 11, got %v", numVal)
	}
}

func TestCyclicCallDepthExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_cycle.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 5, false)
	mgr := manager.New(tmpDir, runner, db)

	pkg := filepath.Join(tmpDir, "pkgCycle")
	os.MkdirAll(pkg, 0755)
	os.WriteFile(filepath.Join(pkg, "loopA.js"), []byte(`
		function main() { return call("pkgCycle/loopA", {}); }
	`), 0644)

	mgr.Reload()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := mgr.Call(ctx, "pkgCycle/loopA", nil)
	if err == nil {
		t.Fatalf("expected cyclic recursion error, got nil")
	}
}

func TestSaveAndDeleteFunction(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_del.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 5, false)
	mgr := manager.New(tmpDir, runner, db)

	// Save function
	code := `function main() { return "hello"; }`
	if err := mgr.SaveFunctionCode("testpkg", "greet.js", code); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	fn := mgr.GetFunction("testpkg/greet")
	if fn == nil {
		t.Fatalf("expected function to exist after save")
	}

	// Delete function
	if err := mgr.DeleteFunction("testpkg", "greet.js"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	fnAfter := mgr.GetFunction("testpkg/greet")
	if fnAfter != nil {
		t.Fatalf("expected function to be removed after delete")
	}
}

