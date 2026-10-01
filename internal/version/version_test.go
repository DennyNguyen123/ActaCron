package version

import (
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()
	if info.Version == "" {
		t.Errorf("Expected non-empty version, got empty")
	}
	if info.GitCommit == "" {
		t.Errorf("Expected non-empty git commit, got empty")
	}
	if info.BuildDate == "" {
		t.Errorf("Expected non-empty build date, got empty")
	}
}
