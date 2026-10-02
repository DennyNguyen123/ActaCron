# External Workspace Folders Design

## Overview
Currently, ActaCron only discovers and loads packages located inside the local `packages/` directory (`m.packagesDir`). Users who already have projects, repositories, or automation scripts located in external directories cannot directly use them without manually copying them into `packages/`.

This feature allows users to select or enter any existing folder on their filesystem and use it directly as an active Workspace in ActaCron.
**Strict Constraint:** Files and directories from the chosen folder are NEVER copied into the `packages/` directory. ActaCron reads, runs, saves, watches (`fsnotify`), and configures (`.env`, `workspace.json`) scripts in-place directly from the external folder path.

## Requirements

1. **Storage & Persistence**:
   - Store registered external workspaces in SQLite (`external_workspaces` table).
   - Columns: `name TEXT PRIMARY KEY`, `path TEXT NOT NULL`, `created_at DATETIME`.
   - On application startup or `Manager.Reload()`, ActaCron loads both internal packages (from `packagesDir`) and all registered external workspaces from the database.

2. **Manager Runtime & Package Path Resolution**:
   - `domain.PackageInfo` gains `IsExternal bool`.
   - Refactor `Manager` to decouple package paths from hardcoded `filepath.Join(m.packagesDir, pkgName)`:
     - `GetPackageDir(pkgName string) (string, error)` returns `pkg.Path` if exists, or defaults to `filepath.Join(m.packagesDir, pkgName)`.
     - Script saving, reading, deleting, `.env` loading, and `workspace.json` loading use the resolved package directory.
     - Module sandbox resolution in `require()` permits paths that are either inside the package's resolved root directory or inside `packages/_shared`.
     - File watcher (`fsnotify`) adds watch paths for each external workspace folder in addition to `packagesDir`.

3. **API Endpoints**:
   - `POST /api/workspace/pick-folder`: Opens native OS folder picker (Windows FolderBrowserDialog, macOS osascript, Linux zenity) and returns the chosen directory path `{ "path": "..." }`.
   - `GET /api/workspace/external`: Lists all registered external workspaces.
   - `POST /api/workspace/external`: Registers a new external workspace `{ "name": "...", "path": "..." }`. Validates that the path exists and is a directory. Automatically reloads manager.
   - `DELETE /api/workspace/external`: Unlinks/removes an external workspace by name (`?name=...`). Removes registration from DB, stops watching, reloads manager. Does NOT delete files from the disk.

4. **Web UI**:
   - In the "Packages & Scripts" pane header, add a `+ Folder` button (alongside `+ Script`, `+ Git`, `📁 Open Dir`).
   - Add a modal for selecting/adding external folder:
     - "Folder Path" input field.
     - "Browse..." button that triggers `/api/workspace/pick-folder`.
     - "Workspace Name" input (auto-populated with folder basename, editable).
     - Validation and error display.
   - In the scripts tree, display external workspaces with a distinct external icon/badge (e.g. 🔗 or `[ext]`).
   - For external workspaces, provide an "Unlink" action button with confirmation dialog explaining that files will not be deleted from disk.
   - I18n support in `web/js/i18n.js` (English & Vietnamese).

## Security & Architecture Considerations
- **Path Traversal & Sandboxing**: `require()` within an external package must be restricted to within that package's path, or `_shared/`.
- **Duplicate Names**: Workspace names must be unique. If an external workspace name clashes with an internal package or another external workspace, an error is returned.
- **Missing Folders**: If an external folder was moved or deleted while ActaCron was closed, `Manager.Reload()` marks it with an error or skips loading scripts gracefully without crashing.
