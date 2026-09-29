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
