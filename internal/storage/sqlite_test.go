package storage_test

import (
	"os"
	"testing"
	"time"

	"actacron/internal/domain"
	"actacron/internal/storage"
)

func TestSQLite(t *testing.T) {
	tmpFile := "test_actacron.db"
	defer os.Remove(tmpFile)

	db, err := storage.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Test Log Insert & Query
	log := domain.ExecutionLog{
		ExecutionID:  "exec-1",
		PackageName:  "dev-utils",
		FunctionName: "format_json",
		TriggerType:  "cron",
		Status:       "success",
		InputParams:  `{"data":1}`,
		OutputData:   `{"result":true}`,
		DurationMs:   42,
		CreatedAt:    time.Now(),
	}

	if err := db.InsertLog(&log); err != nil {
		t.Fatalf("insert log failed: %v", err)
	}

	logs, total, err := db.QueryLogs("format_json", "", 10, 0)
	if err != nil {
		t.Fatalf("query logs failed: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("expected 1 log, got total=%d len=%d", total, len(logs))
	}

	// Test KV Store
	if err := db.SetKV("dev-utils", "cursor", "100"); err != nil {
		t.Fatalf("set kv failed: %v", err)
	}
	val, err := db.GetKV("dev-utils", "cursor")
	if err != nil || val != "100" {
		t.Fatalf("expected '100', got '%s', err: %v", val, err)
	}

	// Test Settings Store
	if err := db.SetSetting("theme_accent", "#22C55E"); err != nil {
		t.Fatalf("set setting failed: %v", err)
	}
	accent, err := db.GetSetting("theme_accent")
	if err != nil || accent != "#22C55E" {
		t.Fatalf("expected '#22C55E', got '%s', err: %v", accent, err)
	}
}

func TestRunCountLifecycle(t *testing.T) {
	tmpFile := "test_run_count.db"
	defer os.Remove(tmpFile)

	db, err := storage.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Initial run count should be 0
	count, err := db.GetRunCount("cronPkg", "job1")
	if err != nil {
		t.Fatalf("GetRunCount error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// Increment 3 times
	if err := db.IncrementRunCount("cronPkg", "job1"); err != nil {
		t.Fatalf("IncrementRunCount failed: %v", err)
	}
	if err := db.IncrementRunCount("cronPkg", "job1"); err != nil {
		t.Fatalf("IncrementRunCount failed: %v", err)
	}
	if err := db.IncrementRunCount("cronPkg", "job1"); err != nil {
		t.Fatalf("IncrementRunCount failed: %v", err)
	}

	count, err = db.GetRunCount("cronPkg", "job1")
	if err != nil || count != 3 {
		t.Errorf("expected 3, got %d, err: %v", count, err)
	}

	// Reset run count
	if err := db.ResetRunCount("cronPkg", "job1"); err != nil {
		t.Fatalf("ResetRunCount failed: %v", err)
	}

	count, err = db.GetRunCount("cronPkg", "job1")
	if err != nil || count != 0 {
		t.Errorf("expected 0 after reset, got %d, err: %v", count, err)
	}
}

func TestExternalWorkspacesCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "actacron-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := tmpDir + "/test.db"
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// 1. Add external workspace
	err = db.AddExternalWorkspace("my_external", "D:/External/Scripts")
	if err != nil {
		t.Fatalf("expected no error adding external workspace, got: %v", err)
	}

	// 2. Get external workspace
	ws, err := db.GetExternalWorkspace("my_external")
	if err != nil || ws == nil {
		t.Fatalf("failed to get external workspace: %v", err)
	}
	if ws.Name != "my_external" || ws.Path != "D:/External/Scripts" {
		t.Fatalf("unexpected ws data: %+v", ws)
	}

	// 3. List external workspaces
	list, err := db.ListExternalWorkspaces()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 external workspace, got %d, err: %v", len(list), err)
	}

	// 4. Delete external workspace
	err = db.DeleteExternalWorkspace("my_external")
	if err != nil {
		t.Fatalf("failed to delete external workspace: %v", err)
	}

	listAfter, err := db.ListExternalWorkspaces()
	if err != nil || len(listAfter) != 0 {
		t.Fatalf("expected 0 external workspaces after delete, got %d", len(listAfter))
	}
}

