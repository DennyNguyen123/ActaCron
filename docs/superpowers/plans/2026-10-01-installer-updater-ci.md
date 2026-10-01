# Windows Installer, In-App Auto-Update & Multi-Platform CI/CD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement full cross-platform compilation support (Windows, Linux, macOS), centralized versioning, an in-app updater querying GitHub Releases with in-place executable replacement on Windows, an Inno Setup installer script, and a GitHub Actions Matrix CI/CD release workflow.

**Architecture:** 
1. Decouple Windows `systray` using Go build tags (`//go:build windows` vs `//go:build !windows`), allowing pure `CGO_ENABLED=0` builds across all target OSes.
2. Centralize version info in `internal/version` injected via `-ldflags`.
3. Create an `updater` package in `internal/updater` that fetches release metadata from GitHub Releases API, extracts matching assets, safely swaps binaries on Windows using rename semantics (`.exe` -> `.exe.old`), and restarts.
4. Expose updater endpoints (`/api/version`, `/api/update/check`, `/api/update/apply`) and render an update banner/modal in the embedded Web UI.
5. Provide an Inno Setup script (`scripts/installer.iss`) for User-Level installation in `%LOCALAPPDATA%\Programs\ActaCron`.
6. Configure `.github/workflows/release.yml` with a matrix runner strategy across Windows, Linux, and macOS to automate builds and GitHub Releases.

**Tech Stack:** Go (pure Go 1.26, CGO_ENABLED=0), modernc.org/sqlite, Inno Setup (ISCC), GitHub Actions, Vanilla JS / CSS embedded Web UI.

**Spec:** [docs/superpowers/specs/2026-10-01-installer-updater-ci-design.md](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-10-01-installer-updater-ci-design.md)

## Global Constraints

- Must compile with `CGO_ENABLED=0` across Windows, Linux, and macOS.
- Installer must not require Administrator privileges (`PrivilegesRequired=lowest`).
- Installer and in-app update must never overwrite or delete user scripts in `packages/` or `actacron.db`.
- GitHub API client must include `User-Agent: ActaCron-Updater/<Version>`.
- All tests must pass: `go test ./...`.

## Review Focus

1. Non-Windows builds: Verify `GOOS=linux go build` and `GOOS=darwin go build` succeed without CGO.
2. In-place binary replacement: Verify executable rename works without file lock errors on Windows.
3. Version comparison: Handle `v` prefix gracefully (`v1.0.1` vs `1.0.1` vs `1.0.0`).
4. GitHub API rate limits / network timeouts: Gracefully return structured error responses to the UI.
5. Inno Setup installer: Ensure existing settings/packages are preserved on upgrade.

---

### Task 1: Cross-Platform Decoupling for System Tray

**Files:**
- Create: `internal/tray/tray_windows.go`
- Create: `internal/tray/tray_other.go`
- Remove / Refactor: `internal/tray/tray.go`
- Test: Verify build on Windows and cross-compile to Linux

**Interfaces:**
- Produces: `TrayHandler` struct with `Run()` method that works on all OS targets.

- [ ] **Step 1: Create `internal/tray/tray_windows.go`**

Move the Windows-specific systray code from `internal/tray/tray.go` into `internal/tray/tray_windows.go` with build tag `//go:build windows`:

