package tray

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenDashboard opens the ActaCron dashboard in Edge App Mode (--app=url)
// or falls back to the system default browser.
func OpenDashboard(url string) error {
	if runtime.GOOS == "windows" {
		edgePaths := []string{
			filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "Application", "msedge.exe"),
		}

		for _, path := range edgePaths {
			if path == "" {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				cmd := exec.Command(path, "--app="+url)
				return cmd.Start()
			}
		}

		// Fallback on Windows
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		return cmd.Start()
	}

	// Non-Windows fallbacks
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}
