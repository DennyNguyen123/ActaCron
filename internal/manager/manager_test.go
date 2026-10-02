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

func TestWorkspaceEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_env.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	os.Setenv("GLOBAL_ONLY", "world")
	os.Setenv("API_KEY", "global_secret")
	defer os.Unsetenv("GLOBAL_ONLY")
	defer os.Unsetenv("API_KEY")

	runner := engine.New(db, 5, false)
	mgr := manager.New(tmpDir, runner, db)

	pkgPath := filepath.Join(tmpDir, "myPkg")
	os.MkdirAll(pkgPath, 0755)

	// Workspace .env
	os.WriteFile(filepath.Join(pkgPath, ".env"), []byte("API_KEY=workspace_secret\nLOCAL_ONLY=hello\n"), 0644)

	// Workspace script
	os.WriteFile(filepath.Join(pkgPath, "check_env.js"), []byte(`
		function main() {
			return {
				apiKey: env("API_KEY"),
				localOnly: env("LOCAL_ONLY"),
				globalOnly: env("GLOBAL_ONLY"),
				allVars: env.all ? env.all() : null
			};
		}
	`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := mgr.Call(ctx, "myPkg/check_env", nil)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}

	resMap, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T: %v", res, res)
	}

	if resMap["apiKey"] != "workspace_secret" {
		t.Errorf("expected apiKey=workspace_secret, got %v", resMap["apiKey"])
	}
	if resMap["localOnly"] != "hello" {
		t.Errorf("expected localOnly=hello, got %v", resMap["localOnly"])
	}
	if resMap["globalOnly"] != "world" {
		t.Errorf("expected globalOnly=world, got %v", resMap["globalOnly"])
	}
	var apiKeyVal string
	if mStr, ok := resMap["allVars"].(map[string]string); ok {
		apiKeyVal = mStr["API_KEY"]
	} else if mAny, ok := resMap["allVars"].(map[string]interface{}); ok {
		apiKeyVal, _ = mAny["API_KEY"].(string)
	} else {
		t.Fatalf("unexpected type for allVars: %T", resMap["allVars"])
	}
	if apiKeyVal != "workspace_secret" {
		t.Errorf("expected allVars[API_KEY]=workspace_secret, got %q", apiKeyVal)
	}
}

func TestWorkspaceEnvExampleFallback(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_env_example.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 5, false)
	mgr := manager.New(tmpDir, runner, db)

	pkgPath := filepath.Join(tmpDir, "myPkg")
	os.MkdirAll(pkgPath, 0755)

	// Create only .env.example
	os.WriteFile(filepath.Join(pkgPath, ".env.example"), []byte("API_URL=example_url\n"), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if mgr.HasWorkspaceEnvFile("myPkg") {
		t.Errorf("expected HasWorkspaceEnvFile to be false when only .env.example exists")
	}

	envMap := mgr.GetWorkspaceEnv("myPkg")
	if envMap["API_URL"] != "example_url" {
		t.Errorf("expected API_URL=example_url from .env.example fallback, got %q", envMap["API_URL"])
	}

	exampleEnv := mgr.GetWorkspaceExampleEnv("myPkg")
	if exampleEnv["API_URL"] != "example_url" {
		t.Errorf("expected API_URL=example_url in exampleEnv, got %q", exampleEnv["API_URL"])
	}

	// Now add a real .env, it should override
	os.WriteFile(filepath.Join(pkgPath, ".env"), []byte("API_URL=real_url\n"), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if !mgr.HasWorkspaceEnvFile("myPkg") {
		t.Errorf("expected HasWorkspaceEnvFile to be true when .env exists")
	}

	envMap2 := mgr.GetWorkspaceEnv("myPkg")
	if envMap2["API_URL"] != "real_url" {
		t.Errorf("expected API_URL=real_url from .env override, got %q", envMap2["API_URL"])
	}
}

func TestExternalWorkspaceLoadingAndExecution(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	extDir := filepath.Join(tmpDir, "external_project")
	os.MkdirAll(packagesDir, 0755)
	os.MkdirAll(extDir, 0755)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Register external workspace in DB
	err = db.AddExternalWorkspace("myext", extDir)
	if err != nil {
		t.Fatalf("failed to add external ws: %v", err)
	}

	// Create a script and .env in external dir
	os.WriteFile(filepath.Join(extDir, ".env"), []byte("FOO=bar_ext\n"), 0644)
	os.WriteFile(filepath.Join(extDir, "calc.js"), []byte(`
/**
 * @name calculate
 */
function main(params) {
    return { res: params.a * 2, env: env("FOO") };
}
`), 0644)

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload failed: %v", err)
	}

	// Verify package is loaded as external
	pkg := mgr.GetPackage("myext")
	if pkg == nil {
		t.Fatalf("expected package myext to exist")
	}
	if !pkg.IsExternal {
		t.Fatalf("expected pkg.IsExternal to be true")
	}
	if pkg.Path != extDir {
		t.Fatalf("expected pkg.Path=%s, got %s", extDir, pkg.Path)
	}

	// Verify function is loaded
	fn := mgr.GetFunction("myext/calculate")
	if fn == nil {
		t.Fatalf("expected function myext/calculate to exist")
	}

	// Execute function
	ctx := context.Background()
	output, err := mgr.CallWithTrigger(ctx, "myext/calculate", map[string]interface{}{"a": 21}, "test")
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	resMap, ok := output.(map[string]interface{})
	if !ok || resMap["res"] != int64(42) || resMap["env"] != "bar_ext" {
		t.Fatalf("unexpected output: %v", output)
	}
}