```go
//go:build windows

package tray

import (
	"log"

	"github.com/getlantern/systray"

	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
)

type TrayHandler struct {
	dashboardURL string
	sched        *scheduler.Scheduler
	gitSvc       *gitmgr.GitService
	mgr          *manager.Manager
	onExit       func()
}

func New(dashboardURL string, sched *scheduler.Scheduler, gitSvc *gitmgr.GitService, mgr *manager.Manager, onExit func()) *TrayHandler {
	return &TrayHandler{
		dashboardURL: dashboardURL,
		sched:        sched,
		gitSvc:       gitSvc,
		mgr:          mgr,
		onExit:       onExit,
	}
}

func (th *TrayHandler) Run() {
	systray.Run(th.onReady, th.onExitInternal)
}

func (th *TrayHandler) onReady() {
	systray.SetIcon(generateDefaultIcon())
	systray.SetTitle("ActaCron")
	systray.SetTooltip("ActaCron - Dynamic JS Engine & Cron Hub")

	mOpen := systray.AddMenuItem("Open Dashboard", "Open ActaCron Edge Web UI")
	mOpenFolder := systray.AddMenuItem("Open Packages Folder", "Open packages workspace directory in File Explorer")
	mSync := systray.AddMenuItem("Sync All Git", "Pull and sync all Git packages")

	var mPause *systray.MenuItem
	if th.sched != nil {
		mPause = systray.AddMenuItem("Pause Cron", "Pause/Resume cron schedules")
	}

	systray.AddSeparator()
	mExit := systray.AddMenuItem("Exit ActaCron", "Shut down background daemon")

	cronPaused := false

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				log.Printf("[Tray] Open Dashboard clicked: launching %s", th.dashboardURL)
				if err := OpenDashboard(th.dashboardURL); err != nil {
					log.Printf("[Tray] Failed to open dashboard: %v", err)
				}

			case <-mOpenFolder.ClickedCh:
				if th.mgr != nil {
					log.Printf("[Tray] Opening packages folder: %s", th.mgr.PackagesDir())
					if err := OpenFolder(th.mgr.PackagesDir()); err != nil {
						log.Printf("[Tray] Failed to open folder: %v", err)
					}
				}

			case <-mSync.ClickedCh:
				log.Println("[Tray] Syncing all packages...")
				if th.mgr != nil && th.gitSvc != nil {
					for _, pkg := range th.mgr.ListPackages() {
						if _, err := th.gitSvc.Pull(pkg.Path, ""); err != nil {
							log.Printf("[Tray] Failed sync for %s: %v", pkg.Name, err)
						}
					}
					th.mgr.Reload()
				}

			case <-mExit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	if mPause != nil {
		go func() {
			for range mPause.ClickedCh {
				cronPaused = !cronPaused
				if cronPaused {
					th.sched.Stop()
					mPause.SetTitle("Resume Cron")
					systray.SetTooltip("ActaCron (Cron Paused)")
				} else {
					th.sched.Start()
					mPause.SetTitle("Pause Cron")
					systray.SetTooltip("ActaCron - Dynamic JS Engine & Cron Hub")
				}
			}
		}()
	}
}

func (th *TrayHandler) onExitInternal() {
	if th.onExit != nil {
		th.onExit()
	}
}
```

- [ ] **Step 2: Create `internal/tray/tray_other.go`**

Implement headless stub for non-Windows platforms with `//go:build !windows`:

```go
//go:build !windows

package tray

import (
	"os"
	"os/signal"
	"syscall"

	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
)

type TrayHandler struct {
	dashboardURL string
	sched        *scheduler.Scheduler
	gitSvc       *gitmgr.GitService
	mgr          *manager.Manager
	onExit       func()
}

func New(dashboardURL string, sched *scheduler.Scheduler, gitSvc *gitmgr.GitService, mgr *manager.Manager, onExit func()) *TrayHandler {
	return &TrayHandler{
		dashboardURL: dashboardURL,
		sched:        sched,
		gitSvc:       gitSvc,
		mgr:          mgr,
		onExit:       onExit,
	}
}

// Run blocks until OS interrupt or SIGTERM signal on non-Windows headless environments.
func (th *TrayHandler) Run() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	if th.onExit != nil {
		th.onExit()
	}
}
```

- [ ] **Step 3: Remove old `internal/tray/tray.go`**

Delete `internal/tray/tray.go` since it has been replaced by `tray_windows.go` and `tray_other.go`.

- [ ] **Step 4: Test build on Windows and cross-compile for Linux and macOS**

