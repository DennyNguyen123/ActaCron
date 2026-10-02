package tray

import (
	"os/exec"
	"runtime"
	"strings"
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

// OpenFolder opens the specified directory in the system file explorer.
func OpenFolder(dirPath string) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("explorer", dirPath)
		return cmd.Start()
	}
	if runtime.GOOS == "darwin" {
		return exec.Command("open", dirPath).Start()
	}
	return exec.Command("xdg-open", dirPath).Start()
}

// PickFolder launches a native folder selection dialog and returns the selected path.
func PickFolder() (string, error) {
	if runtime.GOOS == "windows" {
		cmdStr := `Add-Type -AssemblyName System.Windows.Forms; $f = New-Object System.Windows.Forms.FolderBrowserDialog; $f.Description = 'Select ActaCron Workspace Folder'; $f.ShowNewFolderButton = $true; if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $f.SelectedPath }`
		out, err := exec.Command("powershell", "-NoProfile", "-Command", cmdStr).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Select ActaCron Workspace Folder")`).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
	// Linux fallback
	out, err := exec.Command("zenity", "--file-selection", "--directory", "--title=Select ActaCron Workspace Folder").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

