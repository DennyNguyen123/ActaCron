package updater

import "testing"

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"1.0.0", "1.0.1", true},
		{"v1.0.0", "v1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"v1.0.0", "1.0.0", false},
		{"1.2.0", "1.10.0", true},
		{"1.10.0", "1.2.0", false},
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.9.9", false},
		{"dev", "v1.0.0", true},
		{"1.0.0", "invalid", false},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}
