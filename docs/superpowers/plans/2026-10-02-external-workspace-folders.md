# External Workspace Folders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to select any folder on their filesystem and use it directly as an active ActaCron workspace without copying files into the `packages/` directory.

**Architecture:** External workspace folders are registered in SQLite. The Manager is decoupled from assuming packages are always subdirectories of `packagesDir`, resolving package paths dynamically from either `packagesDir` or registered external directory paths. An API endpoint triggers the native OS folder picker or accepts manual paths. The Web UI provides a "+ Folder" action, external folder modal, badges in the scripts tree, and an unlink option.

**Tech Stack:** Go (Goja JS engine, modernc.org/sqlite, fsnotify, net/http), Vanilla JS/HTML/CSS for embedded web UI, PowerShell/osascript/zenity for native folder picker.

**Spec:** `docs/superpowers/specs/2026-10-02-external-workspace-folders-design.md`

## Global Constraints

- Never copy files or directories from external workspaces into `packages/`.
- External workspaces must be persisted across application restarts.
- Unlinking an external workspace must never delete or alter original files on disk.
- JavaScript `require()` within an external workspace must be allowed to load relative files inside that workspace directory and files inside `packages/_shared/`.
- Workspace names must be unique across both internal packages and external workspaces.

## Review Focus

1. External path with spaces or special characters (e.g. `D:\My Projects\Script Folder`) resolves and executes correctly.
2. Deleted or disconnected external folder path on disk does not crash Manager.Reload(), reporting a graceful status instead.
3. Unlink external workspace removes registration from database and memory while leaving disk files intact.
4. Relative `require("./helper")` and `require("_shared")` work correctly inside external workspaces without triggering path traversal security errors.
5. Watcher (`fsnotify`) triggers reload when scripts inside external workspace folders are modified.

---

### Task 1: Storage Layer for External Workspaces

**Files:**
- Modify: `internal/domain/models.go`
- Modify: `internal/storage/sqlite.go`
- Test: `internal/storage/sqlite_test.go`

**Interfaces:**
- Consumes: `domain.ExternalWorkspace`
- Produces:
  - `s.AddExternalWorkspace(name, path string) error`
  - `s.ListExternalWorkspaces() ([]domain.ExternalWorkspace, error)`
  - `s.DeleteExternalWorkspace(name string) error`
  - `s.GetExternalWorkspace(name string) (*domain.ExternalWorkspace, error)`

- [ ] **Step 1: Write the failing test**

In `internal/storage/sqlite_test.go`:
```go
func TestExternalWorkspacesCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "actacron-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// 1. Add external workspace
	err = db.AddExternalWorkspace("my_external", "D:/External/Scripts")
	if err != nil {
		t.Fatalf("expected no error adding external workspace, got: %v", err)
	}

	// 2. Get external workspace
	ws, err := db.GetExternalWorkspace("my_external")
	if err != nil || ws == nil {
		t.Fatalf("failed to get external workspace: %v", err)
	}
	if ws.Name != "my_external" || ws.Path != "D:/External/Scripts" {
		t.Fatalf("unexpected ws data: %+v", ws)
	}

	// 3. List external workspaces
	list, err := db.ListExternalWorkspaces()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 external workspace, got %d, err: %v", len(list), err)
	}

	// 4. Delete external workspace
	err = db.DeleteExternalWorkspace("my_external")
	if err != nil {
		t.Fatalf("failed to delete external workspace: %v", err)
	}

	listAfter, err := db.ListExternalWorkspaces()
	if err != nil || len(listAfter) != 0 {
		t.Fatalf("expected 0 external workspaces after delete, got %d", len(listAfter))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/storage -run TestExternalWorkspacesCRUD`
Expected: FAIL with undefined `AddExternalWorkspace` / `ExternalWorkspace`.

- [ ] **Step 3: Implement minimal code**

