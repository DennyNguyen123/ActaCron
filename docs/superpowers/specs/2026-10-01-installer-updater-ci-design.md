# ActaCron Windows Installer, In-App Auto-Update & Multi-Platform CI/CD Specification

**Date:** 2026-10-01  
**Status:** Approved  
**Author:** DennyNguyen & Antigravity  

---

## 1. Overview & Goals

ActaCron is a standalone Windows daemon and developer hub with an embedded Dark OLED Web Dashboard, Goja JavaScript runtime, Git manager, Cron scheduler, and MCP server.

This specification details:
1. **Cross-Platform Decoupling**: Decouple the Windows System Tray implementation from non-Windows platforms so ActaCron can be built cleanly with `CGO_ENABLED=0` across Windows, Linux, and macOS.
2. **Version Management**: Centralized version, git commit, and build timestamp injection via Go `-ldflags`.
3. **In-App Auto-Update Subsystem**:
   - Backend service querying GitHub Releases API (`DennyNguyen123/ActaCron`).
   - REST API endpoints for checking and applying updates.
   - Safe in-place executable replacement on Windows (rename running executable to `.old`, write new binary, restart).
   - Embedded Web Dashboard UI with Update notifications and Changelog display.
4. **Inno Setup Windows Installer**: User-level (`%LOCALAPPDATA%\Programs\ActaCron`) installer requiring no Administrator/UAC elevation, with Desktop/Start Menu shortcuts, Auto-start option, and preservation of user data (`actacron.db` and `packages/`).
5. **GitHub Actions Matrix Release Pipeline**: Automated build and release workflow on Git tag (`v*`) generating installers, portable archives, and SHA256 checksums.

---

## 2. Architecture & Design

### 2.1 Cross-Platform Build Tags (`internal/tray`)

- `internal/tray/tray_windows.go` (`//go:build windows`):
  - Uses `github.com/getlantern/systray` for the native Win32 system tray menu and message loop.
- `internal/tray/tray_other.go` (`//go:build !windows`):
  - Provides a headless implementation of `TrayHandler`.
  - `Run()` waits on OS termination signals (`SIGINT`, `SIGTERM`) and calls `onExit()`.
- Result: `CGO_ENABLED=0` builds compile seamlessly for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`.

### 2.2 Version Package (`internal/version`)

```go
package version

var (
    Version   = "dev"
    GitCommit = "none"
    BuildDate = "unknown"
)
```

Injected during compilation:
`-ldflags "-s -w -X 'actacron/internal/version.Version=v1.0.1' -X 'actacron/internal/version.GitCommit=abc123' -X 'actacron/internal/version.BuildDate=2026-10-01T09:00:00Z'"`

### 2.3 Updater Subsystem (`internal/updater`)

- **GitHub Release Query**:
  - Endpoint: `https://api.github.com/repos/DennyNguyen123/ActaCron/releases/latest`
  - Header: `User-Agent: ActaCron-Updater/<Version>`
  - Evaluates semantic version (`vA.B.C` vs `vX.Y.Z`).
  - Matches platform assets:
    - Windows: `actacron-windows-amd64.zip` or executable.
- **In-Place Binary Replacement (Windows)**:
  1. Download latest archive/binary to temporary location `actacron.exe.download`.
  2. Locate currently running executable path via `os.Executable()`.
  3. Rename running executable: `actacron.exe` ➔ `actacron.exe.old`.
  4. Move/rename `actacron.exe.download` ➔ `actacron.exe`.
  5. Spawn new process `actacron.exe` with same arguments.
  6. Exit current process (`os.Exit(0)`).
  7. On next boot, `main.go` cleans up any existing `*.old` files silently.

### 2.4 REST API Endpoints

- `GET /api/version`
  - Returns: `{ "version": "v1.0.0", "commit": "...", "build_date": "..." }`
- `GET /api/update/check`
  - Returns:
    ```json
    {
      "has_update": true,
      "current_version": "v1.0.0",
      "latest_version": "v1.0.1",
      "release_notes": "...",
      "published_at": "2026-10-01T10:00:00Z",
      "download_url": "https://..."
    }
    ```
- `POST /api/update/apply`
  - Body: `{ "download_url": "https://..." }` (optional override)
  - Performs background download, swap, and triggers graceful restart.

### 2.5 Web UI Integration (`web/`)

- Settings tab:
  - Section "System Information": Displays current version and build commit.
  - "Check for Updates" button with loading spinner.
  - Banner/Modal if update is found:
    - Shows version difference, release date, release notes/changelog.
    - "Update & Restart" button with live download progress.

### 2.6 Windows Inno Setup Installer (`scripts/installer.iss`)

- **Install Directory**: `{localappdata}\Programs\ActaCron` (`PrivilegesRequired=lowest`).
- **Shortcuts**:
  - `{userprograms}\ActaCron.lnk`
  - `{userdesktop}\ActaCron.lnk`
- **User Data Safety**:
  - `packages/` and `actacron.db` are never deleted during upgrade.
  - Uninstaller provides option to retain user data.
- **Auto-Close**:
  - `CloseApplications=yes` to automatically close running ActaCron instance before setup.

### 2.7 GitHub Actions Matrix (`.github/workflows/release.yml`)

- Matrix strategy across OS runners:
  - `windows-latest`: `windows/amd64` (produces `ActaCron-Setup.exe` via Inno Setup CLI `iscc`, and `.zip`), `windows/arm64` (.zip).
  - `ubuntu-latest`: `linux/amd64`, `linux/arm64` (.tar.gz).
  - `macos-latest`: `darwin/amd64`, `darwin/arm64` (.tar.gz).
- Auto-generates `checksums-sha256.txt`.
- Creates GitHub Release with draft/publish toggle and auto-generated release notes.
