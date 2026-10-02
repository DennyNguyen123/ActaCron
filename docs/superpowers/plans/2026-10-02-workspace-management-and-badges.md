# Workspace Management & UI Refinements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate workspace creation buttons into a unified `+ Workspace` menu, render a `GIT` badge for git repositories, and provide a secure internal workspace deletion API and UI.

**Architecture:** 
1. Extend `internal/manager/Manager` with `DeletePackage(pkgName string) error` to safely validate and delete internal package folders while rejecting `_shared`, external workspaces, and path traversal.
2. Expose `DELETE /api/packages?package=<pkgName>` and `POST /api/packages/delete` via `APIHandler`.
3. In `web/index.html`, consolidate `+ Folder` and `+ Git` into a clean `+ Workspace ▾` dropdown container preserving button IDs to guarantee backwards compatibility.
4. In `web/js/app.js`, render the `GIT` badge next to workspace names when `pkgMeta.is_git` is true, and provide a `🗑️` delete button for internal workspaces with user confirmation.

**Tech Stack:** Go 1.24, Vanilla JS / HTML5 / CSS3, Inno Setup, SQLite.

**Spec:** Bounded design based on user requirements in chat (2026-10-02):
- Consolidate `+ Folder` and `+ Git` into 1 unified workspace action button.
- Render `GIT` badge when workspace has a `.git` folder (`is_git === true`).
- Enable workspace deletion for internal packages (with safety checks and confirmation).

## Global Constraints

- Never delete external workspaces from disk — external workspaces must only be unlinked via `/api/workspace/external`.
- Never allow deleting the `_shared` package.
- Prevent all directory traversal attacks (`..`, `/`, `\`) on package deletion.
- Retain existing element IDs (`btnOpenExternalFolderModal`, `btnOpenCloneModal`, etc.) so `web/embed_test.go` and existing handlers continue to pass without regression.
- Keep all unit tests passing with zero build/lint errors across all packages.

## Review Focus

1. Deleting an external workspace path via `/api/packages` is rejected with an informative error directing user to unlink instead.
2. Attempting to delete `_shared`, empty string, `.` or `..` returns HTTP 400 Bad Request.
3. Path traversal attempts (e.g. `../../sensitive` or `sub/folder`) are rejected.
4. Clicking outside the `+ Workspace` dropdown closes the menu; clicking an action opens the corresponding modal and closes the dropdown.
5. `GIT` badge displays properly alongside `EXT` or `MISSING` badges without layout breakage on narrow tree sidebars.

---

### Task 1: Backend Manager & API for Package Deletion

**Files:**
- Modify: `internal/manager/manager.go:660-680`
- Modify: `internal/api/handlers.go:73-89`
- Modify: `internal/api/router.go:38-48`
- Test: `internal/manager/manager_test.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `m.packagesDir`, `m.externalWorkspaces`, `m.Reload()`
- Produces: `func (m *Manager) DeletePackage(pkgName string) error`
- Endpoint: `DELETE /api/packages?package={name}` & `POST /api/packages/delete`

- [ ] **Step 1: Write failing test in `internal/manager/manager_test.go`**

```go
func TestDeletePackage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "actacron-pkg-delete-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := storage.NewDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mgr := New(tmpDir, nil, db)
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	// Create dummy package
	dummyPkg := filepath.Join(tmpDir, "dummy_pkg")
	if err := os.MkdirAll(dummyPkg, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dummyPkg, "test.js"), []byte("// test"), 0644)
	_ = mgr.Reload()

	// 1. Should fail on empty or _shared
	if err := mgr.DeletePackage(""); err == nil {
		t.Error("expected error for empty package name")
	}
	if err := mgr.DeletePackage("_shared"); err == nil {
		t.Error("expected error deleting _shared")
	}

	// 2. Should fail on path traversal
	if err := mgr.DeletePackage("../something"); err == nil {
		t.Error("expected error for path traversal")
	}

	// 3. Should succeed on valid dummy package
	if err := mgr.DeletePackage("dummy_pkg"); err != nil {
		t.Fatalf("unexpected error deleting dummy_pkg: %v", err)
	}

	// Verify directory removed from disk
	if _, err := os.Stat(dummyPkg); !os.IsNotExist(err) {
		t.Errorf("expected dummy_pkg directory to be deleted")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestDeletePackage ./internal/manager`
Expected: FAIL with `mgr.DeletePackage undefined`

- [ ] **Step 3: Implement `DeletePackage` in `internal/manager/manager.go` and handlers**

In `internal/manager/manager.go`:
```go
// DeletePackage removes an internal package directory from disk and reloads.
// It rejects empty names, _shared, path traversals, non-existent packages, and external workspaces.
func (m *Manager) DeletePackage(pkgName string) error {
	if pkgName == "" || pkgName == "_shared" {
		return fmt.Errorf("cannot delete package '%s'", pkgName)
	}
	cleanName := filepath.Clean(pkgName)
	if cleanName == "." || cleanName == ".." || strings.Contains(cleanName, "/") || strings.Contains(cleanName, "\\") {
		return fmt.Errorf("invalid package name '%s'", pkgName)
	}

	m.mu.RLock()
	if _, isExt := m.externalWorkspaces[cleanName]; isExt {
		m.mu.RUnlock()
		return fmt.Errorf("package '%s' is an external workspace; use unlink instead", cleanName)
	}
	m.mu.RUnlock()

	pkgDir := filepath.Join(m.packagesDir, cleanName)
	if fi, err := os.Stat(pkgDir); os.IsNotExist(err) || !fi.IsDir() {
		return fmt.Errorf("package '%s' does not exist", cleanName)
	}

	if err := os.RemoveAll(pkgDir); err != nil {
		return fmt.Errorf("failed to delete package directory: %w", err)
	}

	return m.Reload()
}
```

In `internal/api/handlers.go`:
```go
func (h *APIHandler) handleDeletePackage(w http.ResponseWriter, r *http.Request) {
	pkgName := r.URL.Query().Get("package")
	if pkgName == "" && r.Method == http.MethodPost {
		var req struct {
			Package string `json:"package"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		pkgName = req.Package
	}
	if pkgName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing package parameter"})
		return
	}
	if h.mgr == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "manager not initialized"})
		return
	}
	if err := h.mgr.DeletePackage(pkgName); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
