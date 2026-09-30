# Workspace Environment UI Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make workspace-scoped environment (`.env`) configuration prominent and accessible across the Tree Pane, Editor Toolbar, and Script Inspector in the ActaCron Web UI.

**Architecture:** Extend the frontend (`web/index.html`, `web/js/app.js`, `web/css/style.css`, and `web/js/i18n.js`) to provide reactive access points to the existing `openWorkspaceConfigModal(pkgName)` helper, allowing users to configure workspace `.env` directly from the editor toolbar and inspector when a script is selected, while upgrading the tree action buttons with clear styling and tooltips.

**Tech Stack:** Vanilla JavaScript (ES6+), HTML5, CSS3, Go (embed tests).

**Spec:** [docs/superpowers/specs/2026-09-30-workspace-env-ui-improvements-design.md](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-09-30-workspace-env-ui-improvements-design.md)

## Global Constraints

- Must preserve existing API contracts (`GET /api/workspace/config` and `POST /api/workspace/config`).
- Scripts inside `_shared` must never display `.env` configuration buttons (as `_shared` is a shared code library without a workspace `.env`).
- Must support both English (`en`) and Vietnamese (`vi`) localization via `web/js/i18n.js`.
- All Go tests (`go test ./...`) must pass with 0 regressions.

## Review Focus

- **Shared package isolation:** Opening a script from `_shared` must hide `#btnActiveWsConfig` and `#inspectorWsSection`.
- **Deselection state:** Calling `clearActiveScript()` (e.g. on script delete or initial empty state) must hide `#btnActiveWsConfig` and `#inspectorWsSection`.
- **Package switching:** Switching from a script in package `A` to package `B` must update `#inspWsName` and ensure clicking the buttons opens modal for package `B`.
- **Language toggle:** Changing interface language dynamically via Settings must properly translate new labels (`data-i18n`).
- **Modal lifecycle:** Saving or cancelling configuration from the toolbar/inspector triggers must behave identically to the tree action button.

---

### Task 1: Add Localization Keys & Enhance Tree Package Action Styling

**Files:**
- Modify: `web/js/i18n.js:35-90`
- Modify: `web/css/style.css:401-417`
- Test: `web/embed_test.go:1-25`

**Interfaces:**
- Consumes: `window.I18n.translations`
- Produces: `workspace_env_btn`, `workspace_info_label`, `configure_env`, `ws_config_tooltip` keys in `en` and `vi`; `.btn-pkg-action` bordered pill styling.

- [ ] **Step 1: Write failing embed test in `web/embed_test.go`**

Update `web/embed_test.go` to assert required i18n keys and css styles:

```go
package web_test

import (
	"strings"
	"testing"

	"actacron/web"
)

func TestEmbeddedFiles(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil || len(data) == 0 {
		t.Fatalf("expected index.html to be embedded, got err: %v", err)
	}
	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil || len(i18nData) == 0 {
		t.Fatalf("expected js/i18n.js to be embedded, got err: %v", err)
	}
	i18nStr := string(i18nData)
	for _, key := range []string{"workspace_env_btn", "workspace_info_label", "configure_env", "ws_config_tooltip"} {
		if !strings.Contains(i18nStr, key) {
			t.Fatalf("expected i18n.js to contain key %q", key)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/... -v`  
Expected: FAIL with `expected i18n.js to contain key "workspace_env_btn"`

- [ ] **Step 3: Implement translations and styling**

In `web/js/i18n.js`, add the keys to both `en` and `vi` dictionaries:
- `en`:
  ```javascript
  workspace_env_btn: "Workspace .env",
  workspace_info_label: "Workspace & Environment",
  configure_env: "⚙️ Config & .env",
  ws_config_tooltip: "Workspace Settings & .env",
  ```
- `vi`:
  ```javascript
  workspace_env_btn: "Biến môi trường (.env)",
  workspace_info_label: "Workspace & Môi trường",
  configure_env: "⚙️ Cấu hình & .env",
  ws_config_tooltip: "Cấu hình Workspace & .env",
  ```