```powershell
go build .
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -o dist/test_linux .
$env:GOOS="darwin"; $env:GOARCH="arm64"; $env:CGO_ENABLED="0"; go build -o dist/test_darwin .
Remove-Item dist/test_linux, dist/test_darwin -ErrorAction SilentlyContinue
$env:GOOS=""; $env:GOARCH=""
```
Expected: Both Windows, Linux and macOS compile cleanly without CGO errors.

- [ ] **Step 5: Commit changes**

```bash
git add internal/tray/
git commit -m "refactor(tray): decouple systray behind build tags for zero-cgo cross compilation"
```

---

### Task 2: Centralized Version Management Package

**Files:**
- Create: `internal/version/version.go`
- Modify: `main.go`
- Modify: `internal/mcp/server.go`
- Test: `internal/version/version_test.go`

**Interfaces:**
- Produces: `version.Version`, `version.GitCommit`, `version.BuildDate`, `version.GetInfo()`

- [ ] **Step 1: Write test for `internal/version`**

```go
package version

import (
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()
	if info.Version == "" {
		t.Errorf("Expected non-empty version")
	}
}
```

- [ ] **Step 2: Implement `internal/version/version.go`**

```go
package version

var (
	Version   = "1.0.0"
	GitCommit = "dev"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
}

func GetInfo() Info {
	return Info{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	}
}
```

- [ ] **Step 3: Update `main.go` and `internal/mcp/server.go`**

In `main.go`:
- Log version on startup: `log.Printf("Starting ActaCron %s (%s)", version.Version, version.GitCommit)`
- Clean up any leftover `.old` executables on startup (`cleanupOldBinary(execDir)`).

In `internal/mcp/server.go`:
- Replace hardcoded `"1.0.0"` with `version.Version`.

- [ ] **Step 4: Run tests & verify**

```powershell
go test ./internal/version
go test ./internal/mcp
```
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add internal/version/ main.go internal/mcp/server.go
git commit -m "feat(version): add centralized version package with ldflags support"
```

---

### Task 3: In-App Updater Service

**Files:**
- Create: `internal/updater/updater.go`
- Create: `internal/updater/updater_test.go`
- Create: `internal/updater/semver.go`
- Create: `internal/updater/semver_test.go`

**Interfaces:**
- Consumes: GitHub Releases API
- Produces: `CheckForUpdate(ownerRepo string, currentVer string) (*ReleaseInfo, error)`
- Produces: `ApplyUpdate(assetURL string, targetExePath string) error`

- [ ] **Step 1: Write tests for semantic version comparison (`internal/updater/semver_test.go`)**

```go
package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		wantNew bool
	}{
		{"1.0.0", "1.0.1", true},
		{"v1.0.0", "v1.0.1", true},
		{"1.0.1", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"v1.0.0", "1.0.0", false},
		{"1.2.0", "1.10.0", true},
	}

	for _, tt := range tests {
		got := IsNewerVersion(tt.current, tt.latest)
		if got != tt.wantNew {
			t.Errorf("IsNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, got, tt.wantNew)
		}
	}
}
```

- [ ] **Step 2: Implement `internal/updater/semver.go`**

Implement version parsing and comparison supporting optional `v` prefix and numeric major.minor.patch components.

- [ ] **Step 3: Implement `internal/updater/updater.go`**

Implement:
- `ReleaseInfo` model with `Version`, `TagName`, `Notes`, `PublishedAt`, `AssetURL`, `AssetName`, `HasUpdate`.
- `CheckForUpdate(client *http.Client, repo string, currentVersion string) (*ReleaseInfo, error)`:
  - Query `https://api.github.com/repos/{repo}/releases/latest`.
  - Set `User-Agent: ActaCron-Updater/{version}`.
  - Find matching asset for `runtime.GOOS` and `runtime.GOARCH` (e.g. `actacron-windows-amd64.zip` or `.exe`).
- `ApplyUpdate(assetURL string, currentExePath string) error`:
  - Download to temporary file.
  - If zip, extract `actacron.exe` or binary.
  - Windows safe replacement:
    - `os.Rename(currentExePath, currentExePath + ".old")`
    - `os.Rename(downloadedNewFile, currentExePath)`