```

In `internal/api/router.go`:
Map `DELETE /api/packages` and `POST /api/packages/delete`:
```go
mux.HandleFunc("/api/packages", func(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		handler.handleDeletePackage(w, r)
		return
	}
	handler.handleListPackages(w, r)
})
mux.HandleFunc("/api/packages/delete", handler.handleDeletePackage)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run TestDeletePackage ./internal/manager`
Run: `go test -v ./internal/api/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/manager/manager.go internal/manager/manager_test.go internal/api/handlers.go internal/api/router.go internal/api/api_test.go
git commit -m "feat(api): add package deletion API and safety validations"
```

---

### Task 2: Consolidate `+ Folder` and `+ Git` into `+ Workspace` Dropdown

**Files:**
- Modify: `web/index.html:139-147`
- Modify: `web/css/style.css`
- Modify: `web/js/app.js:1330-1430`
- Test: `web/embed_test.go`

**Interfaces:**
- Consumes: `#btnWorkspaceMenu`, `#dropdownWorkspaceMenu`
- Produces: Seamless popup dropdown with `#btnOpenExternalFolderModal` and `#btnOpenCloneModal` intact.

- [ ] **Step 1: Check existing `web/embed_test.go` and update `web/index.html`**

In `web/index.html`:
Replace the crowded button row in `#treePane .pane-header`:
```html
<div class="pane-header">
  <span data-i18n="scripts_tree">Packages & Scripts</span>
  <div style="display:flex; gap:4px; align-items:center;">
    <button class="btn btn-outline btn-sm" id="btnNewScript" data-i18n="new_script">+ Script</button>
    <div class="dropdown-wrapper" id="wsDropdownWrapper" style="position:relative;">
      <button class="btn btn-outline btn-sm" id="btnWorkspaceMenu" data-i18n="add_workspace_menu">+ Workspace ▾</button>
      <div class="dropdown-menu" id="dropdownWorkspaceMenu">
        <button class="dropdown-item" id="btnOpenExternalFolderModal" title="Open External Folder as Workspace">
          <span>📁</span> <span data-i18n="add_local_folder">Add Local Folder</span>
        </button>
        <button class="dropdown-item" id="btnOpenCloneModal" title="Clone Git Repository">
          <span>🌐</span> <span data-i18n="clone_git_repo">Clone Git Repo</span>
        </button>
      </div>
    </div>
    <button class="btn btn-outline btn-sm" id="btnOpenWorkspaceFolder" title="Open Workspace Directory" data-i18n="open_folder">📁 Open Dir</button>
  </div>
</div>
```