In `web/css/style.css`, update `.btn-pkg-action`:
```css
.btn-pkg-action {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  cursor: pointer;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  color: var(--color-muted-foreground);
  font-size: 11px;
  line-height: 1.2;
  transition: all 120ms ease;
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.btn-pkg-action:hover {
  background-color: var(--color-surface-hover);
  color: var(--color-foreground);
  border-color: var(--color-accent);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./web/... -v`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/js/i18n.js web/css/style.css web/embed_test.go
git commit -m "feat(ui): add workspace env i18n keys and enhance tree action styling"
```

---

### Task 2: Add Workspace Env UI Elements in Editor Toolbar & Script Inspector

**Files:**
- Modify: `web/index.html:150-205`
- Test: `web/embed_test.go:20-40`

**Interfaces:**
- Consumes: `web/css/style.css`, `web/js/i18n.js`
- Produces: `#btnActiveWsConfig` element in `.editor-actions`, `#inspectorWsSection` with `#inspWsName` and `#btnInspWsConfig` in `.inspector-pane`.

- [ ] **Step 1: Write failing embed test for HTML elements**

In `web/embed_test.go`, add test `TestWorkspaceUIElements`:

```go
func TestWorkspaceUIElements(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(data)
	requiredElements := []string{
		`id="btnActiveWsConfig"`,
		`id="inspectorWsSection"`,
		`id="inspWsName"`,
		`id="btnInspWsConfig"`,
	}
	for _, el := range requiredElements {
		if !strings.Contains(htmlStr, el) {
			t.Fatalf("expected index.html to contain element %s", el)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/... -run TestWorkspaceUIElements -v`  
Expected: FAIL with `expected index.html to contain element id="btnActiveWsConfig"`

- [ ] **Step 3: Update `web/index.html`**

1. In `.editor-actions` (around line 155):
```html
              <div class="editor-actions">
                <button class="btn btn-outline btn-sm" id="btnActiveWsConfig" style="display:none;" title="Workspace Config & .env">
                  ⚙️ <span data-i18n="workspace_env_btn">Workspace .env</span>
                </button>
                <button class="btn btn-secondary btn-sm" id="btnSaveScript" data-i18n="save_script">Save (Ctrl+S)</button>
                <button class="btn btn-primary btn-sm" id="btnRunScript" data-i18n="run_script">Run (Ctrl+Enter)</button>
                <button class="btn btn-outline btn-sm" id="btnDeleteScript" style="display:none; color:var(--color-danger); border-color:rgba(239,68,68,0.35);" data-i18n="delete_script">🗑️ Delete</button>
              </div>
```

2. In `.inspector-pane` (around line 180, before `Function Metadata`):
```html
            <div class="inspector-section" id="inspectorWsSection" style="display:none;">
              <span class="inspector-label" data-i18n="workspace_info_label">Workspace & Environment</span>
              <div style="display:flex; justify-content:space-between; align-items:center; background:var(--color-surface); padding:6px 10px; border-radius:var(--radius-sm); border:1px solid var(--color-border); margin-top:4px;">
                <div style="display:flex; align-items:center; gap:6px; overflow:hidden;">
                  <span style="font-size:13px;">📁</span>
                  <span id="inspWsName" style="font-size:12px; font-weight:600; color:var(--color-foreground); text-overflow:ellipsis; overflow:hidden; white-space:nowrap;">default</span>
                </div>
                <button class="btn btn-outline btn-sm" id="btnInspWsConfig" style="padding:2px 8px; font-size:11px; white-space:nowrap;" data-i18n="configure_env">⚙️ Config & .env</button>
              </div>
            </div>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./web/... -v`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/embed_test.go
git commit -m "feat(ui): add workspace env buttons to editor toolbar and inspector"
```

---

### Task 3: Implement Client-Side State & Event Handlers in `web/js/app.js`

**Files:**
- Modify: `web/js/app.js:230-240, 315-360, 1070-1090`
- Test: `web/embed_test.go` and `internal/api/api_test.go`

**Interfaces:**
- Consumes: `#btnActiveWsConfig`, `#inspectorWsSection`, `#inspWsName`, `#btnInspWsConfig`, `openWorkspaceConfigModal(pkgName)`
- Produces: Dynamic reactive updates of workspace `.env` buttons across selection, deletion, tree rendering, and click events.