In `internal/domain/models.go`:
```go
type ExternalWorkspace struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}
```
Add `IsExternal bool` to `PackageInfo`:
```go
type PackageInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	IsGit      bool      `json:"is_git"`
	IsExternal bool      `json:"is_external"`
	RemoteURL  string    `json:"remote_url,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	Status     string    `json:"status"`
	Functions  []string  `json:"functions"`
	UpdatedAt  time.Time `json:"updated_at"`
}
```

In `internal/storage/sqlite.go`:
Add table in `initSchema()`:
```sql
CREATE TABLE IF NOT EXISTS external_workspaces (
	name TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```
Add CRUD methods:
```go
func (s *DB) AddExternalWorkspace(name, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `INSERT OR REPLACE INTO external_workspaces (name, path, created_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, name, path, time.Now())
	return err
}

func (s *DB) ListExternalWorkspaces() ([]domain.ExternalWorkspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT name, path, created_at FROM external_workspaces ORDER BY name ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.ExternalWorkspace
	for rows.Next() {
		var ws domain.ExternalWorkspace
		if err := rows.Scan(&ws.Name, &ws.Path, &ws.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, ws)
	}
	return result, nil
}

func (s *DB) GetExternalWorkspace(name string) (*domain.ExternalWorkspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT name, path, created_at FROM external_workspaces WHERE name = ?`
	row := s.db.QueryRow(query, name)
	var ws domain.ExternalWorkspace
	if err := row.Scan(&ws.Name, &ws.Path, &ws.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ws, nil
}

func (s *DB) DeleteExternalWorkspace(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `DELETE FROM external_workspaces WHERE name = ?`
	_, err := s.db.Exec(query, name)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/storage -run TestExternalWorkspacesCRUD`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/models.go internal/storage/sqlite.go internal/storage/sqlite_test.go
git commit -m "feat(storage): add external_workspaces table and CRUD methods"
```

---

### Task 2: Manager Dynamic Package Resolution & External Workspaces Support

**Files:**
- Modify: `internal/manager/manager.go`
- Test: `internal/manager/manager_test.go`

**Interfaces:**
- Consumes:
  - `storage.DB.ListExternalWorkspaces()`
- Produces:
  - `m.GetPackageDir(pkgName string) (string, error)`
  - `m.Reload()` handles both internal `packagesDir` and DB `external_workspaces`
  - `m.SaveFunction` uses `GetPackageDir`
  - `m.DeleteFunction` uses `GetPackageDir`
  - `require()` sandbox supports external package root directory + `packages/_shared/`
  - `watcher` adds all external workspace paths

- [ ] **Step 1: Write the failing test**

In `internal/manager/manager_test.go`:
```go
func TestExternalWorkspaceLoadingAndExecution(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	extDir := filepath.Join(tmpDir, "external_project")
	os.MkdirAll(packagesDir, 0755)
	os.MkdirAll(extDir, 0755)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Register external workspace in DB
	err = db.AddExternalWorkspace("myext", extDir)
	if err != nil {
		t.Fatalf("failed to add external ws: %v", err)
	}

	// Create a script and .env in external dir
	os.WriteFile(filepath.Join(extDir, ".env"), []byte("FOO=bar_ext\n"), 0644)
	os.WriteFile(filepath.Join(extDir, "calc.js"), []byte(`
/**
 * @name calculate
 */
function main(params) {
    return { res: params.a * 2, env: process.env.FOO };
}
`), 0644)

	runner := engine.New(db, 30, false)
	mgr := New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload failed: %v", err)
	}

	// Verify package is loaded as external
	pkg := mgr.GetPackage("myext")
	if pkg == nil {
		t.Fatalf("expected package myext to exist")
	}
	if !pkg.IsExternal {
		t.Fatalf("expected pkg.IsExternal to be true")
	}
	if pkg.Path != extDir {
		t.Fatalf("expected pkg.Path=%s, got %s", extDir, pkg.Path)
	}

	// Verify function is loaded
	fn := mgr.GetFunction("myext/calculate")
	if fn == nil {
		t.Fatalf("expected function myext/calculate to exist")
	}

	// Execute function
	ctx := context.Background()
	output, err := mgr.CallWithTrigger(ctx, "myext/calculate", map[string]interface{}{"a": 21}, "test")
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	resMap, ok := output.(map[string]interface{})
	if !ok || resMap["res"] != int64(42) || resMap["env"] != "bar_ext" {
		t.Fatalf("unexpected output: %v", output)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/manager -run TestExternalWorkspaceLoadingAndExecution`
Expected: FAIL because Manager does not yet scan external workspaces or recognize `IsExternal`.

- [ ] **Step 3: Implement minimal code**

In `internal/manager/manager.go`:
1. Add `GetPackageDir`:
```go
func (m *Manager) GetPackageDir(pkgName string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if pkg, ok := m.packages[pkgName]; ok && pkg.Path != "" {
		return pkg.Path, nil
	}
	return filepath.Join(m.packagesDir, pkgName), nil
}
```
2. Refactor package scanning logic into a helper `scanPackage(pkgName, pkgPath string, isExternal bool, ...)`:
In `Reload()`:
- Scan `m.packagesDir` subdirectories as internal packages (`isExternal = false`).
- If `m.db != nil`:
  Query `m.db.ListExternalWorkspaces()`. For each `extWs`:
  - Check if `extWs.Path` exists and is dir.
  - If valid, scan using `scanPackage(extWs.Name, extWs.Path, true, ...)`.
  - Mark `pkgInfo.IsExternal = true`.
3. Update `SaveFunction`:
Replace:
```go
pkgPath := filepath.Join(m.packagesDir, pkgName)
```
with:
```go
pkgPath, _ := m.GetPackageDir(pkgName)
```
4. Update `DeleteFunction`:
Use `GetPackageDir(pkgName)` to locate file path.
5. Update `require()` sandbox checks:
In `executeRequire`:
Determine allowed roots:
```go
cleanPkgDir := filepath.Clean(pkgPath)
cleanSharedDir := filepath.Clean(filepath.Join(m.packagesDir, "_shared"))
isInsidePkg := strings.HasPrefix(resolvedPath, cleanPkgDir+string(filepath.Separator)) || resolvedPath == cleanPkgDir
isInsideShared := strings.HasPrefix(resolvedPath, cleanSharedDir+string(filepath.Separator)) || resolvedPath == cleanSharedDir
if !isInsidePkg && !isInsideShared {
    return nil, fmt.Errorf("security error: path traversal outside workspace or _shared not permitted")
}
```
6. Update `StartWatcher()` to add `watcher.Add(pkgPath)` for all external workspace directories.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/manager -run TestExternalWorkspaceLoadingAndExecution`
Expected: PASS

- [ ] **Step 5: Run existing tests to ensure no regression**

Run: `go test -v ./internal/manager/...`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/manager/manager.go internal/manager/manager_test.go
git commit -m "feat(manager): support external workspace loading, execution and sandboxing"
```

---

### Task 3: Native OS Folder Picker & External Workspace API Endpoints

**Files:**
- Modify: `internal/tray/window.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Produces:
  - `tray.PickFolder() (string, error)`
  - `POST /api/workspace/pick-folder` -> `{ "path": "/path/to/folder" }`
  - `GET /api/workspace/external` -> `[]domain.ExternalWorkspace`
  - `POST /api/workspace/external` -> `{ "name": "...", "path": "..." }`
  - `DELETE /api/workspace/external?name=...` -> `{ "status": "deleted" }`

- [ ] **Step 1: Write the failing test**

In `internal/api/api_test.go`:
```go
func TestExternalWorkspaceAPI(t *testing.T) {
	ts, mgr, db, cleanup := setupTestServer(t)
	defer cleanup()

	tmpExtDir := t.TempDir()

	// 1. POST /api/workspace/external
	payload := fmt.Sprintf(`{"name":"ext_project","path":%q}`, tmpExtDir)
	resp, err := http.Post(ts.URL+"/api/workspace/external", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/workspace/external failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// 2. GET /api/workspace/external
	resp, err = http.Get(ts.URL + "/api/workspace/external")
	if err != nil {
		t.Fatalf("GET /api/workspace/external failed: %v", err)
	}
	var extList []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&extList)
	if len(extList) != 1 || extList[0]["name"] != "ext_project" {
		t.Fatalf("unexpected extList: %v", extList)
	}

	// Verify manager loaded it
	if mgr.GetPackage("ext_project") == nil {
		t.Fatalf("manager did not load external package")
	}

	// 3. DELETE /api/workspace/external?name=ext_project
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/workspace/external?name=ext_project", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE /api/workspace/external failed: status=%d, err=%v", resp.StatusCode, err)
	}

	// Verify manager unloaded it
	if mgr.GetPackage("ext_project") != nil {
		t.Fatalf("manager did not unload deleted external package")
	}

	// Verify external directory still exists on disk (NEVER DELETED)
	if _, err := os.Stat(tmpExtDir); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: external directory was deleted on disk!")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api -run TestExternalWorkspaceAPI`
Expected: FAIL (404 Not Found on `/api/workspace/external`).

- [ ] **Step 3: Implement minimal code**

In `internal/tray/window.go`:
```go
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
```

In `internal/api/handlers.go`:
```go
func (h *APIHandler) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	selectedPath, err := tray.PickFolder()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open folder picker: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": selectedPath})
}

func (h *APIHandler) handleExternalWorkspace(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if h.db == nil {
			writeJSON(w, http.StatusOK, []interface{}{})
			return
		}
		list, err := h.db.ListExternalWorkspaces()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)

	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Path = strings.TrimSpace(req.Path)
		if req.Name == "" || req.Path == "" {
			writeError(w, http.StatusBadRequest, "name and path are required")
			return
		}
		stat, err := os.Stat(req.Path)
		if err != nil || !stat.IsDir() {
			writeError(w, http.StatusBadRequest, "specified path does not exist or is not a directory")
			return
		}
		if h.db != nil {
			if err := h.db.AddExternalWorkspace(req.Name, req.Path); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save external workspace: "+err.Error())
				return
			}
		}
		_ = h.mgr.Reload()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "name": req.Name})

	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "name parameter is required")
			return
		}
		if h.db != nil {
			if err := h.db.DeleteExternalWorkspace(name); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to delete external workspace: "+err.Error())
				return
			}
		}
		_ = h.mgr.Reload()
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
```

In `internal/api/router.go`:
```go
mux.HandleFunc("/api/workspace/pick-folder", handler.handlePickFolder)
mux.HandleFunc("/api/workspace/external", handler.handleExternalWorkspace)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api -run TestExternalWorkspaceAPI`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tray/window.go internal/api/handlers.go internal/api/router.go internal/api/api_test.go
git commit -m "feat(api): add pick-folder and external workspace API endpoints"
```

---

### Task 4: Web UI Integration

**Files:**
- Modify: `web/index.html`
- Modify: `web/js/app.js`
- Modify: `web/js/i18n.js`
- Test: `web/embed_test.go`

**Interfaces:**
- Produces:
  - Button `#btnOpenExternalFolderModal` with text `+ Folder` in sidebar header
  - Modal `#modalAddExternalFolder` with path input, Browse button, name input, Add button
  - External badge indicator in script tree for packages where `is_external === true`
  - Unlink action button for external workspace in script detail / tree
  - Translation keys: `add_folder`, `browse_folder`, `folder_path`, `unlink_workspace`, `confirm_unlink_workspace`

- [ ] **Step 1: Write the failing test**

In `web/embed_test.go`:
```go
func TestExternalFolderUIElements(t *testing.T) {
	data, err := Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed to read embedded index.html: %v", err)
	}
	html := string(data)

	for _, id := range []string{"btnOpenExternalFolderModal", "modalAddExternalFolder", "btnBrowseFolder", "btnSubmitExternalFolder"} {
		if !strings.Contains(html, id) {
			t.Errorf("expected index.html to contain element with id %q", id)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./web -run TestExternalFolderUIElements`
Expected: FAIL with missing elements.

- [ ] **Step 3: Implement minimal code**

In `web/index.html`:
In `#workspace-view .pane-header`:
```html
<button class="btn btn-outline btn-sm" id="btnOpenExternalFolderModal" title="Open External Folder as Workspace" data-i18n="add_folder">+ Folder</button>
```

Add Modal `#modalAddExternalFolder`:
```html
<div class="modal-overlay" id="modalAddExternalFolder">
  <div class="modal">
    <div class="modal-header">
      <h3 data-i18n="add_external_folder_title">Add Folder as Workspace</h3>
      <button class="btn-close" id="btnCloseAddExternalFolder">&times;</button>
    </div>
    <div class="modal-body">
      <div class="form-group">
        <label data-i18n="folder_path">Folder Path</label>
        <div style="display: flex; gap: 8px;">
          <input type="text" id="inputExtFolderPath" class="form-control" style="flex:1;" placeholder="D:\Projects\MyScripts" />
          <button class="btn btn-outline" id="btnBrowseFolder" data-i18n="browse_folder">📂 Browse...</button>
        </div>
      </div>
      <div class="form-group">
        <label data-i18n="workspace_name">Workspace Name</label>
        <input type="text" id="inputExtWorkspaceName" class="form-control" placeholder="MyScripts" />
      </div>
      <p style="font-size: 12px; color: var(--text-muted); margin-top: 4px;" data-i18n="external_folder_note">
        Files will remain in their original folder and will NOT be copied into packages.
      </p>
    </div>
    <div class="modal-footer">
      <button class="btn btn-outline" id="btnCancelAddExternalFolder" data-i18n="cancel">Cancel</button>
      <button class="btn btn-primary" id="btnSubmitExternalFolder" data-i18n="add_workspace">Add Workspace</button>
    </div>
  </div>
</div>
```

In `web/js/app.js`:
1. Bind open/close modal `#modalAddExternalFolder`.
2. Browse button `#btnBrowseFolder`:
Calls `POST /api/workspace/pick-folder`. On return, sets `#inputExtFolderPath` and auto-fills `#inputExtWorkspaceName` with folder basename if empty.
3. Submit button `#btnSubmitExternalFolder`:
Sends `POST /api/workspace/external` with name and path. On success, closes modal, refreshes packages and scripts tree.
4. Render scripts tree:
If `pkg.is_external`, display an external icon badge `🔗` next to package name, and add an "Unlink" button or menu item to unlink workspace (`DELETE /api/workspace/external?name=...`).

In `web/js/i18n.js`:
Add English and Vietnamese translations:
- `add_folder`: "+ Thư mục" / "+ Folder"
- `add_external_folder_title`: "Thêm Thư Mục làm Workspace" / "Add Folder as Workspace"
- `folder_path`: "Đường dẫn Thư mục" / "Folder Path"
- `browse_folder`: "📂 Chọn..." / "📂 Browse..."
- `workspace_name`: "Tên Workspace" / "Workspace Name"
- `external_folder_note`: "Các file sẽ được sử dụng trực tiếp tại thư mục gốc, KHÔNG copy vào packages." / "Files will remain in their original folder and will NOT be copied into packages."
- `unlink_workspace`: "Hủy liên kết" / "Unlink Workspace"
- `confirm_unlink_workspace`: "Bạn có chắc muốn hủy liên kết Workspace này khỏi ActaCron? Các file trên ổ đĩa sẽ KHÔNG bị xóa." / "Are you sure you want to unlink this workspace? Files on disk will NOT be deleted."

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./web -run TestExternalFolderUIElements`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/js/app.js web/js/i18n.js web/embed_test.go
git commit -m "feat(ui): add external folder workspace modal, browse picker and tree integration"
```

---

### Task 5: End-to-End Integration Verification

**Files:**
- Test: `tests/e2e/external_workspace_e2e_test.go`

**Interfaces:**
- Verifies full stack: DB registration, package scanning, script execution with environment variables, file safety (zero file copying into packages), and clean unlinking.

- [ ] **Step 1: Write E2E test**

Create `tests/e2e/external_workspace_e2e_test.go`:
```go
package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/storage"
)

func TestExternalWorkspaceE2E(t *testing.T) {
	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	extFolder := filepath.Join(tmpDir, "my_standalone_scripts")
	os.MkdirAll(packagesDir, 0755)
	os.MkdirAll(extFolder, 0755)

	db, err := storage.New(filepath.Join(tmpDir, "app.db"))
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// 1. Create files in standalone folder
	os.WriteFile(filepath.Join(extFolder, ".env"), []byte("API_SECRET=supersecret123\n"), 0644)
	os.WriteFile(filepath.Join(extFolder, "runner.js"), []byte(`
/**
 * @name runTask
 */
function main(params) {
    return { status: "ok", secret: process.env.API_SECRET, count: params.count + 1 };
}
`), 0644)

	// 2. Register external workspace in DB
	if err := db.AddExternalWorkspace("standalone", extFolder); err != nil {
		t.Fatalf("failed to register external ws: %v", err)
	}

	runner := engine.New(db, 30, false)
	mgr := manager.New(packagesDir, runner, db)
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload failed: %v", err)
	}

	// 3. Verify files are NOT in packagesDir
	copiedTarget := filepath.Join(packagesDir, "standalone")
	if _, err := os.Stat(copiedTarget); !os.IsNotExist(err) {
		t.Fatalf("VIOLATION: files were copied into packages directory: %s", copiedTarget)
	}

	// 4. Verify function execution
	out, err := mgr.CallWithTrigger(context.Background(), "standalone/runTask", map[string]interface{}{"count": 5}, "manual")
	if err != nil {
		t.Fatalf("CallWithTrigger failed: %v", err)
	}
	resMap := out.(map[string]interface{})
	if resMap["status"] != "ok" || resMap["secret"] != "supersecret123" || resMap["count"] != int64(6) {
		t.Fatalf("unexpected result: %v", resMap)
	}

	// 5. Unlink external workspace
	if err := db.DeleteExternalWorkspace("standalone"); err != nil {
		t.Fatalf("failed to delete external ws: %v", err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatalf("mgr.Reload after unlink failed: %v", err)
	}

	if mgr.GetPackage("standalone") != nil {
		t.Fatalf("expected package to be unlinked")
	}

	// Original folder must remain untouched
	if _, err := os.Stat(filepath.Join(extFolder, "runner.js")); err != nil {
		t.Fatalf("original file was removed or corrupted: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test -v ./tests/e2e -run TestExternalWorkspaceE2E`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/external_workspace_e2e_test.go
git commit -m "test(e2e): add external workspace lifecycle and no-copy integrity test"
```
