package api_test

import (
	"encoding/json"
	"fmt"
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

func TestSettingsSaveAndPartialUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_settings_api.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("storage.New: %v", err)
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

	// 1. Post General settings using frontend field names
	generalPayload := `{"port":9000,"timeout_seconds":45,"retention_days":14,"run_on_startup":true,"allow_shell":true}`
	resp, err := http.Post(ts.URL+"/api/settings", "application/json", strings.NewReader(generalPayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK saving general settings, got %v, err: %v", resp, err)
	}

	// Verify settings via GET
	resp, err = http.Get(ts.URL + "/api/settings")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/settings failed: %v", err)
	}
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if data["start_with_windows"] != true && data["run_on_startup"] != true {
		t.Errorf("expected start_with_windows/run_on_startup to be true, got %+v", data)
	}
	if data["allow_shell_exec"] != true && data["allow_shell"] != true {
		t.Errorf("expected allow_shell_exec/allow_shell to be true, got %+v", data)
	}
	if int(data["log_retention_days"].(float64)) != 14 && int(data["retention_days"].(float64)) != 14 {
		t.Errorf("expected retention days to be 14, got %+v", data)
	}

	// 2. Post UI settings (partial update)
	uiPayload := `{"theme_accent":"#38BDF8","density":"compact"}`
	resp, err = http.Post(ts.URL+"/api/settings", "application/json", strings.NewReader(uiPayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK saving UI settings, got %v, err: %v", resp, err)
	}

	// Verify general settings were NOT wiped out by partial UI update
	resp, err = http.Get(ts.URL + "/api/settings")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/settings failed: %v", err)
	}
	data = nil
	json.NewDecoder(resp.Body).Decode(&data)

	if data["theme_accent"] != "#38BDF8" {
		t.Errorf("expected theme_accent to be #38BDF8, got %v", data["theme_accent"])
	}
	if data["start_with_windows"] != true && data["run_on_startup"] != true {
		t.Errorf("general setting start_with_windows was wiped out by UI update! got %+v", data)
	}
	if data["allow_shell_exec"] != true && data["allow_shell"] != true {
		t.Errorf("general setting allow_shell_exec was wiped out by UI update! got %+v", data)
	}
}

func TestExternalWorkspaceAPI(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test_api_ext.db")
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

	tmpExtDir := t.TempDir()

	// 1. POST /api/workspace/external
	payload := fmt.Sprintf(`{"name":"ext_project","path":%q}`, tmpExtDir)
	resp, err := http.Post(ts.URL+"/api/workspace/external", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/workspace/external failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// 2. GET /api/workspace/external
	resp, err = http.Get(ts.URL + "/api/workspace/external")
	if err != nil {
		t.Fatalf("GET /api/workspace/external failed: %v", err)
	}
	var extList []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&extList)
	if len(extList) != 1 || extList[0]["name"] != "ext_project" {
		t.Fatalf("unexpected extList: %v", extList)
	}

	// Verify manager loaded it
	if mgr.GetPackage("ext_project") == nil {
		t.Fatalf("manager did not load external package")
	}

	// 3. DELETE /api/workspace/external?name=ext_project
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/workspace/external?name=ext_project", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/workspace/external failed: status=%d, err=%v", resp.StatusCode, err)
	}

	// Verify manager unloaded it
	if mgr.GetPackage("ext_project") != nil {
		t.Fatalf("manager did not unload deleted external package")
	}

	// Verify external directory still exists on disk (NEVER DELETED)
	if _, err := os.Stat(tmpExtDir); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: external directory was deleted on disk!")
	}
}

func TestAPIDeletePackage(t *testing.T) {
	pkgDir := t.TempDir()
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	mgr := manager.New(pkgDir, nil, db)
	_ = mgr.Reload()

	router := api.NewRouter(mgr, nil, db, nil, nil, nil)
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Create dummy package
	p1 := filepath.Join(pkgDir, "dummy_to_delete")
	os.MkdirAll(p1, 0755)
	os.WriteFile(filepath.Join(p1, "run.js"), []byte("// ok"), 0644)
	_ = mgr.Reload()

	// 1. DELETE without package query param -> 400
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/packages", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing package, got %d", resp.StatusCode)
	}

	// 2. DELETE _shared -> 400
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/packages?package=_shared", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on _shared, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/packages?package=_shared/", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on _shared/, got %d", resp.StatusCode)
	}

	// 3. DELETE valid package -> 200
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/packages?package=dummy_to_delete", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 deleting dummy_to_delete, got %d", resp.StatusCode)
	}

	// Verify package deleted on disk and in manager
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Fatalf("expected dummy_to_delete to be removed from disk")
	}
	if mgr.GetPackage("dummy_to_delete") != nil {
		t.Fatalf("expected dummy_to_delete to be removed from manager")
	}
}




