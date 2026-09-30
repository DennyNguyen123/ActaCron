package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/settings"
	"actacron/internal/storage"
)

func TestWorkspaceConfigAndSharedE2E(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "actacron.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	// 1. Setup Global Environment
	globalEnvPath := filepath.Join(tmpDir, ".env")
	os.WriteFile(globalEnvPath, []byte("ROOT_VAR=from_root\nOVERRIDDEN_VAR=root_val\n"), 0644)
	settings.New(db, globalEnvPath)

	packagesDir := filepath.Join(tmpDir, "packages")
	os.MkdirAll(packagesDir, 0755)

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	mgr.SetGlobalEnvPath(globalEnvPath)

	// 2. Initial reload should scaffold _shared
	if err := mgr.Reload(); err != nil {
		t.Fatalf("initial reload failed: %v", err)
	}

	// Verify _shared exists
	sharedIndex := filepath.Join(packagesDir, "_shared", "index.js")
	if _, err := os.Stat(sharedIndex); os.IsNotExist(err) {
		t.Fatalf("expected _shared/index.js to be scaffolded, but was not")
	}

	// 3. Create custom workspace with workspace.json and .env
	customPkgDir := filepath.Join(packagesDir, "dataproc")
	os.MkdirAll(customPkgDir, 0755)
	os.WriteFile(filepath.Join(customPkgDir, "workspace.json"), []byte(`{
		"timeout_seconds": 2,
		"description": "Data Processing Workspace"
	}`), 0644)
	os.WriteFile(filepath.Join(customPkgDir, ".env"), []byte("OVERRIDDEN_VAR=workspace_val\nWS_LOCAL=local_val\n"), 0644)

	// Create script utilizing env, shared, and require
	scriptCode := `/**
 * @name process_data
 * @cron * * * * *
 * @mcp true
 */
function main(params) {
	const utilsMod = require("_shared/utils");
	return {
		rootVar: env("ROOT_VAR"),
		overriddenVar: env("OVERRIDDEN_VAR"),
		wsLocal: env("WS_LOCAL"),
		uuidFromShared: shared.utils.uuid(),
		uuidFromRequire: utilsMod.uuid(),
		todayIso: shared.datetime.nowISO(),
		httpStatusOk: shared.constants.HTTP_STATUS.OK
	};
}`
	os.WriteFile(filepath.Join(customPkgDir, "process_data.js"), []byte(scriptCode), 0644)

	// Create long running script to test workspace timeout (2s)
	longScriptCode := `/**
 * @name long_runner
 */
function main() {
	while (true) {}
}`
	os.WriteFile(filepath.Join(customPkgDir, "long_runner.js"), []byte(longScriptCode), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload after adding scripts failed: %v", err)
	}

	// 4. Test normal execution
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := mgr.Call(ctx, "dataproc/process_data", nil)
	if err != nil {
		t.Fatalf("call process_data failed: %v", err)
	}

	resMap, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T: %v", res, res)
	}

	if resMap["rootVar"] != "from_root" {
		t.Errorf("expected rootVar=from_root, got %v", resMap["rootVar"])
	}
	if resMap["overriddenVar"] != "workspace_val" {
		t.Errorf("expected overriddenVar=workspace_val, got %v", resMap["overriddenVar"])
	}
	if resMap["wsLocal"] != "local_val" {
		t.Errorf("expected wsLocal=local_val, got %v", resMap["wsLocal"])
	}

	u1, _ := resMap["uuidFromShared"].(string)
	if len(u1) != 36 || !strings.Contains(u1, "-") {
		t.Errorf("invalid uuidFromShared: %v", u1)
	}

	u2, _ := resMap["uuidFromRequire"].(string)
	if len(u2) != 36 || !strings.Contains(u2, "-") {
		t.Errorf("invalid uuidFromRequire: %v", u2)
	}

	var statusOk float64
	switch v := resMap["httpStatusOk"].(type) {
	case int64:
		statusOk = float64(v)
	case float64:
		statusOk = v
	}
	if statusOk != 200 {
		t.Errorf("expected statusOk=200, got %v", resMap["httpStatusOk"])
	}

	// 5. Test workspace timeout (2s) enforcement
	start := time.Now()
	// Call without deadline so callWithTrigger applies effective timeout (2s)
	_, loopErr := mgr.Call(context.Background(), "dataproc/long_runner", nil)
	elapsed := time.Since(start)

	if loopErr == nil {
		t.Fatalf("expected timeout error for long_runner, got nil")
	}
	if !strings.Contains(loopErr.Error(), "timeout") && !strings.Contains(loopErr.Error(), "cancelled") {
		t.Errorf("expected error containing 'timeout' or 'cancelled', got: %v", loopErr)
	}
	// Verify elapsed time was around 2 seconds (not 30s)
	if elapsed > 4*time.Second {
		t.Errorf("expected long_runner to time out around 2s, but took %v", elapsed)
	}
}