- Helper function `CleanupOldBinary(exePath string)`:
  - Check if `exePath + ".old"` exists, attempt `os.Remove(exePath + ".old")`.

- [ ] **Step 4: Run tests**

```powershell
go test ./internal/updater/...
```
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add internal/updater/
git commit -m "feat(updater): add GitHub releases checker and in-place updater"
```

---

### Task 4: REST API Endpoints for Version & Updater

**Files:**
- Modify: `internal/api/router.go`
- Modify: `internal/api/handlers.go`
- Test: `internal/api/updater_api_test.go`

**Interfaces:**
- Exposes:
  - `GET /api/version` -> version info
  - `GET /api/update/check` -> returns `ReleaseInfo`
  - `POST /api/update/apply` -> downloads and applies update, returns restart status

- [ ] **Step 1: Write test for API update endpoints (`internal/api/updater_api_test.go`)**

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetVersion(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()
	// Router handler call
	// assert w.Code == 200 and json contains version
}
```

- [ ] **Step 2: Add handlers to `internal/api/handlers.go`**

Implement:
- `handleGetVersion(w http.ResponseWriter, r *http.Request)`
- `handleCheckUpdate(w http.ResponseWriter, r *http.Request)`
- `handleApplyUpdate(w http.ResponseWriter, r *http.Request)`
- Add restart helper triggering process restart after HTTP response flush.

- [ ] **Step 3: Register routes in `internal/api/router.go`**

```go
mux.HandleFunc("/api/version", h.handleGetVersion)
mux.HandleFunc("/api/update/check", h.handleCheckUpdate)
mux.HandleFunc("/api/update/apply", h.handleApplyUpdate)
```

- [ ] **Step 4: Run API tests**

```powershell
go test ./internal/api/...
```
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add internal/api/
git commit -m "feat(api): add /api/version and /api/update endpoints"
```

---

### Task 5: Web UI Frontend Updates & Notification Modal

**Files:**
- Modify: `web/index.html`
- Modify: `web/js/app.js`
- Modify: `web/css/style.css` (if styling needed)
- Test: `web/embed_test.go`

- [ ] **Step 1: Update `web/index.html`**

- In Settings modal or top header:
  - Add version badge: `<span id="app-version-badge">v1.0.0</span>`.
  - Add "Check for Updates" button (`id="btn-check-update"`).
- Add Update Modal dialog:
  - Title: "Update Available"
  - Details: New version, release notes / changelog.
  - Actions: "Later" button, "Update & Restart" button with progress bar.

- [ ] **Step 2: Add update logic in `web/js/app.js`**

- Fetch `/api/version` on initial load and display in the badge.
- Event listener for `#btn-check-update`:
  - Show spinner.
  - Call `GET /api/update/check`.
  - If `has_update == true`: populate modal with release notes, show Update Modal.
  - If no update: show notification "ActaCron is up to date".
- Event listener for `#btn-apply-update`:
  - Disable button, show "Downloading update...".
  - Call `POST /api/update/apply`.
  - On success: show "Restarting ActaCron...", poll `/api/version` until app comes back up, then refresh page.

- [ ] **Step 3: Run web tests**

```powershell
go test ./web/...
```
Expected: PASS.

- [ ] **Step 4: Commit changes**

```bash
git add web/
git commit -m "feat(ui): add in-app update check, release notes modal and live progress"
```

---

### Task 6: Inno Setup Windows Installer Script & Build Automation

**Files:**
- Create: `scripts/installer.iss`
- Modify: `scripts/build.ps1`
- Test: Local execution of `scripts/build.ps1`

- [ ] **Step 1: Create `scripts/installer.iss`**