- [ ] **Step 1: Write test assertion for script wiring in `web/embed_test.go`**

Add assertion in `web/embed_test.go`:
```go
func TestAppJSEventWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatalf("failed reading js/app.js: %v", err)
	}
	appStr := string(data)
	for _, token := range []string{"btnActiveWsConfig", "inspectorWsSection", "inspWsName", "btnInspWsConfig"} {
		if !strings.Contains(appStr, token) {
			t.Fatalf("expected js/app.js to reference %s", token)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/... -run TestAppJSEventWiring -v`  
Expected: FAIL with `expected js/app.js to reference btnActiveWsConfig`

- [ ] **Step 3: Implement logic in `web/js/app.js`**

1. In `renderTree()`: update tree button tooltip to use `ws_config_tooltip`:
```javascript
const cfgTooltip = (window.I18n && window.I18n.t("ws_config_tooltip")) || "Workspace Settings & .env";
const actionButtons = `
  <div style="display:flex; align-items:center; gap:4px;">
    ${!isShared ? `<button class="btn-pkg-action btn-pkg-config" title="${cfgTooltip}" data-pkg="${pkgName}">⚙️</button>` : ''}
    <button class="btn-pkg-action btn-pkg-folder" title="Open '${pkgName}' in Explorer" data-pkg="${pkgName}">📁</button>
  </div>
`;
```

2. In `selectScript(fn)`:
```javascript
    // Update active workspace buttons
    const btnActiveWs = document.getElementById("btnActiveWsConfig");
    const inspWsSection = document.getElementById("inspectorWsSection");
    const inspWsName = document.getElementById("inspWsName");

    if (fn.package && fn.package !== "_shared") {
      if (btnActiveWs) btnActiveWs.style.display = "inline-flex";
      if (inspWsSection) inspWsSection.style.display = "block";
      if (inspWsName) inspWsName.textContent = fn.package;
    } else {
      if (btnActiveWs) btnActiveWs.style.display = "none";
      if (inspWsSection) inspWsSection.style.display = "none";
    }
```

3. In `clearActiveScript()`:
```javascript
    const btnActiveWs = document.getElementById("btnActiveWsConfig");
    if (btnActiveWs) btnActiveWs.style.display = "none";
    const inspWsSection = document.getElementById("inspectorWsSection");
    if (inspWsSection) inspWsSection.style.display = "none";
```

4. In `setupEventListeners()`:
```javascript
    const btnActiveWs = document.getElementById("btnActiveWsConfig");
    if (btnActiveWs) {
      btnActiveWs.addEventListener("click", () => {
        if (activeScript && activeScript.package && activeScript.package !== "_shared") {
          openWorkspaceConfigModal(activeScript.package);
        }
      });
    }

    const btnInspWs = document.getElementById("btnInspWsConfig");
    if (btnInspWs) {
      btnInspWs.addEventListener("click", () => {
        if (activeScript && activeScript.package && activeScript.package !== "_shared") {
          openWorkspaceConfigModal(activeScript.package);
        }
      });
    }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`  
Expected: PASS all packages

- [ ] **Step 5: Commit**

```bash
git add web/js/app.js web/embed_test.go
git commit -m "feat(ui): bind workspace env actions to editor toolbar and inspector"
```

---

### Task 4: End-to-End Verification & Sanity Check

**Files:**
- Test: `tests/e2e/e2e_test.go`
- Verification: manual verification via CLI/API check

- [ ] **Step 1: Run complete test suite**

Run: `go test -v ./...`  
Expected: All tests pass with no errors or regressions.

- [ ] **Step 2: Verification of workspace config API and UI bindings**

Run:
```powershell
go test -v ./internal/api -run TestWorkspaceConfigAPI
```
Expected: PASS

- [ ] **Step 3: Verification of build artifacts**

Run:
```powershell
go build -o dist/actacron_test.exe .
```
Verify binary builds cleanly without any compiler or embed errors. Remove test binary afterwards.
