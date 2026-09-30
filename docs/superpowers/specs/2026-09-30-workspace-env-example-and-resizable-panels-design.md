# Workspace .env.example Auto-Fallback & Resizable Panels Design Spec

**Date:** 2026-09-30  
**Status:** Approved for Implementation  
**Target Project:** ActaCron (`d:\Personal_Sources\ActaCron`)

---

## 1. Problem Statement

1. **Missing .env on Fresh / Cloned Workspaces:**
   When cloning or adding a new workspace package (e.g. `ActaCron_Redmine2Vikunja`), version control contains `.env.example`, but `.env` is git-ignored and not yet created. Currently, opening the **Workspace Configuration** modal displays an empty environment table, forcing users to manually type every configuration key.
2. **Clipped Action Buttons & Fixed Layout:**
   The workspace layout currently uses fixed-width panes (`.tree-pane: 250px`, `.inspector-pane: 280px`). Long package names (e.g. `ActaCron_Redmine2Vikunja`) overflow the tree pane, pushing the package action buttons (`⚙️` and `📁`) out of view. Users cannot adjust pane widths to fit their screen or directory naming styles.

---

## 2. Design Goals

1. **Automatic .env.example Fallback:**
   - If a package does not have `.env` but has `.env.example`, the backend manager and `/api/workspace/config` endpoint will automatically load variables from `.env.example`.
   - The Workspace Configuration modal will pre-populate these keys and sample values, displaying a subtle indicator badge (`💡 Pre-filled from .env.example`).
   - Add a **"📄 Load from .env.example"** button to allow users to reload keys from the example file at any time.
2. **Resizable 3-Pane Layout:**
   - Add draggable dividers (`.layout-resizer`) between:
     - Tree Pane $\leftrightarrow$ Editor Pane
     - Editor Pane $\leftrightarrow$ Inspector Pane
   - Persist user-adjusted widths in `localStorage` (`actacron_tree_width`, `actacron_inspector_width`).
3. **Flexbox Truncation & Button Pinning:**
   - Ensure package and script titles use `min-width: 0`, `overflow: hidden`, `text-overflow: ellipsis`, and `white-space: nowrap`.
   - Action buttons (`⚙️`, `📁`, `🗑️`) use `flex-shrink: 0`, ensuring they remain permanently visible regardless of pane width or name length.

---

## 3. Architecture & Interfaces

### 3.1 Backend Changes (`internal/manager`, `internal/api`)
- **`internal/manager/manager.go`:**
  - During package scanning, if `packages/<pkg>/.env` does not exist, check for `packages/<pkg>/.env.example`. If found, load it into `m.workspaceEnvs[pkg]`.
  - Add helper `HasWorkspaceEnvFile(pkgName string) bool` and `GetWorkspaceExampleEnv(pkgName string) map[string]string`.
- **`internal/api/handlers.go`:**
  - In `handleWorkspaceConfig` (GET):
    Include `has_env_file` (bool), `is_from_example` (bool), and `example_env` (map) in the JSON payload.
  - In `handleWorkspaceConfig` (POST):
    Saving always writes to `packages/<pkg>/.env`.

### 3.2 Frontend Changes (`web/index.html`, `web/js/app.js`, `web/css/style.css`, `web/js/i18n.js`)
- **Workspace Config Modal:**
  - Add button `#btnLoadWsEnvExample` next to `#btnAddWsEnvRow`.
  - When loading config, if `is_from_example` is true, show hint banner `#wsEnvExampleHint`.
- **Resizable Layout:**
  - Add `<div class="layout-resizer" id="resizerLeft"></div>` and `<div class="layout-resizer" id="resizerRight"></div>`.
  - Mouse event listeners in `app.js` adjust `treePane.style.width` and `inspectorPane.style.width` within constraints (min 200px, max 600px).
- **Styling:**
  - Add `.layout-resizer` styling with hover and active states.
  - Set `text-overflow: ellipsis` and `flex-shrink: 0` on tree headers and file items.

---

## 4. Verification Plan

1. Backend Unit Tests (`internal/manager/manager_test.go`, `internal/api/api_test.go`):
   - Test loading workspace when only `.env.example` exists.
   - Test `GET /api/workspace/config` returning `is_from_example: true` when `.env` is absent.
   - Test saving writes `.env` and subsequent GET returns `is_from_example: false`.
2. Web Embed Tests (`web/embed_test.go`):
   - Test presence of `#resizerLeft`, `#resizerRight`, `#btnLoadWsEnvExample`, and i18n keys.
3. Full Suite Test: `go test ./...` with 100% pass.
