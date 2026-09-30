# Workspace .env.example Auto-Fallback & Resizable Panels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically load environment variables from `.env.example` when `.env` is missing during workspace setup, and add resizable splitters with robust flexbox truncation to the workspace layout.

**Architecture:** Update the Go backend (`internal/manager` and `internal/api`) to detect `.env.example` as a fallback when `.env` is absent, reporting example status and sample keys. On the frontend (`web/index.html`, `web/js/app.js`, `web/css/style.css`), add draggable resizer dividers to adjust pane widths dynamically with `localStorage` persistence, and introduce a `.env.example` prefill notice and manual reload button in the Workspace Config modal.

**Tech Stack:** Go 1.22+, Vanilla JavaScript (ES6+), HTML5, CSS3, Goja runtime.

**Spec:** [docs/superpowers/specs/2026-09-30-workspace-env-example-and-resizable-panels-design.md](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-09-30-workspace-env-example-and-resizable-panels-design.md)

## Global Constraints

- Must preserve existing API contracts (`GET /api/workspace/config` and `POST /api/workspace/config`).
- When saving configuration via `POST /api/workspace/config`, it must always write to `packages/<pkg>/.env`, converting the example state into a real `.env` file.
- Resizable panels must constrain minimum width to 200px and maximum width to 600px to avoid breaking layout readability.
- Multi-language support (English and Vietnamese) must cover all new UI notices and buttons.
- All existing and new tests (`go test ./...`) must pass with 0 regressions.

## Review Focus

- **Fresh Workspace Detection:** Packages with only `.env.example` must return `is_from_example: true` on GET and prefill the table with example values.
- **Real .env Override:** Once `.env` is created/saved, `.env` takes absolute priority over `.env.example`, returning `is_from_example: false`.
- **Manual Reload Button:** Clicking `Load from .env.example` in the modal must repopulate the table with `.env.example` values even if the user had modified rows.
- **Tree Action Truncation:** Long package names (e.g. `ActaCron_Redmine2Vikunja`) must truncate with `...` while keeping `⚙️` and `📁` buttons completely visible.
- **Divider Drag Constraints:** Dragging divider handles left or right must resize smoothly, unselect text during drag, and persist widths to `localStorage`.

---

### Task 1: Backend Fallback to `.env.example` & API Enhancements

