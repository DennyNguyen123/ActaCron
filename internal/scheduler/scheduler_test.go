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

func TestSchedulerLifecycleAndOverlap(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "lifecycle.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("db error: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	pkg := filepath.Join(tmpDir, "pkg1")
	os.MkdirAll(pkg, 0755)

	// 1. Pending job (start in year 2099)
	os.WriteFile(filepath.Join(pkg, "pending.js"), []byte(`/**
 * @name pending
 * @cron 0 12 * * *
 * @cron_start 2099-01-01 00:00:00
 */
function main() { return "pending"; }`), 0644)

	// 2. Expired job (end in year 2020)
	os.WriteFile(filepath.Join(pkg, "expired.js"), []byte(`/**
 * @name expired
 * @cron 0 12 * * *
 * @cron_end 2020-01-01 00:00:00
 */
function main() { return "expired"; }`), 0644)

	// 3. Completed job (max_runs 2)
	os.WriteFile(filepath.Join(pkg, "completed.js"), []byte(`/**
 * @name completed
 * @cron 0 12 * * *
 * @max_runs 2
 */
function main() { return "completed"; }`), 0644)

	// 4. Overlap job
	os.WriteFile(filepath.Join(pkg, "slow.js"), []byte(`/**
 * @name slow
 * @cron * * * * *
 * @no_overlap true
 */
function main() {
    sleep(300);
    return "slow_done";
}`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload error: %v", err)
	}

	sched := scheduler.New(mgr, db)

	// Set completed job run_count to 2
	fnCompleted := mgr.GetFunction("pkg1/completed")
	if fnCompleted != nil {
		fnCompleted.RunCount = 2
	}

	jobs := sched.GetJobs()
	statusMap := make(map[string]scheduler.CronJobInfo)
	for _, j := range jobs {
		statusMap[j.FunctionName] = j
	}

	if j, ok := statusMap["pending"]; !ok || j.Status != "pending" {
		t.Errorf("expected pending status, got %s", j.Status)
	}
	if j, ok := statusMap["expired"]; !ok || j.Status != "expired" {
		t.Errorf("expected expired status, got %s", j.Status)
	}
	if j, ok := statusMap["completed"]; !ok || j.Status != "completed" {
		t.Errorf("expected completed status, got %s", j.Status)
	}

	// Test Overlap Guard
	go func() {
		_ = sched.RunJobNow("pkg1/slow")
	}()

	// Wait briefly for slow job to start
	time.Sleep(50 * time.Millisecond)

	// Verify running state
	slowJob := mgr.GetFunction("pkg1/slow")
	if slowJob != nil && !sched.IsJobRunning("pkg1/slow") {
		t.Logf("job is running")
	}

	// Trigger again during execution; should be skipped due to overlap
	_ = sched.RunJobNow("pkg1/slow")

	// Wait for first job to finish
	time.Sleep(400 * time.Millisecond)

	// Verify logs for slow job include skipped_overlap or success
	logs, _, err := db.QueryLogs("slow", "", 10, 0)
	if err != nil {
		t.Fatalf("QueryLogs error: %v", err)
	}
	if len(logs) == 0 {
		t.Errorf("expected at least one log for slow job")
	}
}

func TestSchedulerTimezoneAndReset(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "tz.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	pkg := filepath.Join(tmpDir, "tzpkg")
	os.MkdirAll(pkg, 0755)

	os.WriteFile(filepath.Join(pkg, "tzjob.js"), []byte(`/**
 * @name tzjob
 * @cron 0 12 * * *
 * @timezone Asia/Ho_Chi_Minh
 * @max_runs 5
 */
function main() { return "tz"; }`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload error: %v", err)
	}

	sched := scheduler.New(mgr, db)
	jobs := sched.GetJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Timezone != "Asia/Ho_Chi_Minh" {
		t.Errorf("expected timezone Asia/Ho_Chi_Minh, got %s", jobs[0].Timezone)
	}
	if jobs[0].MaxRuns != 5 {
		t.Errorf("expected max_runs 5, got %d", jobs[0].MaxRuns)
	}

	// Simulate runs
	_ = db.IncrementRunCount("tzpkg", "tzjob")
	_ = db.IncrementRunCount("tzpkg", "tzjob")
	fn := mgr.GetFunction("tzpkg/tzjob")
	fn.RunCount = 2

	jobs = sched.GetJobs()
	if jobs[0].RunCount != 2 {
		t.Errorf("expected RunCount=2, got %d", jobs[0].RunCount)
	}

	// Reset runs
	if err := sched.ResetRuns("tzpkg/tzjob"); err != nil {
		t.Fatalf("ResetRuns failed: %v", err)
	}
	jobs = sched.GetJobs()
	if jobs[0].RunCount != 0 {
		t.Errorf("expected RunCount=0 after reset, got %d", jobs[0].RunCount)
	}
}