```pascal
#ifndef MyAppVersion
  #define MyAppVersion "1.0.0"
#endif

[Setup]
AppId={{D8C8E192-3B47-49F1-8B0C-1A6E1E4F22A0}
AppName=ActaCron
AppVersion={#MyAppVersion}
AppPublisher=DennyNguyen
AppPublisherURL=https://github.com/DennyNguyen123/ActaCron
AppSupportURL=https://github.com/DennyNguyen123/ActaCron/issues
DefaultDirName={localappdata}\Programs\ActaCron
DefaultGroupName=ActaCron
OutputBaseFilename=ActaCron-Setup-{#MyAppVersion}
OutputDir=..\dist
Compression=lzma2/ultra64
SolidCompression=yes
PrivilegesRequired=lowest
CloseApplications=yes
RestartApplications=no
WizardStyle=modern
UninstallDisplayIcon={app}\actacron.exe

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "autostart"; Description: "Start ActaCron automatically when Windows starts"; GroupDescription: "Startup:"; Flags: unchecked

[Files]
Source: "..\dist\ActaCron-{#MyAppVersion}-windows-amd64\actacron.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\ActaCron-{#MyAppVersion}-windows-amd64\.env.example"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\ActaCron-{#MyAppVersion}-windows-amd64\README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme
Source: "..\dist\ActaCron-{#MyAppVersion}-windows-amd64\packages\demo-pack\*"; DestDir: "{app}\packages\demo-pack"; Flags: onlyifdoesntexist recursesubdirs createallsubdirs

[Icons]
Name: "{group}\ActaCron"; Filename: "{app}\actacron.exe"
Name: "{group}\{cm:UninstallProgram,ActaCron}"; Filename: "{uninstallexe}"
Name: "{userdesktop}\ActaCron"; Filename: "{app}\actacron.exe"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "ActaCron"; ValueData: """{app}\actacron.exe"""; Flags: uninsdeletevalue; Tasks: autostart

[Run]
Filename: "{app}\actacron.exe"; Description: "{cm:LaunchProgram,ActaCron}"; Flags: nowait postinstall skipifsilent
```

- [ ] **Step 2: Update `scripts/build.ps1`**

Support version argument `$Version = "v1.0.0"`, inject `-ldflags`, check for `iscc.exe` (Inno Setup Compiler) if installed locally to generate setup.

