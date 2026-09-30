package manager_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/storage"
)

func TestSharedLibraryScaffoldAndInjection(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_shared.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 5, false)
	mgr := manager.New(tmpDir, runner, db)

	// Reload should auto-scaffold packages/_shared
	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	sharedDir := filepath.Join(tmpDir, "_shared")
	for _, expectedFile := range []string{"datetime.js", "notify.js", "utils.js", "constants.js", "index.js"} {
		fPath := filepath.Join(sharedDir, expectedFile)
		if _, err := os.Stat(fPath); os.IsNotExist(err) {
			t.Errorf("expected scaffolded file %s does not exist", fPath)
		}
	}

	// Create a user package calling shared
	userPkg := filepath.Join(tmpDir, "userPkg")
	os.MkdirAll(userPkg, 0755)

	scriptCode := `
		function main() {
			const utilsMod = require("_shared/utils");
			return {
				directUuid: shared.utils ? shared.utils.uuid() : null,
				requiredUuid: utilsMod ? utilsMod.uuid() : null,
				nowIso: shared.datetime ? shared.datetime.nowISO() : null,
				httpOk: shared.constants ? shared.constants.HTTP_STATUS.OK : null
			};
		}
	`
	os.WriteFile(filepath.Join(userPkg, "test_shared.js"), []byte(scriptCode), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload after script failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := mgr.Call(ctx, "userPkg/test_shared", nil)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	resMap, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T: %v", res, res)
	}

	directUuid, _ := resMap["directUuid"].(string)
	if len(directUuid) != 36 || !strings.Contains(directUuid, "-") {
		t.Errorf("expected valid directUuid, got %v", directUuid)
	}

	requiredUuid, _ := resMap["requiredUuid"].(string)
	if len(requiredUuid) != 36 || !strings.Contains(requiredUuid, "-") {
		t.Errorf("expected valid requiredUuid, got %v", requiredUuid)
	}

	nowIso, _ := resMap["nowIso"].(string)
	if len(nowIso) < 10 {
		t.Errorf("expected valid nowIso, got %v", nowIso)
	}

	var httpOk float64
	switch v := resMap["httpOk"].(type) {
	case int64:
		httpOk = float64(v)
	case float64:
		httpOk = v
	}
	if httpOk != 200 {
		t.Errorf("expected httpOk=200, got %v", resMap["httpOk"])
	}
}

func TestSharedRequireSecurityTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_sec.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 5, false)
	mgr := manager.New(tmpDir, runner, db)

	// Create a secret file outside packages
	secretFile := filepath.Join(filepath.Dir(tmpDir), "secret.js")
	os.WriteFile(secretFile, []byte(`module.exports = { secret: "123" };`), 0644)
	defer os.Remove(secretFile)

	userPkg := filepath.Join(tmpDir, "hackerPkg")
	os.MkdirAll(userPkg, 0755)

	scriptCode := `
		function main() {
			return require("../../secret.js");
		}
	`
	os.WriteFile(filepath.Join(userPkg, "exploit.js"), []byte(scriptCode), 0644)

	mgr.Reload()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := mgr.Call(ctx, "hackerPkg/exploit", nil)
	if err == nil {
		t.Fatalf("expected security error when traversing outside packages, got nil")
	}
}