- [ ] **Step 2: Add CSS for `.dropdown-wrapper` and `.dropdown-menu` in `web/css/style.css`**

```css
.dropdown-wrapper {
  position: relative;
  display: inline-block;
}

.dropdown-menu {
  display: none;
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  min-width: 175px;
  background: var(--color-card, #1e293b);
  border: 1px solid var(--color-border, #334155);
  border-radius: 6px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.4), 0 4px 6px -2px rgba(0, 0, 0, 0.2);
  z-index: 1000;
  padding: 4px;
}

.dropdown-menu.show {
  display: block;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--color-foreground, #f8fafc);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease;
}

.dropdown-item:hover {
  background: var(--color-accent, rgba(255, 255, 255, 0.08));
  color: var(--color-primary, #38bdf8);
}
```

- [ ] **Step 3: Wire up Dropdown behavior in `web/js/app.js`**

Add toggle and click-outside handler:
```javascript
const wsWrapper = document.getElementById("wsDropdownWrapper");
const wsMenuBtn = document.getElementById("btnWorkspaceMenu");
const wsDropdown = document.getElementById("dropdownWorkspaceMenu");

if (wsMenuBtn && wsDropdown) {
  wsMenuBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    wsDropdown.classList.toggle("show");
  });

  document.addEventListener("click", (e) => {
    if (wsDropdown.classList.contains("show") && !wsWrapper.contains(e.target)) {
      wsDropdown.classList.remove("show");
    }
  });

  // Close dropdown when any item inside is clicked
  wsDropdown.querySelectorAll(".dropdown-item").forEach(item => {
    item.addEventListener("click", () => {
      wsDropdown.classList.remove("show");
    });
  });
}
```

- [ ] **Step 4: Run test to verify embedded UI assertions pass**

Run: `go test -v ./web`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/css/style.css web/js/app.js web/embed_test.go
git commit -m "feat(ui): consolidate folder and git buttons into unified workspace dropdown"
```

---

### Task 3: Git Badge & Workspace Deletion UI

**Files:**
- Modify: `web/js/app.js:320-375`
- Modify: `web/css/style.css`
- Modify: `web/js/i18n.js`

**Interfaces:**
- Consumes: `pkgMeta.is_git`, `pkgMeta.is_external`, `/api/packages?package=`
- Produces: `GIT` badge, `🗑️` delete button for internal packages, localized tooltips.

- [ ] **Step 1: Add i18n keys for badges and deletion in `web/js/i18n.js`**

In `en` and `vi`:
```javascript
// en
add_workspace_menu: "+ Workspace ▾",
add_local_folder: "Add Local Folder",
clone_git_repo: "Clone Git Repo",
delete_workspace: "Delete Workspace",
confirm_delete_workspace: "Are you sure you want to permanently delete workspace '{name}' from disk? This action cannot be undone.",