- [ ] **Step 3: Test local build**

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```
Expected: Successfully generates `dist/ActaCron-v1.0.0-windows-amd64.zip`.

- [ ] **Step 4: Commit changes**

```bash
git add scripts/
git commit -m "feat(installer): create Inno Setup installer script and updated build automation"
```

---

### Task 7: GitHub Actions Matrix Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  build:
    name: Build (${{ matrix.goos }}/${{ matrix.goarch }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        include:
          # Windows amd64 (GUI + Inno Setup Installer)
          - os: windows-latest
            goos: windows
            goarch: amd64
            archive_ext: zip
            build_installer: true

          # Windows arm64
          - os: windows-latest
            goos: windows
            goarch: arm64
            archive_ext: zip
            build_installer: false

          # Linux amd64
          - os: ubuntu-latest
            goos: linux
            goarch: amd64
            archive_ext: tar.gz
            build_installer: false

          # Linux arm64
          - os: ubuntu-latest
            goos: linux
            goarch: arm64
            archive_ext: tar.gz
            build_installer: false

          # macOS amd64 (Intel)
          - os: macos-latest
            goos: darwin
            goarch: amd64
            archive_ext: tar.gz
            build_installer: false

          # macOS arm64 (Apple Silicon)
          - os: macos-latest
            goos: darwin
            goarch: arm64
            archive_ext: tar.gz
            build_installer: false

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          check-latest: true

      - name: Run Tests
        run: go test ./...

      - name: Build Binary (Windows)
        if: matrix.goos == 'windows'
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: '0'
        run: |
          $ver = "${{ github.ref_name }}"
          $commit = "${{ github.sha }}"
          $date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
          $flags = "-H windowsgui -s -w -X 'actacron/internal/version.Version=$ver' -X 'actacron/internal/version.GitCommit=$commit' -X 'actacron/internal/version.BuildDate=$date'"
          $pkgDir = "dist/actacron-${ver}-${{ matrix.goos }}-${{ matrix.goarch }}"
          New-Item -ItemType Directory -Path "$pkgDir/packages/demo-pack" -Force | Out-Null
          go build -ldflags "$flags" -o "$pkgDir/actacron.exe" .
          Copy-Item ".env.example" "$pkgDir/.env.example"
          Copy-Item "README.md" "$pkgDir/README.md" -ErrorAction SilentlyContinue
          Copy-Item -Recurse "packages/demo-pack/*" "$pkgDir/packages/demo-pack/"
          Compress-Archive -Path "$pkgDir" -DestinationPath "dist/actacron-${ver}-${{ matrix.goos }}-${{ matrix.goarch }}.zip"

      - name: Build Inno Setup Installer (Windows amd64)
        if: matrix.build_installer == true
        run: |
          $ver = "${{ github.ref_name }}"
          iscc scripts/installer.iss "/DMyAppVersion=$ver"

      - name: Build Binary (Unix)
        if: matrix.goos != 'windows'
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: '0'
        run: |
          VER="${{ github.ref_name }}"
          COMMIT="${{ github.sha }}"
          DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
          FLAGS="-s -w -X 'actacron/internal/version.Version=$VER' -X 'actacron/internal/version.GitCommit=$COMMIT' -X 'actacron/internal/version.BuildDate=$DATE'"
          PKG_DIR="dist/actacron-${VER}-${{ matrix.goos }}-${{ matrix.goarch }}"
          mkdir -p "$PKG_DIR/packages/demo-pack"
          go build -ldflags "$FLAGS" -o "$PKG_DIR/actacron" .
          cp .env.example "$PKG_DIR/.env.example"
          cp README.md "$PKG_DIR/README.md" || true
          cp -r packages/demo-pack/* "$PKG_DIR/packages/demo-pack/" || true
          tar -czvf "dist/actacron-${VER}-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz" -C dist "actacron-${VER}-${{ matrix.goos }}-${{ matrix.goarch }}"

      - name: Upload Artifacts
        uses: actions/upload-artifact@v4
        with:
          name: assets-${{ matrix.goos }}-${{ matrix.goarch }}
          path: dist/*.*

  publish:
    name: Publish GitHub Release
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Download all artifacts
        uses: actions/download-artifact@v4
        with:
          path: release-assets
          merge-multiple: true

      - name: Generate Checksums
        working-directory: release-assets
        run: |
          sha256sum * > checksums-sha256.txt

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: release-assets/*
          generate_release_notes: true
```

- [ ] **Step 2: Commit workflow**

```bash
git add .github/workflows/release.yml
git commit -m "ci(github): add matrix release workflow for Windows, Linux and macOS"
```

---

### Task 8: Verification & End-to-End Self-Update Testing

- [ ] **Step 1: Run all unit tests**
  ```powershell
  go test -v ./...
  ```
  Expected: All tests pass.
- [ ] **Step 2: Test Cross-Compilation for all matrix targets**
  ```powershell
  $env:CGO_ENABLED="0"
  $targets = @(
      @{OS="windows"; ARCH="amd64"},
      @{OS="windows"; ARCH="arm64"},
      @{OS="linux"; ARCH="amd64"},
      @{OS="linux"; ARCH="arm64"},
      @{OS="darwin"; ARCH="amd64"},
      @{OS="darwin"; ARCH="arm64"}
  )
  foreach ($t in $targets) {
      $env:GOOS = $t.OS
      $env:GOARCH = $t.ARCH
      Write-Host "Compiling for $($t.OS)/$($t.ARCH)..."
      go build -o dist/test_build .
      Remove-Item dist/test_build* -ErrorAction SilentlyContinue
  }
  $env:GOOS=""; $env:GOARCH=""
  ```
  Expected: All 6 cross-compile targets compile cleanly.
- [ ] **Step 3: Verification of In-App Updater API endpoints**
  Start `go run . headless` and verify `curl http://localhost:8080/api/version` and `curl http://localhost:8080/api/update/check`.
