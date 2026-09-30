# Workspace Configuration & Shared Library System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable per-workspace custom `.env` and timeout configurations (`workspace.json` and `@timeout` JSDoc) and provide a shared library system (`packages/_shared/`) for DRY functions and variables across all scripts.

**Architecture:** Extend ActaCron's Goja runtime with scoped environment resolution (Workspace > Global > OS) and inject a frozen `shared` namespace sourced from `packages/_shared/`. Refactor execution triggers (API `/api/run`, Cron scheduler, and MCP) to calculate effective timeouts dynamically. Expose workspace configuration endpoints in the REST API and provide an intuitive UI modal and `_shared` tree view in the Web Dashboard.

**Tech Stack:** Go (Goja ECMAScript runtime, standard net/http, SQLite), HTML5/CSS3/Vanilla JavaScript.

**Spec:** [docs/superpowers/specs/2026-09-30-workspace-env-timeout-and-shared-design.md](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-09-30-workspace-env-timeout-and-shared-design.md)

## Global Constraints

- Pure Go with CGO_ENABLED=0 (no external C dependencies).
- Sandboxed execution: Zero path traversal outside `packages/` when calling `require()`.
- Backward compatibility: Existing scripts and packages without `workspace.json` or `.env` must continue to function using global settings.
- All modified endpoints and Go files must pass existing tests (`go test ./...`).

## Review Focus

1. Cyclic calls or path traversal in `require('_shared/...')` attempts escaping `packages/` directory.
2. Workspace `.env` contains special characters or quotes; parsing must not corrupt or lose key-value pairs.
3. Script with invalid/negative `@timeout` or `workspace.json` timeout must fallback safely to global timeout.
4. Auto-scaffold of `packages/_shared/` must be idempotent and not overwrite existing user modifications.
5. Concurrent script executions in different workspaces must not leak or mutate each other's `shared` or `env` data.

---

### Task 1: Domain Models & JSDoc `@timeout` Parser Support

**Files:**
- Modify: `internal/domain/models.go`
- Modify: `internal/parser/parser.go`
- Test: `internal/parser/parser_test.go`

**Interfaces:**
- Consumes: JSDoc string with potential `@timeout <seconds>` tag.
- Produces: `domain.FunctionMeta.TimeoutSeconds int` and `domain.WorkspaceConfig` struct.

- [ ] **Step 1: Write the failing test for `@timeout` parsing in `internal/parser/parser_test.go`**

```go
func TestParse_Timeout(t *testing.T) {
	code := `
/**
 * @name data_sync
 * @timeout 90
 * @description Syncs large datasets with extended timeout
 */
function main(params) {
    return { ok: true };
}
`
	meta, err := Parse(code, "sync.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.TimeoutSeconds != 90 {
		t.Errorf("expected TimeoutSeconds=90, got %d", meta.TimeoutSeconds)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/parser -run TestParse_Timeout`
Expected: FAIL (field `TimeoutSeconds` undefined)

- [ ] **Step 3: Update `domain/models.go` and implement `@timeout` in `parser/parser.go`**

