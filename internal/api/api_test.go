package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"actacron/internal/api"
	"actacron/internal/engine"
	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
	"actacron/internal/settings"
	"actacron/internal/storage"
	"actacron/web"
)

func TestAPIEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_api.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)
	sched := scheduler.New(mgr, db)
	gitSvc := gitmgr.New()
	settingsSvc := settings.New(db, filepath.Join(tmpDir, ".env"))

	router := api.NewRouter(mgr, sched, db, gitSvc, settingsSvc, nil)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// 1. Health check
	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/health, got %v, err: %v", resp, err)
	}

	var healthRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&healthRes)
	if healthRes["status"] != "ok" {
		t.Fatalf("expected status: ok, got %v", healthRes)
	}

	// 2. Settings endpoint
	resp, err = http.Get(ts.URL + "/api/settings")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/settings, got %v, err: %v", resp, err)
	}

	// 3. Static assets serving
	routerWithStatic := api.NewRouter(mgr, sched, db, gitSvc, settingsSvc, web.Assets)
	tsStatic := httptest.NewServer(routerWithStatic)
	defer tsStatic.Close()

	resp, err = http.Get(tsStatic.URL + "/")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for static index.html, got %v, err: %v", resp, err)
	}

	// 4. Save and Delete function API
	savePayload := `{"package":"testpkg","filename":"hello.js","code":"function main() { return 42; }"}`
	resp, err = http.Post(ts.URL+"/api/functions/save", "application/json", strings.NewReader(savePayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for save, got %v, err: %v", resp, err)
	}

	delPayload := `{"package":"testpkg","filename":"hello.js"}`
	resp, err = http.Post(ts.URL+"/api/functions/delete", "application/json", strings.NewReader(delPayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete, got %v, err: %v", resp, err)
	}
}

func TestWorkspaceConfigAPI(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_api_ws.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)
	sched := scheduler.New(mgr, db)
	gitSvc := gitmgr.New()
	settingsSvc := settings.New(db, filepath.Join(tmpDir, ".env"))

	router := api.NewRouter(mgr, sched, db, gitSvc, settingsSvc, nil)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// 1. POST /api/workspace/config to save config
	savePayload := `{
		"package": "analytics",
		"timeout_seconds": 75,
		"description": "Analytics Workspace",
		"env": {
			"API_URL": "https://api.example.com",
			"TOKEN": "secret_123"
		}
	}`
	resp, err := http.Post(ts.URL+"/api/workspace/config", "application/json", strings.NewReader(savePayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for POST /api/workspace/config, got status %v, err %v", resp.StatusCode, err)
	}

	// 2. GET /api/workspace/config?package=analytics
	resp, err = http.Get(ts.URL + "/api/workspace/config?package=analytics")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/workspace/config, got status %v, err %v", resp.StatusCode, err)
	}

	var getRes struct {
		Package        string            `json:"package"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		Description    string            `json:"description"`
		Env            map[string]string `json:"env"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&getRes); err != nil {
		t.Fatalf("failed decoding GET response: %v", err)
	}

	if getRes.Package != "analytics" || getRes.TimeoutSeconds != 75 || getRes.Description != "Analytics Workspace" {
		t.Errorf("unexpected config result: %+v", getRes)
	}
	if getRes.Env["API_URL"] != "https://api.example.com" || getRes.Env["TOKEN"] != "secret_123" {
		t.Errorf("unexpected env result: %+v", getRes.Env)
	}
}

func TestCronAPI(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_api_cron.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	// Create test script with comprehensive cron annotations
	pkgDir := filepath.Join(tmpDir, "cronpkg")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "sample.js"), []byte(`/**
 * @name sample
 * @cron 0 12 * * *
 * @cron_start 2099-01-01 00:00:00
 * @timezone Asia/Tokyo
 * @max_runs 10
 */
function main() { return "hello"; }`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	sched := scheduler.New(mgr, db)
	gitSvc := gitmgr.New()
	settingsSvc := settings.New(db, filepath.Join(tmpDir, ".env"))

	router := api.NewRouter(mgr, sched, db, gitSvc, settingsSvc, nil)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// 1. GET /api/cron should return enriched job details
	resp, err := http.Get(ts.URL + "/api/cron")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/cron, got status %v, err %v", resp.StatusCode, err)
	}

	var jobs []scheduler.CronJobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		t.Fatalf("failed decoding GET /api/cron: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 cron job, got %d", len(jobs))
	}
	job := jobs[0]
	if job.FunctionName != "sample" {
		t.Errorf("expected sample, got %s", job.FunctionName)
	}
	if job.Status != "pending" {
		t.Errorf("expected pending, got %s", job.Status)
	}
	if job.Timezone != "Asia/Tokyo" {
		t.Errorf("expected Asia/Tokyo, got %s", job.Timezone)
	}
	if job.CronStart != "2099-01-01 00:00:00" {
		t.Errorf("expected cron_start 2099-01-01 00:00:00, got %s", job.CronStart)
	}
	if job.MaxRuns != 10 {
		t.Errorf("expected max_runs 10, got %d", job.MaxRuns)
	}
	if job.RunCount != 0 {
		t.Errorf("expected run_count 0, got %d", job.RunCount)
	}

	// 2. Increment run count in storage & memory
	_ = db.IncrementRunCount("cronpkg", "sample")
	fn := mgr.GetFunction("cronpkg/sample")
	fn.RunCount = 1

	// 3. POST /api/cron with action "reset_runs"
	resetPayload := `{"action":"reset_runs","target":"cronpkg/sample"}`
	resp, err = http.Post(ts.URL+"/api/cron", "application/json", strings.NewReader(resetPayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for reset_runs, got status %v, err %v", resp.StatusCode, err)
	}

	var resetRes map[string]string
	json.NewDecoder(resp.Body).Decode(&resetRes)
	if resetRes["status"] != "reset" {
		t.Errorf("expected status 'reset', got %s", resetRes["status"])
	}

	// 4. Verify run_count is 0 via GET /api/cron
	resp, err = http.Get(ts.URL + "/api/cron")
	if err != nil {
		t.Fatalf("GET /api/cron error: %v", err)
	}
	jobs = nil
	json.NewDecoder(resp.Body).Decode(&jobs)
	if len(jobs) > 0 && jobs[0].RunCount != 0 {
		t.Errorf("expected RunCount=0 after reset, got %d", jobs[0].RunCount)
	}

	// 5. POST /api/cron with action "toggle" to pause
	togglePayload := `{"action":"toggle","target":"cronpkg/sample","enable":false}`
	resp, err = http.Post(ts.URL+"/api/cron", "application/json", strings.NewReader(togglePayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for toggle, got status %v, err %v", resp.StatusCode, err)
	}

	resp, err = http.Get(ts.URL + "/api/cron")
	jobs = nil
	json.NewDecoder(resp.Body).Decode(&jobs)
	if len(jobs) > 0 && jobs[0].Status != "paused" {
		t.Errorf("expected status 'paused' after toggle, got %s", jobs[0].Status)
	}
}