// vi
add_workspace_menu: "+ Workspace ▾",
add_local_folder: "Thêm thư mục máy",
clone_git_repo: "Clone từ Git",
delete_workspace: "Xóa Workspace",
confirm_delete_workspace: "Bạn có chắc chắn muốn xóa vĩnh viễn workspace '{name}' khỏi đĩa? Thao tác này không thể hoàn tác.",
```

- [ ] **Step 2: Add styles for `.badge-git` and `.btn-pkg-delete` in `web/css/style.css`**

```css
.badge-git {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  background: rgba(249, 115, 22, 0.18);
  color: #f97316;
  font-weight: 700;
  flex-shrink: 0;
  border: 1px solid rgba(249, 115, 22, 0.35);
}

.btn-pkg-delete:hover {
  background: rgba(239, 68, 68, 0.2) !important;
  color: #ef4444 !important;
}
```

- [ ] **Step 3: Update `renderTree` in `web/js/app.js` to render GIT badge and Delete button**

In `web/js/app.js`:
```javascript
const isGit = !!pkgMeta.is_git;

// Status badges
let badgesHtml = "";
if (isMissing) {
  badgesHtml += `<span style="font-size:10px; padding:1px 5px; border-radius:3px; background:rgba(239,68,68,0.2); color:#ef4444; font-weight:700; flex-shrink:0;">MISSING</span>`;
} else if (isExternal) {
  badgesHtml += `<span style="font-size:10px; padding:1px 5px; border-radius:3px; background:rgba(56,189,248,0.15); color:#38bdf8; font-weight:700; flex-shrink:0;">EXT</span>`;
}
if (isGit) {
  badgesHtml += `<span class="badge-git" title="Git Repository">GIT</span>`;
}

// Action buttons
const deleteTooltip = (window.I18n && window.I18n.t("delete_workspace")) || "Delete Workspace";
const actionButtons = `
  <div style="display:flex; align-items:center; gap:4px; flex-shrink:0;">
    ${!isShared ? `<button class="btn-pkg-action btn-pkg-config" data-i18n-title="ws_config_tooltip" title="${cfgTooltip}" data-pkg="${pkgName}">⚙️</button>` : ''}
    <button class="btn-pkg-action btn-pkg-folder" title="Open '${pkgName}' in Explorer" data-pkg="${pkgName}">📁</button>
    ${isExternal 
      ? `<button class="btn-pkg-action btn-pkg-unlink" title="${unlinkTooltip}" data-pkg="${pkgName}">🔗❌</button>` 
      : (!isShared ? `<button class="btn-pkg-action btn-pkg-delete" title="${deleteTooltip}" data-pkg="${pkgName}">🗑️</button>` : '')
    }
  </div>
`;
```

Attach delete click handler:
```javascript
const deleteBtn = headerDiv.querySelector(".btn-pkg-delete");
if (deleteBtn) {
  deleteBtn.addEventListener("click", async (e) => {
    e.stopPropagation();
    const confirmTpl = (window.I18n && window.I18n.t("confirm_delete_workspace")) || "Are you sure you want to permanently delete workspace '{name}' from disk?";
    const msg = confirmTpl.replace("{name}", pkgName);
    if (!confirm(msg)) return;

    try {
      const res = await fetch(`/api/packages?package=${encodeURIComponent(pkgName)}`, {
        method: "DELETE"
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        alert("Failed to delete workspace: " + (data.error || res.statusText));
      } else {
        await reloadPackagesAndTree();
      }
    } catch (err) {
      alert("Error deleting workspace: " + err.message);
    }
  });
}
```

- [ ] **Step 4: Run all unit tests**

Run: `go test -v ./...`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add web/js/app.js web/css/style.css web/js/i18n.js
git commit -m "feat(ui): display GIT badge and add delete package button with confirmation"
```

---

### Task 4: Full Verification & Sanity Check

**Files:**
- Test all packages: `go test -v ./...`
- Verify web assets: `go test -v ./web`

- [ ] **Step 1: Run complete test suite**

Run: `go test -v ./...`
Expected: PASS on all 16 packages.

- [ ] **Step 2: Commit final documentation / test sync if needed**

```bash
git status
```