**Files:**
- Modify: `internal/manager/manager.go:140-155, 610-630`
- Modify: `internal/api/handlers.go:535-560`
- Test: `internal/manager/manager_test.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `settings.ReadEnv`
- Produces: `GetWorkspaceExampleEnv(pkgName string) map[string]string`, `HasWorkspaceEnvFile(pkgName string) bool`, and updated `/api/workspace/config` GET response containing `is_from_example` and `example_env`.

- [ ] **Step 1: Write failing tests in `internal/manager/manager_test.go` and `internal/api/api_test.go`**

In `internal/manager/manager_test.go`:
```go
func TestWorkspaceEnvExampleFallback(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "examplepkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .env.example with sample keys
	exampleContent := "SAMPLE_KEY=sample_val\nAPI_HOST=https://api.test\n"
	if err := os.WriteFile(filepath.Join(pkgDir, ".env.example"), []byte(exampleContent), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := New(tmpDir, t.TempDir(), nil, nil)
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	// Should fallback to .env.example since .env is missing
	env := mgr.GetWorkspaceEnv("examplepkg")
	if env["SAMPLE_KEY"] != "sample_val" || env["API_HOST"] != "https://api.test" {
		t.Fatalf("expected env from .env.example, got: %v", env)
	}
	if mgr.HasWorkspaceEnvFile("examplepkg") {
		t.Fatalf("expected HasWorkspaceEnvFile to be false")
	}

	// Once .env is created, it should take precedence
	realContent := "SAMPLE_KEY=real_val\n"
	if err := os.WriteFile(filepath.Join(pkgDir, ".env"), []byte(realContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}
	env = mgr.GetWorkspaceEnv("examplepkg")
	if env["SAMPLE_KEY"] != "real_val" || env["API_HOST"] != "" {
		t.Fatalf("expected real .env to override, got: %v", env)
	}
	if !mgr.HasWorkspaceEnvFile("examplepkg") {
		t.Fatalf("expected HasWorkspaceEnvFile to be true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/manager -run TestWorkspaceEnvExampleFallback`  
Expected: FAIL (`mgr.HasWorkspaceEnvFile undefined` or `expected env from .env.example`)

- [ ] **Step 3: Implement minimal code in `manager.go` and `handlers.go`**

In `internal/manager/manager.go`:
```go
		// 2. Read workspace .env if present, otherwise fallback to .env.example
		wsEnvPath := filepath.Join(pkgPath, ".env")
		if envMap, err := settings.ReadEnv(wsEnvPath); err == nil && len(envMap) > 0 {
			newWorkspaceEnvs[pkgName] = envMap
		} else {
			wsExamplePath := filepath.Join(pkgPath, ".env.example")
			if exMap, err := settings.ReadEnv(wsExamplePath); err == nil && len(exMap) > 0 {
				newWorkspaceEnvs[pkgName] = exMap
			}
		}
```
Add helper functions to `internal/manager/manager.go`:
```go
func (m *Manager) HasWorkspaceEnvFile(pkgName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pkgPath := filepath.Join(m.packagesDir, pkgName)
	_, err := os.Stat(filepath.Join(pkgPath, ".env"))
	return err == nil
}

func (m *Manager) GetWorkspaceExampleEnv(pkgName string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pkgPath := filepath.Join(m.packagesDir, pkgName)
	exPath := filepath.Join(pkgPath, ".env.example")
	if exMap, err := settings.ReadEnv(exPath); err == nil {
		return exMap
	}
	return make(map[string]string)
}
```

In `internal/api/handlers.go` (`handleWorkspaceConfig` GET case):
```go
		wsCfg := h.mgr.GetWorkspaceConfig(pkgName)
		wsEnv := h.mgr.GetWorkspaceEnv(pkgName)
		hasEnvFile := h.mgr.HasWorkspaceEnvFile(pkgName)
		exampleEnv := h.mgr.GetWorkspaceExampleEnv(pkgName)

		res := map[string]interface{}{
			"package":         pkgName,
			"timeout_seconds": wsCfg.TimeoutSeconds,
			"description":     wsCfg.Description,
			"env":             wsEnv,
			"has_env_file":    hasEnvFile,
			"is_from_example": !hasEnvFile && len(wsEnv) > 0,
			"example_env":     exampleEnv,
		}
		writeJSON(w, http.StatusOK, res)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/manager ./internal/api`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/manager/manager.go internal/manager/manager_test.go internal/api/handlers.go internal/api/api_test.go
git commit -m "feat(api): fallback to .env.example for workspace configuration when .env is absent"
```

---

### Task 2: Layout Resizer Elements & Truncation Styles

**Files:**
- Modify: `web/index.html:130-185, 525-545`
- Modify: `web/css/style.css:315-380, 400-440`
- Modify: `web/js/i18n.js:35-90, 140-195`
- Test: `web/embed_test.go`

**Interfaces:**
- Consumes: `web/css/style.css`, `web/js/i18n.js`
- Produces: `#resizerLeft`, `#resizerRight`, `#btnLoadWsEnvExample`, `#wsEnvExampleHint`, `.layout-resizer` CSS styles, and ellipsis truncation styles.

- [ ] **Step 1: Write failing embed test in `web/embed_test.go`**

Add test assertions in `web/embed_test.go`:
```go
func TestResizableLayoutAndExampleEnvElements(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, el := range []string{`id="resizerLeft"`, `id="resizerRight"`, `id="btnLoadWsEnvExample"`, `id="wsEnvExampleHint"`} {
		if !strings.Contains(html, el) {
			t.Fatalf("expected index.html to contain %s", el)
		}
	}

	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	i18nStr := string(i18nData)
	for _, key := range []string{"load_from_example", "env_example_hint"} {
		if !strings.Contains(i18nStr, key) {
			t.Fatalf("expected js/i18n.js to contain translation key %s", key)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./web/... -run TestResizableLayoutAndExampleEnvElements`  
Expected: FAIL

- [ ] **Step 3: Update `web/index.html`, `web/css/style.css`, and `web/js/i18n.js`**

1. In `web/index.html`:
Add `#resizerLeft` between `.tree-pane` and `.editor-pane`, and `#resizerRight` between `.editor-pane` and `.inspector-pane`:
```html
        <div class="workspace-layout">
          <!-- Tree Pane (Left) -->
          <aside class="tree-pane" id="treePane">...</aside>

          <!-- Resizer Left -->
          <div class="layout-resizer" id="resizerLeft"></div>

          <!-- Editor Pane (Center) -->
          <div class="editor-pane">...</div>

          <!-- Resizer Right -->
          <div class="layout-resizer" id="resizerRight"></div>

          <!-- Inspector Pane (Right) -->
          <aside class="inspector-pane" id="inspectorPane">...</aside>
        </div>
```

In `modalWorkspaceConfig` (around line 535):
```html
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom: 6px;">
            <label style="margin-bottom:0;">Workspace Environment (.env)</label>
            <div style="display:flex; gap:6px;">
              <button class="btn btn-outline btn-sm" id="btnLoadWsEnvExample" style="display:none; padding: 2px 8px; font-size: 11px;" data-i18n="load_from_example">📄 Load from .env.example</button>
              <button class="btn btn-secondary btn-sm" id="btnAddWsEnvRow" style="padding: 2px 8px; font-size: 11px;">+ Add Variable</button>
            </div>
          </div>
          <div id="wsEnvExampleHint" style="display:none; background:rgba(56,189,248,0.1); border:1px solid rgba(56,189,248,0.3); border-radius:var(--radius-sm); padding:6px 10px; margin-bottom:8px; font-size:11px; color:#38bdf8;" data-i18n="env_example_hint">
            💡 Pre-filled with template values from .env.example. Save configuration to persist to .env.
          </div>
```

2. In `web/css/style.css`:
```css
.layout-resizer {
  width: 5px;
  cursor: col-resize;
  background: transparent;
  z-index: 10;
  transition: background 150ms ease;
  margin: 0 -2px;
  flex-shrink: 0;
  user-select: none;
}

.layout-resizer:hover,
.layout-resizer.dragging {
  background: var(--color-accent);
}

.tree-pane {
  min-width: 200px;
  max-width: 600px;
}

.inspector-pane {
  min-width: 200px;
  max-width: 600px;
}
```

3. In `web/js/i18n.js`:
Add keys:
- `en`:
  ```javascript
  load_from_example: "📄 Load .env.example",
  env_example_hint: "💡 Pre-filled with template values from .env.example. Save configuration to persist to .env.",
  ```
- `vi`:
  ```javascript
  load_from_example: "📄 Tải từ .env.example",
  env_example_hint: "💡 Đã tự động điền giá trị mẫu từ .env.example. Bấm Lưu cấu hình để tạo tệp .env.",
  ```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./web/...`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/css/style.css web/js/i18n.js web/embed_test.go
git commit -m "feat(ui): add layout resizer elements and .env.example modal controls"
```

---

### Task 3: Client JavaScript Logic (Draggable Resizers & .env.example Prefill/Button)

**Files:**
- Modify: `web/js/app.js:225-245, 275-300, 1080-1170`
- Test: `web/embed_test.go`

**Interfaces:**
- Consumes: `#resizerLeft`, `#resizerRight`, `#btnLoadWsEnvExample`, `#wsEnvExampleHint`
- Produces: Smooth resizing of tree and inspector panes with localStorage saving, robust flexbox ellipsis on tree items, and interactive `.env.example` prefill and reload.

- [ ] **Step 1: Write failing embed test for app.js wiring**

In `web/embed_test.go`:
```go
func TestAppJSResizerAndExampleEnvWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	appStr := string(data)
	for _, token := range []string{"initResizers", "btnLoadWsEnvExample", "wsEnvExampleHint", "actacron_tree_width"} {
		if !strings.Contains(appStr, token) {
			t.Fatalf("expected js/app.js to reference %s", token)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./web/... -run TestAppJSResizerAndExampleEnvWiring`  
Expected: FAIL

- [ ] **Step 3: Implement client logic in `web/js/app.js`**

1. In `renderTree()`:
Update `headerTitle` and `actionButtons` to enforce `min-width: 0`, `overflow: hidden`, `text-overflow: ellipsis`, and `flex-shrink: 0`:
```javascript
      const headerTitle = isShared 
        ? `<div style="display:flex; align-items:center; gap:6px; min-width:0; flex:1; overflow:hidden;"><span class="tree-chevron" style="flex-shrink:0;">▾</span><span>🔗</span> <strong style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">_shared</strong> <span class="badge-shared-library" style="flex-shrink:0;">LIB</span></div>`
        : `<div style="display:flex; align-items:center; gap:6px; min-width:0; flex:1; overflow:hidden;"><span class="tree-chevron" style="flex-shrink:0;">▾</span><svg style="flex-shrink:0;" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg><span style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${pkgName}">${pkgName}</span></div>`;

      const cfgTooltip = (window.I18n && window.I18n.t("ws_config_tooltip")) || "Workspace Settings & .env";
      const actionButtons = `
        <div style="display:flex; align-items:center; gap:4px; flex-shrink:0;">
          ${!isShared ? `<button class="btn-pkg-action btn-pkg-config" data-i18n-title="ws_config_tooltip" title="${cfgTooltip}" data-pkg="${pkgName}">⚙️</button>` : ''}
          <button class="btn-pkg-action btn-pkg-folder" title="Open '${pkgName}' in Explorer" data-pkg="${pkgName}">📁</button>
        </div>
      `;
```

And in script items:
```javascript
        itemLi.innerHTML = `
          <span style="overflow:hidden; text-overflow:ellipsis; white-space:nowrap; min-width:0; flex:1; margin-right:6px;" title="${fn.name}">${fn.name}</span>
          <div style="display:flex; gap:4px; align-items:center; flex-shrink:0;">
            ${badges.join("")}
            ${!isShared ? `<button class="btn-tree-delete" title="Delete script" data-pkg="${fn.package}" data-name="${fn.name}">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
            </button>` : ''}
          </div>
        `;
```

2. In `openWorkspaceConfigModal(pkgName)`:
Store `currentExampleEnv = data.example_env || {}`:
```javascript
        if (data.is_from_example) {
          document.getElementById("wsEnvExampleHint").style.display = "block";
        } else {
          document.getElementById("wsEnvExampleHint").style.display = "none";
        }

        const btnLoadEx = document.getElementById("btnLoadWsEnvExample");
        if (btnLoadEx) {
          if (data.example_env && Object.keys(data.example_env).length > 0) {
            btnLoadEx.style.display = "inline-flex";
            btnLoadEx.onclick = () => {
              renderWsEnvRows(data.example_env);
              document.getElementById("wsEnvExampleHint").style.display = "block";
            };
          } else {
            btnLoadEx.style.display = "none";
          }
        }
```

3. Add `initResizers()` function:
```javascript
  function initResizers() {
    const treePane = document.getElementById("treePane");
    const inspectorPane = document.getElementById("inspectorPane");
    const resizerLeft = document.getElementById("resizerLeft");
    const resizerRight = document.getElementById("resizerRight");

    const savedTreeW = localStorage.getItem("actacron_tree_width");
    if (savedTreeW && treePane) treePane.style.width = savedTreeW + "px";

    const savedInspW = localStorage.getItem("actacron_inspector_width");
    if (savedInspW && inspectorPane) inspectorPane.style.width = savedInspW + "px";

    if (resizerLeft && treePane) {
      let isDragging = false;
      resizerLeft.addEventListener("mousedown", (e) => {
        isDragging = true;
        resizerLeft.classList.add("dragging");
        document.body.style.cursor = "col-resize";
        document.body.style.userSelect = "none";
      });

      window.addEventListener("mousemove", (e) => {
        if (!isDragging) return;
        const newW = Math.max(200, Math.min(600, e.clientX));
        treePane.style.width = newW + "px";
        localStorage.setItem("actacron_tree_width", newW);
      });

      window.addEventListener("mouseup", () => {
        if (isDragging) {
          isDragging = false;
          resizerLeft.classList.remove("dragging");
          document.body.style.cursor = "";
          document.body.style.userSelect = "";
        }
      });
    }

    if (resizerRight && inspectorPane) {
      let isDragging = false;
      resizerRight.addEventListener("mousedown", (e) => {
        isDragging = true;
        resizerRight.classList.add("dragging");
        document.body.style.cursor = "col-resize";
        document.body.style.userSelect = "none";
      });

      window.addEventListener("mousemove", (e) => {
        if (!isDragging) return;
        const newW = Math.max(200, Math.min(600, window.innerWidth - e.clientX));
        inspectorPane.style.width = newW + "px";
        localStorage.setItem("actacron_inspector_width", newW);
      });

      window.addEventListener("mouseup", () => {
        if (isDragging) {
          isDragging = false;
          resizerRight.classList.remove("dragging");
          document.body.style.cursor = "";
          document.body.style.userSelect = "";
        }
      });
    }
  }
```

Invoke `initResizers()` inside `DOMContentLoaded` in `web/js/app.js`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./web/...`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/js/app.js web/embed_test.go
git commit -m "feat(ui): implement draggable panel resizers and .env.example prefill wiring"
```

---

### Task 4: End-to-End Verification & Sanity Check

**Files:**
- Test: All tests in `./...`
- Verify with real package `ActaCron_Redmine2Vikunja` containing `.env.example`

- [ ] **Step 1: Run complete test suite**

Run: `go test -v ./...`  
Expected: PASS all packages

- [ ] **Step 2: Verify binary compilation**

Run:
```powershell
go build -o dist/actacron_test.exe .
```
Expected: Exits with 0, clean build. Remove temporary binary afterwards.
