package tray

import (
	"os/exec"
	"runtime"
)

// OpenDashboard opens the ActaCron dashboard in the system default browser.
func OpenDashboard(url string) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "start", "", url)
		if err := cmd.Start(); err == nil {
			return nil
		}
		// Fallback to rundll32
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}

	// Non-Windows fallbacks
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}
