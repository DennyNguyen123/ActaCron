# In-App Direct Updater & Modal Display Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the update modal visibility bug (`.active` vs `.open`) and add a 1-click direct "Update Now" button in the Settings view when a new version is detected.

**Architecture:** Update `web/css/style.css` to support both `.open` and `.active` classes on `.modal-overlay`, standardize `web/js/app.js` to use `.open`, and enhance the Settings General UI row in `web/index.html` and `web/js/app.js` with an inline "Update to vX.X.X" button that seamlessly triggers the update flow. Add unit tests in `web/embed_test.go` to assert correct classes and DOM bindings.

**Tech Stack:** Go 1.24 (embed FS, HTTP handlers), Vanilla HTML5/CSS3/ES6 JavaScript.

**Spec:** [docs/superpowers/specs/2026-10-01-installer-updater-ci-design.md](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-10-01-installer-updater-ci-design.md) (Section 2.5: In-App Updater Web UI).

## Global Constraints

- Do not alter existing Go API route contracts (`/api/version`, `/api/update/check`, `/api/update/apply`).
- Preserve all existing styling tokens and modal structure (`.modal-overlay`, `.modal-box`).
- Ensure no external frontend dependencies or CDNs are introduced.
- Preserve backward compatibility for both `.open` and `.active` modal overlay classes.

## Review Focus

1. **Modal Visibility Mismatch**: If `classList.add("active")` is called, modal must be displayed as `flex` alongside `.open`.
2. **Settings Direct Update Trigger**: When `has_update` is true, an inline primary button "Update to v{tag}" must be visible in Settings row; clicking it should immediately display the update modal or start updating.
3. **Closing and Cancellation**: Closing or canceling the modal via Close/X button must cleanly remove both `.open` and `.active` classes.
4. **Offline / Error State**: If `/api/update/check` or `/api/update/apply` fails, error messages must be rendered gracefully without breaking the settings UI or leaving buttons in a locked disabled state indefinitely.
5. **DOM Token Assertion**: Unit tests in `web/embed_test.go` must verify the presence of the direct update button and correct CSS modal visibility selectors.

---

### Task 1: CSS & Modal Overlay Class Alignment

**Files:**
- Modify: `web/css/style.css:928-932`
- Modify: `web/js/app.js:1569-1575,1605-1608`
- Test: `web/embed_test.go`

**Interfaces:**
- Consumes: `.modal-overlay` container in `web/index.html`
- Produces: Robust CSS selector `.modal-overlay.open, .modal-overlay.active { display: flex; }` and standardized `open` class toggling in `app.js`.

- [ ] **Step 1: Write failing embed test for CSS modal overlay support**

Add a test in `web/embed_test.go` checking that `style.css` contains rule supporting both `.modal-overlay.open` and `.modal-overlay.active`:

```go
func TestCSSModalOverlaySupport(t *testing.T) {
	data, err := web.Assets.ReadFile("css/style.css")
	if err != nil {
		t.Fatalf("failed reading css/style.css: %v", err)
	}
	cssStr := string(data)
	if !strings.Contains(cssStr, ".modal-overlay.open") || !strings.Contains(cssStr, ".modal-overlay.active") {
		t.Fatalf("expected style.css to support both .modal-overlay.open and .modal-overlay.active")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./web -run TestCSSModalOverlaySupport`
Expected: FAIL because `.modal-overlay.active` is not in `web/css/style.css`.

- [ ] **Step 3: Update `web/css/style.css` and `web/js/app.js`**

In `web/css/style.css`, update:
```css
.modal-overlay.open,
.modal-overlay.active {
  display: flex;
}
```