In `internal/domain/models.go`:
```go
type WorkspaceConfig struct {
	Name           string `json:"name"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Description    string `json:"description,omitempty"`
}
```
Add `TimeoutSeconds int` to `domain.FunctionMeta`.

In `internal/parser/parser.go`:
Parse `@timeout <digits>` and assign to `meta.TimeoutSeconds`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/parser -run TestParse_Timeout`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/models.go internal/parser/parser.go internal/parser/parser_test.go
git commit -m "feat(parser): add @timeout JSDoc parsing and WorkspaceConfig model"
```

---

### Task 2: Scoped Environment Resolution & Workspace Config Loader in Manager

**Files:**
- Modify: `internal/engine/helpers.go`
- Modify: `internal/engine/runtime.go`
- Modify: `internal/manager/manager.go`
- Test: `internal/manager/manager_test.go`

**Interfaces:**
- Consumes: `packages/<workspace>/.env`, `packages/<workspace>/workspace.json`, Global settings.
- Produces: `Manager.GetWorkspaceEnv(pkgName)`, `Manager.GetWorkspaceConfig(pkgName)`, `env(key)` resolution prioritizing Workspace > Global > OS, and `env.all()`.

- [ ] **Step 1: Write failing test in `internal/manager/manager_test.go` verifying workspace env overrides global env**

```go
func TestWorkspaceEnvOverride(t *testing.T) {
	// Setup test packages dir with workspace .env overriding global env
	// Verify that calling env("API_KEY") returns the workspace-specific value
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/manager -run TestWorkspaceEnvOverride`
Expected: FAIL

- [ ] **Step 3: Implement scoped env registration and workspace loader**

1. In `internal/engine/helpers.go`:
   Update `registerEnv(vm *goja.Runtime, envResolver func(string) string, allEnv map[string]string)`.
   Provide `env("KEY")`, `env.get("KEY")`, and `env.all()`.
2. In `internal/manager/manager.go`:
   Add `workspaceConfigs map[string]*domain.WorkspaceConfig` and `workspaceEnvs map[string]map[string]string`.
   During `Reload()`, load each workspace's `workspace.json` (if present) and `.env` (via `settings.ReadEnv`).
   Pass the scoped resolver to `r.ExecuteWithSetup`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/manager -run TestWorkspaceEnvOverride`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/helpers.go internal/engine/runtime.go internal/manager/manager.go internal/manager/manager_test.go
git commit -m "feat(engine,manager): implement scoped workspace environment resolution"
```

---

### Task 3: Shared Library System (`packages/_shared/`) Auto-Scaffold & Runtime Injection

**Files:**
- Create: `internal/manager/shared_scaffold.go`
- Modify: `internal/manager/manager.go`
- Test: `internal/manager/shared_test.go`

**Interfaces:**
- Consumes: `packages/_shared/*.js`.
- Produces: Default helper files (`datetime.js`, `notify.js`, `utils.js`, `constants.js`, `index.js`), in-memory compiled module cache, runtime `shared` global object, and safe `require('_shared/...')`.

- [ ] **Step 1: Write failing test in `internal/manager/shared_test.go`**

Verify that:
1. `packages/_shared/` is scaffolded if missing.
2. A script in another package can call `shared.utils.uuid()` or `require('_shared/utils')`.
3. Path traversal attempts like `require('../../secret')` fail with security error.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/manager -run TestSharedLibrary`
Expected: FAIL

- [ ] **Step 3: Implement scaffold and shared loader in `manager`**

1. In `internal/manager/shared_scaffold.go`:
   Implement `ScaffoldSharedLibrary(sharedDir string) error` with standard starter implementations for `datetime.js`, `notify.js`, `utils.js`, `constants.js`, `index.js`.
2. In `internal/manager/manager.go`:
   - Exclude `_shared` from cron parsing/discovery.
   - Compile and cache modules in `_shared`.
   - In `setupFn`, inject `shared` object and enhance `require()` to resolve `_shared/` modules safely.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/manager -run TestSharedLibrary`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/manager/shared_scaffold.go internal/manager/manager.go internal/manager/shared_test.go
git commit -m "feat(manager): implement packages/_shared scaffolding and runtime injection"
```

---

### Task 4: Dynamic Timeout Resolution Across All Execution Triggers

**Files:**
- Modify: `internal/manager/manager.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/scheduler/scheduler.go`
- Modify: `internal/mcp/server.go`
- Test: `internal/manager/timeout_test.go`

**Interfaces:**
- Consumes: `fn.TimeoutSeconds`, `workspaceConfig.TimeoutSeconds`, and `AppSettings.TimeoutSeconds`.
- Produces: `Manager.GetEffectiveTimeout(pkgName, funcName string) time.Duration`.

- [ ] **Step 1: Write failing test for `GetEffectiveTimeout` hierarchy**

Test cases:
- Case A: Function has `@timeout 120` -> returns 120s.
- Case B: Function has no timeout, workspace has 45s -> returns 45s.
- Case C: Neither has timeout -> returns global default (e.g. 30s).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/manager -run TestEffectiveTimeout`
Expected: FAIL

- [ ] **Step 3: Implement `GetEffectiveTimeout` and integrate into callers**

1. In `internal/manager/manager.go`:
   Implement `GetEffectiveTimeout(pkgName, funcName string) time.Duration`.
2. In `internal/api/handlers.go`:
   In `handleRunFunction`, replace hardcoded `35*time.Second` with `h.mgr.GetEffectiveTimeout(reqPkg, reqFunc)`.
3. In `internal/scheduler/scheduler.go`:
   In cron execution callback, replace hardcoded `60*time.Second` with `s.mgr.GetEffectiveTimeout(fn.Package, fn.Name)`.
4. In `internal/mcp/server.go`:
   Use effective timeout for MCP tool calls.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/manager -run TestEffectiveTimeout`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/manager/manager.go internal/api/handlers.go internal/scheduler/scheduler.go internal/mcp/server.go internal/manager/timeout_test.go
git commit -m "feat(timeout): implement dynamic multi-tier timeout resolution for all triggers"
```

---

### Task 5: REST API Endpoints for Workspace Configuration

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- `GET /api/workspace/config?package=<name>` -> `{ timeout_seconds: int, description: string, env: { key: value } }`
- `POST /api/workspace/config` -> `{ package: string, timeout_seconds: int, description: string, env: { key: value } }`

- [ ] **Step 1: Write failing test in `internal/api/api_test.go` for `/api/workspace/config`**

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/api -run TestWorkspaceConfigAPI`
Expected: FAIL (404 Not Found)

- [ ] **Step 3: Implement handlers and route registration**

1. In `internal/api/handlers.go`:
   Implement `handleWorkspaceConfig(w http.ResponseWriter, r *http.Request)`.
   Handles GET (reads `workspace.json` and `.env`) and POST (writes `workspace.json` and `.env`, then triggers `mgr.Reload()`).
2. In `internal/api/router.go`:
   Register `mux.HandleFunc("/api/workspace/config", handler.handleWorkspaceConfig)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/api -run TestWorkspaceConfigAPI`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers.go internal/api/router.go internal/api/api_test.go
git commit -m "feat(api): add GET/POST /api/workspace/config endpoints"
```

---

### Task 6: Web Dashboard UI - Workspace Config Modal & `_shared` Tree View

**Files:**
- Modify: `web/index.html`
- Modify: `web/css/style.css`
- Modify: `web/js/app.js`

**Interfaces:**
- Consumes: `/api/workspace/config`, `/api/packages`.
- Produces: Workspace Settings Modal (Timeout + Env editor) and special tree styling for `_shared`.

- [ ] **Step 1: Add HTML modal in `web/index.html`**

Add `#modalWorkspaceConfig` containing:
- Workspace name display.
- Timeout (seconds) input field.
- Dynamic key-value table for Workspace Environment variables with Add Row and Delete Row buttons.
- Save button calling `POST /api/workspace/config`.

- [ ] **Step 2: Add CSS in `web/css/style.css`**

Add styles for `#modalWorkspaceConfig`, workspace settings gear button next to package header in tree, and special badge for `_shared` package (`.shared-package-badge`).

- [ ] **Step 3: Add JavaScript handlers in `web/js/app.js`**

- In `renderPackagesTree()`: Render `_shared` with `🔗 Shared Library` label and distinct styling.
- Add gear icon button `⚙️` next to each package header.
- On gear click: Fetch `/api/workspace/config?package=...`, populate inputs and table in modal, open modal.
- On save: POST payload to `/api/workspace/config`, show toast notification, refresh tree.

- [ ] **Step 4: Manually verify in browser / subagent test**

Verify modal opens, reads values, saves correctly, and updates `.env` and `workspace.json` on disk.

- [ ] **Step 5: Commit**

```bash
git add web/index.html web/css/style.css web/js/app.js
git commit -m "feat(ui): add Workspace Config modal and _shared tree view"
```

---

### Task 7: Full Integration Testing & Verification

**Files:**
- Test: `tests/workspace_integration_test.go`

- [ ] **Step 1: Write end-to-end integration test**

- Create temporary package `test-pack` with `workspace.json` (timeout=2s) and `.env` (`MY_VAR=hello`).
- Create script `test-pack/main.js` that accesses `env("MY_VAR")`, `shared.utils.uuid()`, and `shared.datetime.nowISO()`.
- Execute via `Manager.CallWithTrigger` and verify output and logs.
- Verify timeout triggers properly if script executes longer than 2s.

- [ ] **Step 2: Run all tests in repository**

Run: `go test -v ./...`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add tests/workspace_integration_test.go
git commit -m "test: add comprehensive workspace and shared library integration tests"
```
