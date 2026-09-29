package scheduler_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
	"actacron/internal/storage"
)

func TestScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_cron.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	// Create test script with cron metadata
	pkg := filepath.Join(tmpDir, "cronPkg")
	os.MkdirAll(pkg, 0755)
	os.WriteFile(filepath.Join(pkg, "job1.js"), []byte(`/**
 * @name job1
 * @cron * * * * *
 */
function main() {
    storage.set("ran", "true");
    return "ok";
}`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	cronSched := scheduler.New(mgr, db)
	cronSched.Start()
	defer cronSched.Stop()

	jobs := cronSched.GetJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 cron job registered, got %d", len(jobs))
	}
	if jobs[0].FunctionName != "job1" {
		t.Fatalf("expected job1, got %s", jobs[0].FunctionName)
	}

	// Test RunJobNow
	err = cronSched.RunJobNow("cronPkg/job1")
	if err != nil {
		t.Fatalf("RunJobNow failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	ran, err := db.GetKV("cronPkg", "ran")
	if err != nil || ran != "true" {
		t.Fatalf("expected KV ran=true, got %s, err: %v", ran, err)
	}

	// Test ToggleJob
	cronSched.ToggleJob("cronPkg/job1", false)
	jobs = cronSched.GetJobs()
	if jobs[0].IsEnabled {
		t.Fatalf("expected job to be disabled")
	}
}
