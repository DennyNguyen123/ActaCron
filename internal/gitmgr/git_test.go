package gitmgr_test

import (
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/gitmgr"
	"github.com/go-git/go-git/v5"
)

func TestGitStatus(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	_, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	svc := gitmgr.New()
	status, err := svc.GetStatus(repoDir)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status != "clean" {
		t.Fatalf("expected clean, got %s", status)
	}

	// Add file
	os.WriteFile(filepath.Join(repoDir, "test.js"), []byte("console.log(1)"), 0644)
	status, _ = svc.GetStatus(repoDir)
	if status != "modified" {
		t.Fatalf("expected modified, got %s", status)
	}
}
