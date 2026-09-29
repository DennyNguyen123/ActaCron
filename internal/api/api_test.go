package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"actacron/internal/api"
	"actacron/internal/engine"
	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
	"actacron/internal/settings"
	"actacron/internal/storage"
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
}
