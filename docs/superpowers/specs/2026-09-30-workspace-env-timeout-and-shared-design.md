# Workspace Configuration & Shared Library System Design

**Date:** 2026-09-30
**Status:** Approved for Implementation
**Target Project:** ActaCron (d:\Personal_Sources\ActaCron)

---

## 1. Problem Statement & Motivation

Currently in ActaCron:
1. All workspaces (packages) share a single global `.env` file and a global timeout configuration (default 30 seconds, or hardcoded timeouts like 35s in API handler and 60s in scheduler). Workspaces cannot specify their own environment variables or custom timeouts.
2. Scripts across workspaces duplicate utility code, formatters, and notification webhooks (violating DRY principles). There is no standard shared library mechanism.

---

## 2. Proposed Architecture & Key Components

### 2.1 Workspace Configuration (`.env` and `workspace.json`)

Each workspace resides in `packages/<workspace_name>/`. It can optionally contain:
- `workspace.json`:
  ```json
  {
    "timeout_seconds": 60,
    "description": "Custom workspace for data pipelines"
  }
  ```
- `.env`: Standard KEY=VALUE pairs specific to this workspace.

#### Multi-Tier Hierarchy:
1. **Environment Variables Resolution:**
   - Priority 1: `packages/<workspace>/.env`
   - Priority 2: Global `.env` (`settingsSvc.GetEnv()`)
   - Priority 3: System OS environment (`os.Getenv`)
   - In JS runtime: `env(key)` and `env.get(key)` query this merged map.
   - `env.all()` returns the full merged key-value map for the current workspace.

2. **Timeout Resolution:**
   - Priority 1: JSDoc annotation on function `@timeout <seconds>` (e.g. `@timeout 120`).
   - Priority 2: Workspace `workspace.json` -> `timeout_seconds`.
   - Priority 3: Global AppSettings (`TimeoutSeconds`, default 30s).
   - Applied dynamically across all execution triggers: Web UI manual run (`/api/run`), Cron Scheduler, and MCP tool invocations.

---

### 2.2 Shared Library System (`packages/_shared/`)

A special package directory `packages/_shared/` holds reusable JavaScript modules and constants.

1. **Auto-Scaffolding on Boot:**
   If `packages/_shared/` does not exist on app startup, ActaCron automatically scaffolds standard starter files:
   - `datetime.js`: `format(date, pattern)`, `nowISO()`, `timeAgo(date)`, `addDays(date, n)`, `startOfDay(date)`.
   - `notify.js`: `telegram({ botToken, chatId, message })`, `discord({ webhookUrl, content })`, `slack({ webhookUrl, text })`, `webhook(url, payload, headers)`.
   - `utils.js`: `retry(fn, options)`, `uuid()`, `chunk(arr, size)`, `safeJson(str, defaultVal)`, `formatBytes(bytes)`, `formatCurrency(amount)`.
   - `constants.js`: `HTTP_STATUS`, `REGEX`, `TIME`.
   - `index.js`: Aggregate exports re-exporting modules.

2. **Runtime Injection & Module Loading:**
   - Every script execution receives a frozen/isolated `shared` global object populated with exports from `_shared`:
     - Sub-namespaces: `shared.datetime.*`, `shared.notify.*`, `shared.utils.*`, `shared.constants.*`.
     - Direct exports from `_shared/index.js` (if present) merged into `shared.*`.
   - Sandbox `require()` is enhanced:
     - Allows `require('_shared/utils')` or `require('../_shared/datetime')`.
     - Path traversal checks ensure access cannot escape the root `packages/` directory.

3. **Parser & Manager Isolation:**
   - `packages/_shared/` is identified as a special library package:
     - Files inside `_shared/` are not parsed as standalone cron jobs or MCP tools (they do not require `main(params)`).
     - On file changes in `_shared/`, hot reload automatically invalidates and refreshes the shared modules cache.

---

### 2.3 Web Dashboard UI Integration

1. **Packages Tree:**
   - `packages/_shared` is rendered at the top of the tree with a distinct icon and badge (`🔗 Shared Library`).
   - Clicking files in `_shared` opens them in the editor for editing, saving (`Ctrl+S`), and syntax checking.

2. **Workspace Settings UI:**
   - In the package tree or toolbar, each workspace has a **"⚙️ Workspace Config"** action.
   - Opens a modal/drawer displaying:
     - **Timeout (seconds)** (reads/writes `workspace.json`).
     - **Workspace Environment (.env)** (table of key-value pairs, reads/writes `packages/<workspace>/.env`).

---

### 2.4 Data Models & Backend Changes

- **`internal/domain/models.go`:**
  - Add `WorkspaceConfig` struct:
    ```go
    type WorkspaceConfig struct {
        Name           string `json:"name"`
        TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
        Description    string `json:"description,omitempty"`
    }
    ```
  - Add `TimeoutSeconds int` to `FunctionMeta` (parsed from `@timeout <n>`).

- **`internal/parser/parser.go`:**
  - Support `@timeout` tag in JSDoc parsing.

- **`internal/engine/helpers.go` & `internal/engine/runtime.go`:**
  - `registerEnv` updated to take `envResolver func(string) string` and `allEnv map[string]string`.
  - Add `env.all()`.
  - Support injecting `shared` object during `setupFn`.

- **`internal/manager/manager.go`:**
  - Manage `workspaceConfigs map[string]*domain.WorkspaceConfig` and `workspaceEnvs map[string]map[string]string`.
  - Scan and compile `packages/_shared/` files into shared objects.
  - Compute `GetEffectiveTimeout(pkgName, funcName string) time.Duration`.
  - Update `callWithTrigger` to execute with effective timeout and scoped env.
  - Update `require()` to resolve `_shared/` references safely.

- **`internal/api/handlers.go` & `internal/api/router.go`:**
  - Add endpoints:
    - `GET /api/workspace/config?package=<name>`
    - `POST /api/workspace/config` (save `workspace.json` and `.env`)
  - Fix `/api/run` to use `mgr.GetEffectiveTimeout(pkg, fn)` instead of hardcoded 35s.

- **`internal/scheduler/scheduler.go`:**
  - Use `s.mgr.GetEffectiveTimeout(pkg, fn)` instead of hardcoded 60s.

---

## 3. Testing Strategy

1. **Unit Tests:**
   - `internal/parser/parser_test.go`: Verify `@timeout` parsing.
   - `internal/settings/dotenv_test.go`: Verify reading/writing workspace `.env`.
   - `internal/manager/manager_test.go`:
     - Test workspace env override over global env.
     - Test timeout resolution (JSDoc > workspace.json > global).
     - Test `shared` object availability and `require('_shared/...')`.
     - Test security boundary on `require` path traversal.
2. **Integration Tests:**
   - Execute a function in a test package that accesses `env("LOCAL_KEY")`, `shared.utils.uuid()`, and `shared.datetime.nowISO()`.
3. **Frontend Verification:**
   - Test workspace config modal, save env and timeout, verify file contents on disk.