func TestExternalWorkspaceRequire(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	extDir := filepath.Join(tmpDir, "external_require_project")
	os.MkdirAll(packagesDir, 0755)
	os.MkdirAll(extDir, 0755)

	// Create shared library in packagesDir/_shared
	sharedDir := filepath.Join(packagesDir, "_shared")
	os.MkdirAll(sharedDir, 0755)
	os.WriteFile(filepath.Join(sharedDir, "math.js"), []byte(`
function square(x) { return x * x; }
module.exports = { square: square };
`), 0644)

	// Create local helper inside external workspace
	os.WriteFile(filepath.Join(extDir, "helper.js"), []byte(`
function addTen(x) { return x + 10; }
module.exports = { addTen: addTen };
`), 0644)

	// Create main script requiring both local helper and _shared
	os.WriteFile(filepath.Join(extDir, "main.js"), []byte(`
var helper = require("./helper");
var sharedMath = require("_shared/math");

function main(params) {
    return {
        tenPlus: helper.addTen(params.val),
        squared: sharedMath.square(params.val)
    };
}
`), 0644)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("init db failed: %v", err)
	}
	defer db.Close()

	if err := db.AddExternalWorkspace("ext_req", extDir); err != nil {
		t.Fatalf("add external ws failed: %v", err)
	}

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload failed: %v", err)
	}

	ctx := context.Background()
	output, err := mgr.CallWithTrigger(ctx, "ext_req/main", map[string]interface{}{"val": 5}, "test")
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	resMap, ok := output.(map[string]interface{})
	if !ok || resMap["tenPlus"] != int64(15) || resMap["squared"] != int64(25) {
		t.Fatalf("unexpected require output: %v", output)
	}
}

func TestMissingExternalWorkspaceGracefulLoading(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	nonExistentDir := filepath.Join(tmpDir, "deleted_folder")
	os.MkdirAll(packagesDir, 0755)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("init db failed: %v", err)
	}
	defer db.Close()

	// Add external workspace pointing to non-existent dir
	if err := db.AddExternalWorkspace("missing_ws", nonExistentDir); err != nil {
		t.Fatalf("add external ws failed: %v", err)
	}

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload should not crash on missing external dir: %v", err)
	}

	pkg := mgr.GetPackage("missing_ws")
	if pkg == nil {
		t.Fatalf("expected missing_ws package to still be registered in memory")
	}
	if !pkg.IsExternal {
		t.Fatalf("expected pkg.IsExternal to be true")
	}
	if pkg.Status != "missing" {
		t.Fatalf("expected pkg.Status to be 'missing', got %q", pkg.Status)
	}
}

func TestWatcherExternalWorkspaceAddAndRemove(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	extDir1 := filepath.Join(tmpDir, "ext1")
	extDir2 := filepath.Join(tmpDir, "ext2")
	os.MkdirAll(packagesDir, 0755)
	os.MkdirAll(extDir1, 0755)
	os.MkdirAll(extDir2, 0755)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("init db failed: %v", err)
	}
	defer db.Close()

	_ = db.AddExternalWorkspace("ext1", extDir1)
	_ = db.AddExternalWorkspace("ext2", extDir2)

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload failed: %v", err)
	}

	if err := mgr.StartWatcher(); err != nil {
		t.Fatalf("StartWatcher failed: %v", err)
	}
	defer mgr.StopWatcher()

	// Now delete ext2 from DB and Reload
	_ = db.DeleteExternalWorkspace("ext2")
	if err := mgr.Reload(); err != nil {
		t.Fatalf("Reload after delete failed: %v", err)
	}

	// Verify ext2 is removed and ext1 remains
	if mgr.GetPackage("ext2") != nil {
		t.Fatalf("expected ext2 to be unlinked")
	}
	if mgr.GetPackage("ext1") == nil {
		t.Fatalf("expected ext1 to still be loaded")
	}
}
