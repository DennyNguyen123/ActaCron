# Workspace Environment UI Improvements Design Spec

**Date:** 2026-09-30  
**Status:** Approved for Implementation  
**Target Project:** ActaCron (`d:\Personal_Sources\ActaCron`)

---

## 1. Problem Statement

ActaCron supports workspace-scoped environment variables (`packages/<workspace>/.env`) and custom timeouts via `workspace.json`, managed by the `/api/workspace/config` API.

However, the user experience on the frontend makes this feature difficult to discover:
1. In the **Workspace View's script tree** (left pane), the workspace configuration trigger (`⚙️`) is rendered as a minimal, faint emoji button next to the package name without clear labeling or descriptive tooltips.
2. In the **Editor Toolbar** (center pane) and **Script Inspector** (right pane), there is no visual indicator or shortcut for the active workspace's configuration or `.env` variables while viewing or authoring a script.
3. Users looking at the **Settings -> Environment** tab only find global environment variables, leading them to believe workspace-scoped `.env` configuration is unavailable on the UI.

---

## 2. Design Goals

- Make workspace configuration and `.env` management prominently visible and effortlessly accessible across all 3 panes of the Workspace layout:
  1. **Tree Pane (Left):** Distinct, styled action button with explicit tooltip (`Workspace Settings & .env`).
  2. **Editor Toolbar (Center):** Quick-action button (`⚙️ Workspace .env`) visible whenever a non-shared script is open.
  3. **Script Inspector (Right):** Dedicated section indicating the parent workspace name and providing a direct `[⚙️ Config & .env]` trigger.
- Maintain seamless multi-language support (English and Vietnamese) via `web/js/i18n.js`.
- Preserve existing backend APIs (`GET/POST /api/workspace/config`) and behavior for `_shared` packages.

---

## 3. UI/UX Details

### 3.1 Tree Pane Package Actions (`web/js/app.js`, `web/css/style.css`)
- Enhance `.btn-pkg-action` styling:
  - Add explicit subtle border and background: `background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-sm);`
  - Hover state: accent border highlighting `border-color: var(--color-accent);`
  - Update tooltip to localized `Workspace Settings & .env`.

### 3.2 Editor Toolbar Quick Action (`web/index.html`, `web/js/app.js`)
- Add button `#btnActiveWsConfig` to `.editor-actions`:
  - Default: hidden (`display: none`).
  - When a script in package `X` is active (and `X !== "_shared"`), display as `inline-flex`.
  - Clicking this button invokes `openWorkspaceConfigModal(activeScript.package)`.
  - When no script is active or script is in `_shared`, hide button.

### 3.3 Script Inspector Workspace Section (`web/index.html`, `web/js/app.js`)
- Add `#inspectorWsSection` above `Function Metadata`:
  - Shows parent package name (`#inspWsName`) with folder icon.
  - Quick action button `#btnInspWsConfig`: clicking opens `openWorkspaceConfigModal(activeScript.package)`.
  - Automatically hidden when `_shared` script is open or when no script is active.

### 3.4 Localization (`web/js/i18n.js`)
- Add keys:
  - `workspace_env_btn`: EN: "Workspace .env" | VI: "Biến môi trường (.env)"
  - `workspace_info_label`: EN: "Workspace & Environment" | VI: "Workspace & Môi trường"
  - `configure_env`: EN: "⚙️ Config & .env" | VI: "⚙️ Cấu hình & .env"
  - `ws_config_tooltip`: EN: "Workspace Settings & .env" | VI: "Cấu hình Workspace & .env"

---

## 4. Verification Plan

1. Verify `web/embed_test.go` and `go test ./...` pass.
2. Verify HTML and JavaScript syntax and element IDs.
3. Validate opening and closing modal from all 3 entry points (Tree button, Editor button, Inspector button).
4. Verify saving `.env` variables from modal correctly persists to disk and updates runtime.