In `web/js/app.js`, standardize update modal open/close to use `.open` (and clean up `.active` if present):
```javascript
function closeUpdateModal() {
  if (modalUpdate) {
    modalUpdate.classList.remove("open", "active");
  }
}
...
if (modalUpdate) {
  modalUpdate.classList.add("open");
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./web -run TestCSSModalOverlaySupport`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/css/style.css web/js/app.js web/embed_test.go
git commit -m "fix(ui): support both open and active classes for modal update overlay"
```

---

### Task 2: Inline Direct Update Action in Settings View

**Files:**
- Modify: `web/index.html:401-410`
- Modify: `web/js/app.js:1540-1615,1625-1635`
- Test: `web/embed_test.go`

**Interfaces:**
- Consumes: `/api/update/check` payload `{has_update: bool, tag_name: string, asset_url: string}`
- Produces: `#btnDirectUpdateSettings` element in `web/index.html` allowing 1-click update trigger directly from Settings.

- [ ] **Step 1: Write failing embed test for direct update button**

In `web/embed_test.go`:
```go
func TestDirectUpdateButtonPresent(t *testing.T) {
	data, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(data)
	if !strings.Contains(htmlStr, `id="btnDirectUpdateSettings"`) {
		t.Fatalf("expected index.html to contain element id=\"btnDirectUpdateSettings\"")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./web -run TestDirectUpdateButtonPresent`
Expected: FAIL because `btnDirectUpdateSettings` is not in `index.html`.

- [ ] **Step 3: Implement direct update button in `web/index.html` and wire in `web/js/app.js`**

In `web/index.html`:
```html
<div class="form-group">
  <label style="font-weight:600;">Application Updates</label>
  <div style="display:flex; align-items:center; justify-content:space-between; margin-top:8px; padding:12px; background:var(--color-surface); border:1px solid var(--color-border); border-radius:6px; gap:12px; flex-wrap:wrap;">
    <div>
      <div style="font-weight:500;">Version: <span id="settingsCurrentVersion" style="color:var(--color-accent); font-family:var(--font-mono);">--</span></div>
      <div class="form-help" id="settingsUpdateStatus">Check for the latest release on GitHub.</div>
    </div>
    <div style="display:flex; gap:8px; align-items:center;">
      <button type="button" class="btn btn-primary btn-sm" id="btnDirectUpdateSettings" style="display:none;">Update Now</button>
      <button type="button" class="btn btn-secondary btn-sm" id="btnCheckUpdateSettings">Check for Updates</button>
    </div>
  </div>
</div>
```

In `web/js/app.js`:
Add reference to `btnDirectUpdate`:
```javascript
const btnDirectUpdate = document.getElementById("btnDirectUpdateSettings");
```
When `data.has_update` is true:
```javascript
if (btnDirectUpdate) {
  btnDirectUpdate.textContent = `Update to ${data.tag_name}`;
  btnDirectUpdate.style.display = "inline-flex";
}
```
When `data.has_update` is false or check fails:
```javascript
if (btnDirectUpdate) {
  btnDirectUpdate.style.display = "none";
}
```
Add click listener for `btnDirectUpdate`:
```javascript
if (btnDirectUpdate) {
  btnDirectUpdate.addEventListener("click", () => {
    if (modalUpdate) {
      modalUpdate.classList.add("open");
    }
  });
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./web -run TestDirectUpdateButtonPresent`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/js/app.js web/embed_test.go
git commit -m "feat(ui): add inline direct update button in settings panel"
```

---

### Task 3: End-to-End Verification & Build Check

**Files:**
- Test: `web/embed_test.go`
- Test: `internal/updater/updater_test.go`
- Test: `internal/api/updater_api_test.go`

**Interfaces:**
- Consumes: Built assets in `web/`
- Produces: Passing full test suite (`go test ./...`) and successful binary build (`go build -o dist/actacron.exe .`).

- [ ] **Step 1: Run full test suite**

Run: `go test -v ./...`
Expected: ALL PASS.

- [ ] **Step 2: Build executable to verify embedded assets compile cleanly**

Run: `go build -o dist/actacron_test_build.exe .`
Expected: Exit code 0, binary created.
Cleanup: Remove `dist/actacron_test_build.exe`.

- [ ] **Step 3: Commit all changes**

```bash
git add .
git commit -m "chore: verify update modal and direct update button"
```
