package updater

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCheckForUpdate(t *testing.T) {
	mockRelease := githubRelease{
		TagName:     "v1.1.0",
		Body:        "## Changelog\n- In-app updater added",
		PublishedAt: time.Now(),
		Assets: []githubAsset{
			{
				Name:               "actacron-v1.1.0-windows-amd64.zip",
				BrowserDownloadURL: "https://example.com/actacron-v1.1.0-windows-amd64.zip",
			},
			{
				Name:               "actacron-v1.1.0-linux-amd64.tar.gz",
				BrowserDownloadURL: "https://example.com/actacron-v1.1.0-linux-amd64.tar.gz",
			},
			{
				Name:               "actacron-v1.1.0-darwin-arm64.tar.gz",
				BrowserDownloadURL: "https://example.com/actacron-v1.1.0-darwin-arm64.tar.gz",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing User-Agent", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	u := NewClient(server.Client())
	u.baseURL = server.URL // override for testing

	info, err := u.CheckForUpdate("test/repo", "1.0.0")
	if err != nil {
		t.Fatalf("CheckForUpdate failed: %v", err)
	}

	if !info.HasUpdate {
		t.Errorf("Expected HasUpdate to be true")
	}
	if info.TagName != "v1.1.0" {
		t.Errorf("Expected TagName v1.1.0, got %s", info.TagName)
	}
	if info.Notes != mockRelease.Body {
		t.Errorf("Expected notes %q, got %q", mockRelease.Body, info.Notes)
	}

	// Verify asset matching for current OS/arch if in test
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		if info.AssetName != "actacron-v1.1.0-windows-amd64.zip" {
			t.Errorf("Expected windows-amd64 asset, got %s", info.AssetName)
		}
	}
}

func TestApplyUpdateZip(t *testing.T) {
	tempDir := t.TempDir()
	curExe := filepath.Join(tempDir, "actacron.exe")
	if err := os.WriteFile(curExe, []byte("version 1.0"), 0755); err != nil {
		t.Fatalf("failed to create dummy exe: %v", err)
	}

	// Create a zip with the new binary
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("actacron.exe")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	_, _ = w.Write([]byte("version 1.1"))
	_ = zw.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer server.Close()

	u := NewClient(server.Client())
	if err := u.ApplyUpdate(server.URL, curExe); err != nil {
		t.Fatalf("ApplyUpdate failed: %v", err)
	}

	// Verify that current executable has the new content
	data, err := os.ReadFile(curExe)
	if err != nil {
		t.Fatalf("failed to read updated exe: %v", err)
	}
	if string(data) != "version 1.1" {
		t.Errorf("Expected 'version 1.1', got %q", string(data))
	}

	// Verify that old executable exists as .old
	oldData, err := os.ReadFile(curExe + ".old")
	if err != nil {
		t.Fatalf("expected .old file to exist: %v", err)
	}
	if string(oldData) != "version 1.0" {
		t.Errorf("Expected .old to contain 'version 1.0', got %q", string(oldData))
	}

	// Test cleanup
	CleanupOldBinary(curExe)
	if _, err := os.Stat(curExe + ".old"); !os.IsNotExist(err) {
		t.Errorf("Expected .old file to be removed after cleanup")
	}
}
