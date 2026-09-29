# ActaCron Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build ActaCron, an ultra-lightweight Go application with dynamic JS scripting, multi-Git package management, cron scheduling, dual MCP server (StdIO & SSE), unified SQLite logging, and an embedded Dark OLED Web UI with Windows System Tray integration.

**Architecture:** A modular Go monolith with CGO disabled. Goja executes isolated ECMAScript functions with built-in helpers (`fetch`, `storage`, `crypto`, `call`); `robfig/cron/v3` triggers scheduled jobs; `go-git` syncs packages; `modernc.org/sqlite` persists logs and state; `systray` manages the Windows background lifecycle and launches the embedded Web UI on demand in an Edge app window.

**Tech Stack:** Go 1.23+, `github.com/dop251/goja`, `modernc.org/sqlite`, `github.com/robfig/cron/v3`, `github.com/go-git/go-git/v5`, `github.com/fsnotify/fsnotify`, `github.com/getlantern/systray`, HTML5/CSS3/Vanilla JS (Dark OLED UI tokens).

**Spec:** [docs/superpowers/specs/2026-09-29-actacron-design.md](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-09-29-actacron-design.md)

## Global Constraints

- **Language & CGO:** Go 1.23+, pure Go only (`CGO_ENABLED=0`). No external runtime dependencies (no Node.js, no Python, no git.exe requirement).
- **Binary Footprint:** Standalone binary under 20MB. Background RAM footprint when idle in tray: 15-25MB.
- **StdIO Cleanliness in MCP Mode:** Zero non-JSON-RPC text emitted to `os.Stdout` when running `actacron mcp`.
- **UI Design System:** Dark OLED theme (`#0F172A` canvas, `#1B2336` cards, `#22C55E` run accent, `#EF4444` error, `IBM Plex Sans` + `JetBrains Mono`). No external CDN dependencies; all CSS/JS assets embedded via `embed.FS`.
- **Platform:** Windows primary (System Tray + Edge App mode) with graceful headless fallback for Linux/macOS.

## Review Focus

1. **Goja Thread Safety:** Goja runtimes must never be shared across concurrent goroutines (every execution uses an isolated runtime or fresh instance).
2. **Infinite Loop & Recursion Protection:** Execution must terminate strictly on timeout (default 30s) or when cross-calling depth exceeds 10.
3. **MCP StdIO Protocol Hygiene:** Ensure no log messages, banners, or debug prints reach `stdout` during `actacron mcp`.
4. **Git Merge Conflicts:** Safe error handling and conflict reporting without crashing or corrupting working files.
5. **Log Retention & Disk Exhaustion:** Automatic deletion of SQLite execution logs older than retention days (default 7 days).

---

### Task 1: Go Module Setup & Core Domain Types

**Files:**
- Create: `go.mod`
- Create: `internal/domain/models.go`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: Standard Go library
- Produces: `domain.FunctionMeta`, `domain.ExecutionLog`, `domain.PackageInfo`, `config.Config`

- [ ] **Step 1: Write failing test for config loader**

```go
package config_test

import (
	"os"
	"testing"

	"actacron/internal/config"
)

func testDefaultConfig(t *testing.T) {
	cfg := config.Load()
	if cfg.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Fatalf("expected timeout 30, got %d", cfg.TimeoutSeconds)
	}
	if cfg.LogRetentionDays != 7 {
		t.Fatalf("expected log retention 7, got %d", cfg.LogRetentionDays)
	}
}

func TestConfig(t *testing.T) {
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")

	cfg := config.Load()
	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL (package or module does not exist)

- [ ] **Step 3: Initialize go.mod and implement config and domain models**

Create `go.mod`:
```go
module actacron

go 1.23.0
```

Create `internal/domain/models.go`:
```go
package domain

import "time"

type ParamSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type FunctionMeta struct {
	Name        string        `json:"name"`
	Package     string        `json:"package"`
	FilePath    string        `json:"file_path"`
	Description string        `json:"description"`
	CronExpr    string        `json:"cron_expr,omitempty"`
	IsMCP       bool          `json:"is_mcp"`
	AllowExec   bool          `json:"allow_exec"`
	Params      []ParamSchema `json:"params"`
	IsEnabled   bool          `json:"is_enabled"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type ExecutionLog struct {
	ID           int64     `json:"id"`
	ExecutionID  string    `json:"execution_id"`
	PackageName  string    `json:"package_name"`
	FunctionName string    `json:"function_name"`
	TriggerType  string    `json:"trigger_type"` // cron, mcp, manual, call
	Status       string    `json:"status"`       // success, failed, timeout
	InputParams  string    `json:"input_params"`
	OutputData   string    `json:"output_data"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ConsoleLogs  string    `json:"console_logs"`
	DurationMs   int64     `json:"duration_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type PackageInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsGit     bool      `json:"is_git"`
	RemoteURL string    `json:"remote_url,omitempty"`
	Branch    string    `json:"branch,omitempty"`
	Status    string    `json:"status"` // clean, modified, behind, conflict
	Functions []string  `json:"functions"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

Create `internal/config/config.go`:
```go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port             int
	Host             string
	DataDir          string
	PackagesDir      string
	TimeoutSeconds   int
	LogRetentionDays int
	AllowShell       bool
}

func Load() *Config {
	port := 8080
	if p, err := strconv.Atoi(os.Getenv("PORT")); err == nil && p > 0 {
		port = p
	}

	timeout := 30
	if t, err := strconv.Atoi(os.Getenv("TIMEOUT_SECONDS")); err == nil && t > 0 {
		timeout = t
	}

	retention := 7
	if r, err := strconv.Atoi(os.Getenv("LOG_RETENTION_DAYS")); err == nil && r > 0 {
		retention = r
	}

	return &Config{
		Port:             port,
		Host:             "127.0.0.1",
		DataDir:          "data",
		PackagesDir:      "packages",
		TimeoutSeconds:   timeout,
		LogRetentionDays: retention,
		AllowShell:       os.Getenv("ALLOW_SHELL") == "true",
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add go.mod internal/domain/models.go internal/config/config.go internal/config/config_test.go
git commit -m "feat(core): initialize go module, domain models, and config loader"
```

---

### Task 2: SQLite Storage Layer

**Files:**
- Create: `internal/storage/sqlite.go`
- Test: `internal/storage/sqlite_test.go`

**Interfaces:**
- Consumes: `modernc.org/sqlite`, `domain.ExecutionLog`
- Produces: `storage.DB` (InsertLog, QueryLogs, SetState, GetState, SetKV, GetKV, SetSetting, GetSetting, GetAllSettings, DeleteOldLogs)

- [ ] **Step 1: Write failing test for SQLite operations**

```go
package storage_test

import (
	"os"
	"testing"
	"time"

	"actacron/internal/domain"
	"actacron/internal/storage"
)

func TestSQLite(t *testing.T) {
	tmpFile := "test_actacron.db"
	defer os.Remove(tmpFile)

	db, err := storage.New(tmpFile)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Test Log Insert & Query
	log := domain.ExecutionLog{
		ExecutionID:  "exec-1",
		PackageName:  "dev-utils",
		FunctionName: "format_json",
		TriggerType:  "cron",
		Status:       "success",
		InputParams:  `{"data":1}`,
		OutputData:   `{"result":true}`,
		DurationMs:   42,
		CreatedAt:    time.Now(),
	}

	if err := db.InsertLog(&log); err != nil {
		t.Fatalf("insert log failed: %v", err)
	}

	logs, total, err := db.QueryLogs("format_json", "", 10, 0)
	if err != nil {
		t.Fatalf("query logs failed: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("expected 1 log, got total=%d len=%d", total, len(logs))
	}

	// Test KV Store
	if err := db.SetKV("dev-utils", "cursor", "100"); err != nil {
		t.Fatalf("set kv failed: %v", err)
	}
	val, err := db.GetKV("dev-utils", "cursor")
	if err != nil || val != "100" {
		t.Fatalf("expected '100', got '%s', err: %v", val, err)
	}

	// Test Settings Store
	if err := db.SetSetting("theme_accent", "#22C55E"); err != nil {
		t.Fatalf("set setting failed: %v", err)
	}
	accent, err := db.GetSetting("theme_accent")
	if err != nil || accent != "#22C55E" {
		t.Fatalf("expected '#22C55E', got '%s', err: %v", accent, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -v`
Expected: FAIL

- [ ] **Step 3: Implement SQLite storage with pure Go driver**

Run `go get modernc.org/sqlite`
Implement `internal/storage/sqlite.go`:
- Schema initialization (DDL for `execution_logs`, `function_state`, `key_value_store`)
- `InsertLog(log *domain.ExecutionLog) error`
- `QueryLogs(funcName, status string, limit, offset int) ([]domain.ExecutionLog, int, error)`
- `SetKV(pkg, key, value string) error`
- `GetKV(pkg, key string) (string, error)`
- `DeleteOldLogs(days int) (int64, error)`
- `SetFunctionEnabled(pkg, funcName string, enabled bool) error`
- `IsFunctionEnabled(pkg, funcName string) bool`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/storage/sqlite.go internal/storage/sqlite_test.go go.mod go.sum
git commit -m "feat(storage): implement SQLite persistence layer with execution logs and kv store"
```

---

### Task 3: Script Metadata Parser & AST Validation

**Files:**
- Create: `internal/parser/parser.go`
- Test: `internal/parser/parser_test.go`

**Interfaces:**
- Consumes: Script content string, `github.com/dop251/goja/parser`
- Produces: `parser.Parse(code string, filename string) (*domain.FunctionMeta, error)`

- [ ] **Step 1: Write failing test for JSDoc and syntax parser**

```go
package parser_test

import (
	"testing"

	"actacron/internal/parser"
)

func TestParseScript(t *testing.T) {
	code := `/**
 * @name calculate_tax
 * @description Computes VAT tax for a given amount
 * @cron 0 12 * * *
 * @mcp true
 * @param {number} amount - Total amount in USD
 * @allowExec false
 */
function main(params) {
    return params.amount * 0.1;
}`

	meta, err := parser.Parse(code, "calculate_tax.js")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if meta.Name != "calculate_tax" {
		t.Fatalf("expected calculate_tax, got %s", meta.Name)
	}
	if meta.CronExpr != "0 12 * * *" {
		t.Fatalf("expected cron expr '0 12 * * *', got %s", meta.CronExpr)
	}
	if !meta.IsMCP {
		t.Fatalf("expected IsMCP to be true")
	}
	if len(meta.Params) != 1 || meta.Params[0].Name != "amount" {
		t.Fatalf("expected param amount, got %v", meta.Params)
	}
}

func TestSyntaxError(t *testing.T) {
	badCode := `function main( { return broken; `
	_, err := parser.Parse(badCode, "bad.js")
	if err == nil {
		t.Fatalf("expected syntax error, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/parser/... -v`
Expected: FAIL

- [ ] **Step 3: Implement parser using regex for JSDoc + Goja AST parser for syntax**

Create `internal/parser/parser.go`:
- Use `goja/parser.ParseFile` to validate syntax.
- Extract comment blocks (`/** ... */`).
- Parse `@name`, `@description`, `@cron`, `@mcp`, `@allowExec`, `@param {type} name - desc`.
- Validate standard cron expression syntax (5 fields).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/parser/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/parser/parser.go internal/parser/parser_test.go go.mod go.sum
git commit -m "feat(parser): implement JSDoc metadata extractor and JS syntax validator"
```

---

### Task 4: Dynamic JS Execution Pipeline & Built-ins

**Files:**
- Create: `internal/engine/runtime.go`
- Create: `internal/engine/helpers.go`
- Test: `internal/engine/runtime_test.go`

**Interfaces:**
- Consumes: `github.com/dop251/goja`, `storage.DB`
- Produces: `engine.Runner` with `Execute(ctx, scriptPath, code, params, callerPkg) (*ExecutionResult, error)`

- [ ] **Step 1: Write failing test for JS runtime and helpers**

```go
package engine_test

import (
	"context"
	"os"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/storage"
)

func TestEngineExecution(t *testing.T) {
	dbFile := "test_engine.db"
	defer os.Remove(dbFile)
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 30, false)

	code := `
	function main(params) {
		console.log("Starting test:", params.name);
		storage.set("test_key", "stored_val");
		const val = storage.get("test_key");
		const hash = crypto.md5("hello");
		return { echo: params.name, val: val, md5: hash };
	}
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := runner.Execute(ctx, "dev", "test.js", code, map[string]interface{}{"name": "acta"})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res.Status != "success" {
		t.Fatalf("expected success, got %s, error: %s", res.Status, res.Error)
	}
	if len(res.ConsoleLogs) == 0 {
		t.Fatalf("expected console logs to be captured")
	}
}

func TestInfiniteLoopTimeout(t *testing.T) {
	dbFile := "test_loop.db"
	defer os.Remove(dbFile)
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 1, false) // 1 second timeout

	code := `function main() { while(true) {} }`

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	res, _ := runner.Execute(ctx, "dev", "loop.js", code, nil)
	if res.Status != "timeout" && res.Status != "failed" {
		t.Fatalf("expected timeout or failed status, got %s", res.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/... -v`
Expected: FAIL

- [ ] **Step 3: Implement Engine and Built-in APIs**

Create `internal/engine/runtime.go` and `internal/engine/helpers.go`:
- Fresh `goja.New()` runtime per execution.
- Register `console.log/warn/error` writing into an in-memory buffer.
- Register `storage.get/set/delete` backed by SQLite.
- Register `crypto.md5/sha256/uuid`.
- Register `base64.encode/decode`.
- Register `http.get/post` and `fetch`.
- Register `sleep(ms)` via `time.Sleep`.
- Register `env(key)` reading from `os.Getenv`.
- Guard `exec(cmd, args)` with `allowShell` boolean.
- Context cancellation listener using `vm.Interrupt()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/engine/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/engine/runtime.go internal/engine/helpers.go internal/engine/runtime_test.go
git commit -m "feat(engine): implement isolated Goja execution pipeline with built-in APIs and timeout guards"
```

---

### Task 5: Cross-Function Calling (`call`), Require, & Package Watcher

**Files:**
- Create: `internal/manager/manager.go`
- Test: `internal/manager/manager_test.go`

**Interfaces:**
- Consumes: `engine.Runner`, `parser.Parse`, `fsnotify`
- Produces: `manager.Manager` (GetFunction, ListFunctions, CallFunction, HotReload)

- [ ] **Step 1: Write failing test for inter-function call and call depth guard**

```go
package manager_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/storage"
)

func TestCrossFunctionCall(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "test.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	// Create package A with helper
	pkgA := filepath.Join(tmpDir, "pkgA")
	os.MkdirAll(pkgA, 0755)
	os.WriteFile(filepath.Join(pkgA, "double.js"), []byte(`
		function main(params) { return params.n * 2; }
	`), 0644)

	// Create package B that calls package A
	pkgB := filepath.Join(tmpDir, "pkgB")
	os.MkdirAll(pkgB, 0755)
	os.WriteFile(filepath.Join(pkgB, "calc.js"), []byte(`
		function main(params) {
			const res = call("pkgA/double", { n: params.x });
			return res + 1;
		}
	`), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := mgr.Call(ctx, "pkgB/calc", map[string]interface{}{"x": 5})
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if result.(int64) != 11 && result.(float64) != 11 {
		t.Fatalf("expected 11, got %v", result)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/manager/... -v`
Expected: FAIL

- [ ] **Step 3: Implement Manager with call stack tracking and fsnotify**

Create `internal/manager/manager.go`:
- Scans `packages/*/*.js`.
- Injects `call(name, params)` into Goja runtime context carrying `depth int`.
- If `depth > 10`, returns error `"maximum call stack depth exceeded (cyclic call detected)"`.
- Injects `require('./file.js')` resolving relative paths in the same package directory.
- Watches directory changes with `fsnotify.Watcher` to reload modified scripts.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/manager/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/manager/manager.go internal/manager/manager_test.go
git commit -m "feat(manager): implement package discovery, cross-function call with depth guard, and hot-reload"
```

---

### Task 6: Cron Engine & Scheduler

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Test: `internal/scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `github.com/robfig/cron/v3`, `manager.Manager`, `storage.DB`
- Produces: `scheduler.Scheduler` (Start, Stop, Reschedule, NextRuns, ToggleJob, RunJobNow)

- [ ] **Step 1: Write failing test for Cron scheduling**

```go
package scheduler_test

import (
	"os"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
	"actacron/internal/storage"
)

func TestScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := tmpDir + "/test_cron.db"
	db, _ := storage.New(dbFile)
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	cronSched := scheduler.New(mgr, db)
	cronSched.Start()
	defer cronSched.Stop()

	jobs := cronSched.GetJobs()
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs initially, got %d", len(jobs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scheduler/... -v`
Expected: FAIL

- [ ] **Step 3: Implement Scheduler**

Create `internal/scheduler/scheduler.go`:
- Initialize `cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)))`.
- Loop over `mgr.ListFunctions()`. If `f.CronExpr != ""` and `f.IsEnabled`, register job.
- When job fires, generate `execution_id`, call `mgr.Call(ctx, funcKey, nil)`, and log to SQLite.
- Provide `NextRuns()` and `RunJobNow(funcKey)` methods.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/scheduler/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/scheduler/scheduler.go internal/scheduler/scheduler_test.go
git commit -m "feat(scheduler): implement robfig cron scheduler with execution logging and manual triggers"
```

---

### Task 7: MCP Server (StdIO & HTTP/SSE)

**Files:**
- Create: `internal/mcp/protocol.go`
- Create: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

**Interfaces:**
- Consumes: `manager.Manager`
- Produces: `mcp.RunStdio(mgr)`, `mcp.NewSSEHandler(mgr)`

- [ ] **Step 1: Write failing test for MCP protocol handler**

```go
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"actacron/internal/domain"
	"actacron/internal/mcp"
)

func TestMCPToolsList(t *testing.T) {
	funcs := []*domain.FunctionMeta{
		{
			Name:        "get_weather",
			Package:     "utils",
			Description: "Get weather for city",
			IsMCP:       true,
			Params: []domain.ParamSchema{
				{Name: "city", Type: "string", Description: "Target city", Required: true},
			},
		},
	}

	handler := mcp.NewHandler(funcs, nil)
	req := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	}

	resp := handler.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result mcp.ToolsListResult
	json.Unmarshal(resp.Result, &result)
	if len(result.Tools) != 1 || result.Tools[0].Name != "get_weather" {
		t.Fatalf("expected get_weather tool, got %v", result)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/... -v`
Expected: FAIL

- [ ] **Step 3: Implement MCP JSON-RPC protocol and transports**

Create `internal/mcp/protocol.go` and `internal/mcp/server.go`:
- Structs for `JSONRPCRequest`, `JSONRPCResponse`, `Tool`, `CallToolParams`.
- Handler for `initialize`, `tools/list`, `tools/call`.
- `RunStdio(mgr)`: Read line-by-line from `os.Stdin`, write JSON lines strictly to `os.Stdout`. Keep `stderr` for any logging.
- `NewSSEHandler(mgr)`: HTTP SSE endpoint `/mcp/sse` and POST endpoint `/mcp/messages`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/mcp/protocol.go internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat(mcp): implement Model Context Protocol handler with Stdio and SSE transports"
```

---

### Task 8: Multi-Git Manager

**Files:**
- Create: `internal/gitmgr/git.go`
- Test: `internal/gitmgr/git_test.go`

**Interfaces:**
- Consumes: `github.com/go-git/go-git/v5`
- Produces: `gitmgr.GitService` (Clone, Pull, CommitAndPush, GetStatus)

- [ ] **Step 1: Write failing test for Git operations**

```go
package gitmgr_test

import (
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/gitmgr"
	"github.com/go-git/go-git/v5"
)

func TestGitStatus(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	_, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	svc := gitmgr.New()
	status, err := svc.GetStatus(repoDir)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status != "clean" {
		t.Fatalf("expected clean, got %s", status)
	}

	// Add file
	os.WriteFile(filepath.Join(repoDir, "test.js"), []byte("console.log(1)"), 0644)
	status, _ = svc.GetStatus(repoDir)
	if status != "modified" {
		t.Fatalf("expected modified, got %s", status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitmgr/... -v`
Expected: FAIL

- [ ] **Step 3: Implement Git service with go-git**

Create `internal/gitmgr/git.go`:
- `Clone(url, targetPath, branch, auth) error`
- `Pull(repoPath, auth) (string, error)`
- `CommitAndPush(repoPath, message, auth) error`
- `GetStatus(repoPath) (string, error)`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitmgr/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/gitmgr/git.go internal/gitmgr/git_test.go
git commit -m "feat(git): implement multi-repo git operations via pure Go go-git"
```

---

### Task 9: App Settings & Environment Manager (Backend & Windows Registry)

**Files:**
- Create: `internal/settings/settings.go`
- Create: `internal/settings/dotenv.go`
- Create: `internal/settings/autostart_windows.go`
- Create: `internal/settings/autostart_other.go`
- Test: `internal/settings/settings_test.go`
- Test: `internal/settings/dotenv_test.go`

**Interfaces:**
- Consumes: `storage.DB`
- Produces: `settings.Service` (GetSettings, SaveSettings, SetAutoStart, GetEnv, SaveEnv)

- [ ] **Step 1: Write failing test for settings service and dotenv parser**

```go
package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/settings"
	"actacron/internal/storage"
)

func TestDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	initialContent := "API_KEY=secret_123\nPORT=8080\n# comment\nDEBUG=true\n"
	os.WriteFile(envPath, []byte(initialContent), 0644)

	envMap, err := settings.ReadEnv(envPath)
	if err != nil {
		t.Fatalf("read env failed: %v", err)
	}
	if envMap["API_KEY"] != "secret_123" || envMap["PORT"] != "8080" {
		t.Fatalf("unexpected env map: %v", envMap)
	}

	envMap["API_KEY"] = "updated_token"
	envMap["NEW_VAR"] = "val"
	if err := settings.WriteEnv(envPath, envMap); err != nil {
		t.Fatalf("write env failed: %v", err)
	}

	updated, _ := settings.ReadEnv(envPath)
	if updated["API_KEY"] != "updated_token" || updated["NEW_VAR"] != "val" {
		t.Fatalf("expected updated env, got: %v", updated)
	}
}

func TestAppSettings(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "settings.db")
	db, _ := storage.New(dbFile)
	defer db.Close()

	svc := settings.New(db, filepath.Join(tmpDir, ".env"))
	cfg, err := svc.Get()
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Port)
	}

	cfg.ThemeAccent = "#38BDF8"
	cfg.Density = "compact"
	cfg.EditorFontSize = 16
	if err := svc.Save(cfg); err != nil {
		t.Fatalf("save settings failed: %v", err)
	}

	reloaded, _ := svc.Get()
	if reloaded.ThemeAccent != "#38BDF8" || reloaded.Density != "compact" || reloaded.EditorFontSize != 16 {
		t.Fatalf("expected saved theme settings, got: %+v", reloaded)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/settings/... -v`
Expected: FAIL

- [ ] **Step 3: Implement Settings and Dotenv Manager**

Create `internal/settings/dotenv.go`:
- `ReadEnv(filePath string) (map[string]string, error)`: Parses key=value pairs, ignores comments.
- `WriteEnv(filePath string, data map[string]string) error`: Serializes key=value pairs cleanly.

Create `internal/settings/autostart_windows.go` (and `autostart_other.go` with build tags `//go:build windows` vs `//go:build !windows`):
- `SetAutoStart(appName, exePath string, enable bool) error`:
  Uses Windows registry `golang.org/x/sys/windows/registry` to set or delete `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\ActaCron`.

Create `internal/settings/settings.go`:
- Struct `AppSettings`:
  ```go
  type AppSettings struct {
      Port              int    `json:"port"`
      TimeoutSeconds    int    `json:"timeout_seconds"`
      LogRetentionDays  int    `json:"log_retention_days"`
      StartWithWindows  bool   `json:"start_with_windows"`
      AllowShellExec    bool   `json:"allow_shell_exec"`
      WindowMode        string `json:"window_mode"` // "edge_app", "browser", "none"
      GitAuthorName     string `json:"git_author_name"`
      GitAuthorEmail    string `json:"git_author_email"`
      GitDefaultToken   string `json:"git_default_token"`
      NotifyOnFailure   bool   `json:"notify_on_failure"`
      ThemeAccent       string `json:"theme_accent"`   // default: #22C55E
      Density           string `json:"density"`        // compact, normal, spacious
      EditorFontSize    int    `json:"editor_font_size"` // 12, 14, 16
  }
  ```
- Reads defaults, overlays SQLite `app_settings` values.
- `Save(settings AppSettings) error`: Writes each setting to SQLite, invokes `SetAutoStart`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/settings/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/settings/settings.go internal/settings/dotenv.go internal/settings/autostart_windows.go internal/settings/autostart_other.go internal/settings/settings_test.go internal/settings/dotenv_test.go
git commit -m "feat(settings): implement app configuration, dotenv manager, and windows startup registry helper"
```

---

### Task 10: Multi-Git Manager

**Files:**
- Create: `internal/gitmgr/git.go`
- Test: `internal/gitmgr/git_test.go`

**Interfaces:**
- Consumes: `github.com/go-git/go-git/v5`
- Produces: `gitmgr.GitService` (Clone, Pull, CommitAndPush, GetStatus)

- [ ] **Step 1: Write failing test for Git operations**

```go
package gitmgr_test

import (
	"os"
	"path/filepath"
	"testing"

	"actacron/internal/gitmgr"
	"github.com/go-git/go-git/v5"
)

func TestGitStatus(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	_, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	svc := gitmgr.New()
	status, err := svc.GetStatus(repoDir)
	if err != nil {
		t.Fatalf("get status failed: %v", err)
	}
	if status != "clean" {
		t.Fatalf("expected clean, got %s", status)
	}

	// Add file
	os.WriteFile(filepath.Join(repoDir, "test.js"), []byte("console.log(1)"), 0644)
	status, _ = svc.GetStatus(repoDir)
	if status != "modified" {
		t.Fatalf("expected modified, got %s", status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitmgr/... -v`
Expected: FAIL

- [ ] **Step 3: Implement Git service with go-git**

Create `internal/gitmgr/git.go`:
- `Clone(url, targetPath, branch, auth) error`
- `Pull(repoPath, auth) (string, error)`
- `CommitAndPush(repoPath, message, auth) error`
- `GetStatus(repoPath) (string, error)`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitmgr/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/gitmgr/git.go internal/gitmgr/git_test.go
git commit -m "feat(git): implement multi-repo git operations via pure Go go-git"
```

---

### Task 11: REST API & Routing

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/handlers.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `manager.Manager`, `scheduler.Scheduler`, `storage.DB`, `gitmgr.GitService`, `settings.Service`
- Produces: `http.Handler` for API endpoints and embedded static UI

- [ ] **Step 1: Write failing test for API endpoints**

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"actacron/internal/api"
)

func TestHealthAndPackagesAPI(t *testing.T) {
	router := api.NewRouter(nil, nil, nil, nil, nil, nil)
	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %v, err: %v", resp, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/... -v`
Expected: FAIL

- [ ] **Step 3: Implement HTTP Router and Handlers**

Create `internal/api/router.go` and `internal/api/handlers.go`:
- `/api/health`: Uptime, RAM usage, active jobs count.
- `/api/packages`: List packages, clone repo, get git status, pull, commit.
- `/api/functions`: List functions, get script content, save script.
- `/api/run`: Test run a function with JSON input, returns output & logs.
- `/api/cron`: List cron schedules, toggle enable, force-run.
- `/api/logs`: Query logs with filters (funcName, status, limit, offset).
- `/api/settings`: GET and POST application settings.
- `/api/env`: GET and POST `.env` key-values.
- `/api/mcp/tools`: List MCP tools and Claude/Cursor config snippets.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add internal/api/router.go internal/api/handlers.go internal/api/api_test.go
git commit -m "feat(api): implement REST API endpoints for packages, functions, cron, settings, and logs"
```

---

### Task 12: Embedded Web Dashboard, UI Config & i18n (UI-UX-Pro-Max Dark OLED)

**Files:**
- Create: `web/index.html`
- Create: `web/css/style.css`
- Create: `web/js/app.js`
- Create: `web/js/i18n.js`
- Create: `web/embed.go`
- Test: `web/embed_test.go`

**Interfaces:**
- Consumes: REST API (`/api/...`), `design-system/actacron/MASTER.md`
- Produces: Embedded SPA assets served via `//go:embed`

- [ ] **Step 1: Write failing test for embedded assets**

```go
package web_test

import (
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
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./web/... -v`
Expected: FAIL

- [ ] **Step 3: Build the Dark OLED UI Dashboard with Theme Customization & i18n**

1. `web/embed.go`:
```go
package web

import "embed"

//go:embed *
var Assets embed.FS
```

2. `web/js/i18n.js`:
Lightweight translation dictionary:
- Default: `en` (English)
- Secondary: `vi` (Vietnamese)
- Covers: Navigation tabs, button labels, modal headers, settings explanations, and a natural language Cron translator (`cronToString(expr, lang)`).

3. `web/css/style.css`:
Implement Master tokens from `design-system/actacron/MASTER.md`:
- CSS Variables: `--color-background: #0F172A`, `--color-card: #1B2336`, `--color-border: #334155`, `--color-accent: #22C55E`, `--color-destructive: #EF4444`, `--color-muted: #94A3B8`.
- Dynamic Customization tokens:
  - Theme Accent override: `[data-theme="emerald"] { --color-accent: #22C55E; }`, `[data-theme="blue"] { --color-accent: #38BDF8; }`, `[data-theme="purple"] { --color-accent: #A855F7; }`.
  - Density override: `[data-density="compact"] { --space-unit: 4px; }`, `[data-density="spacious"] { --space-unit: 8px; }`.
  - Font Size override: `--editor-font-size: 14px`.
- 3-Pane layout: Left tree (260px), Center code editor (flex), Right inspector (360px).
- Log table with quick chips filter and slide-in drawer.
- Settings Screen layout: 4 tabs (General, Git, Environment Variables, UI & Appearance).

4. `web/index.html`:
- Semantic layout: Sidebar Navigation (Overview, Workspace, Logs, MCP Hub, Settings).
- Top header metrics: System status badge, RAM usage, active cron countdown, Language Switcher `[🌐 EN | VI]`, Sync All button.
- Workspace: Tree view, Code editor with live metadata chips, Inspector with Test Runner & Schedule toggle.
- Settings Screen:
  - Tab 1: General (Port, Timeout, Log Retention, Run on Startup switch, Allow Shell switch).
  - Tab 2: Git Credentials (Author Name, Author Email, Token).
  - Tab 3: Environment (.env Key-Value Editor with mask toggle).
  - Tab 4: UI & Appearance (Language selector, Accent color picker, density scale selector, editor font size).

5. `web/js/app.js`:
- Client-side router for tabs.
- Dynamic theme and language injector: applies `data-theme`, `data-density`, and active language immediately and syncs with `/api/settings`.
- Code editor with syntax coloring, line numbers, and keyboard shortcuts (`Ctrl+S`, `Ctrl+Enter`).
- Real-time test runner and live logs viewer.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./web/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```powershell
git add web/index.html web/css/style.css web/js/app.js web/js/i18n.js web/embed.go web/embed_test.go
git commit -m "feat(ui): implement embedded Dark OLED dashboard with App Settings, UI theme config, and i18n (en/vi)"
```

---

### Task 13: Windows System Tray & Lifecycle

**Files:**
- Create: `internal/tray/tray.go`
- Create: `internal/tray/window.go`
- Create: `main.go`

**Interfaces:**
- Consumes: `github.com/getlantern/systray`, `api.Router`, `scheduler.Scheduler`, `settings.Service`
- Produces: CLI commands: `actacron daemon` (default) and `actacron mcp`

- [ ] **Step 1: Write window launcher and tray handler**

Create `internal/tray/window.go`:
- Finds Microsoft Edge (`msedge.exe`) or defaults to standard browser open.
- Launches `msedge.exe --app=http://127.0.0.1:8080` (or browser tab based on `window_mode` setting).

Create `internal/tray/tray.go`:
- Initializes systray icon and tooltip.
- Menu items: Open Dashboard, Sync All Git, Pause/Resume Cron, View Logs, Exit.
- Left-click on tray icon triggers window launch.
- Provides balloon / toast notification method `Notify(title, message string)`.

- [ ] **Step 2: Implement main.go CLI entrypoint**

Create `main.go`:
- Parse flags/subcommands:
  - If `os.Args[1] == "mcp"`, start `mcp.RunStdio(mgr)` without tray and without stdout noise.
  - Default: Start daemon, SQLite, cron, HTTP server, and run `systray.Run`.

- [ ] **Step 3: Verify build compiles clean with zero CGO**

Run: `go build -o actacron.exe .`
Expected: Builds `actacron.exe` cleanly.

- [ ] **Step 4: Commit**

```powershell
git add internal/tray/tray.go internal/tray/window.go main.go
git commit -m "feat(tray): implement Windows System Tray daemon, Edge app window launcher, and notification helper"
```

---

### Task 14: End-to-End Integration Verification

**Files:**
- Create: `tests/e2e/e2e_test.go`

**Interfaces:**
- Consumes: Complete ActaCron stack (Engine, Manager, Storage, Settings, Scheduler, API)
- Produces: End-to-end integration pass report

- [ ] **Step 1: Write end-to-end integration test**

```go
package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"actacron/internal/engine"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
	"actacron/internal/settings"
	"actacron/internal/storage"
)

func TestEndToEndPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "actacron.db")
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
	defer db.Close()

	runner := engine.New(db, 10, false)
	mgr := manager.New(tmpDir, runner, db)

	// Test settings
	envPath := filepath.Join(tmpDir, ".env")
	os.WriteFile(envPath, []byte("GREET_PREFIX=Hello"), 0644)
	settingsSvc := settings.New(db, envPath)

	cfg, _ := settingsSvc.Get()
	cfg.ThemeAccent = "#22C55E"
	settingsSvc.Save(cfg)

	// Create a test function with cron & mcp metadata
	pkgDir := filepath.Join(tmpDir, "testpkg")
	os.MkdirAll(pkgDir, 0755)
	scriptContent := `/**
 * @name greet_task
 * @cron * * * * *
 * @mcp true
 * @param {string} name - Target name
 */
function main(params) {
	const prefix = env("GREET_PREFIX") || "Hi";
	const res = prefix + " " + params.name;
	console.log("Greeting generated:", res);
	storage.set("last_greet", res);
	return res;
}`
	os.WriteFile(filepath.Join(pkgDir, "greet.js"), []byte(scriptContent), 0644)

	if err := mgr.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	fn := mgr.GetFunction("testpkg/greet_task")
	if fn == nil || !fn.IsMCP || fn.CronExpr != "* * * * *" {
		t.Fatalf("function metadata not parsed correctly: %+v", fn)
	}

	// Execute manual run
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := mgr.Call(ctx, "testpkg/greet_task", map[string]interface{}{"name": "Antigravity"})
	if err != nil || res != "Hello Antigravity" {
		t.Fatalf("unexpected call result: %v, err: %v", res, err)
	}

	// Verify log in SQLite
	logs, total, err := db.QueryLogs("greet_task", "", 10, 0)
	if err != nil || total < 1 {
		t.Fatalf("expected logged execution in SQLite, got %d, err: %v", total, err)
	}

	// Verify KV state
	stored, err := db.GetKV("testpkg", "last_greet")
	if err != nil || stored != "Hello Antigravity" {
		t.Fatalf("expected KV 'Hello Antigravity', got '%s'", stored)
	}

	// Verify settings persisted
	reloadedCfg, _ := settingsSvc.Get()
	if reloadedCfg.ThemeAccent != "#22C55E" {
		t.Fatalf("expected accent #22C55E, got %s", reloadedCfg.ThemeAccent)
	}
}
```

- [ ] **Step 2: Run end-to-end test**

Run: `go test ./tests/e2e/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```powershell
git add tests/e2e/e2e_test.go
git commit -m "test(e2e): add end-to-end integration test validating engine, settings, storage, and metadata"
```

---

### Task 15: Release Build & Distribution Packaging

**Files:**
- Create: `scripts/build.ps1`
- Create: `.env.example`
- Create: `packages/demo-pack/check_health.js`
- Create: `packages/demo-pack/math_tool.js`

**Interfaces:**
- Consumes: Production Go compiler
- Produces: `dist/ActaCron-v1.0.0-windows-amd64.zip` containing standalone portable application

- [ ] **Step 1: Create build and release script**

Create `scripts/build.ps1`:
```powershell
$ErrorActionPreference = "Stop"
Write-Host "Building ActaCron Production Release..." -ForegroundColor Cyan

# Clean dist
Remove-Item -Recurse -Force -ErrorAction SilentlyContinue dist
New-Item -ItemType Directory -Path "dist/ActaCron-v1.0.0-windows-amd64" | Out-Null
New-Item -ItemType Directory -Path "dist/ActaCron-v1.0.0-windows-amd64/packages/demo-pack" | Out-Null

# Compile with GUI flag (no black console window) and stripped symbols (< 16MB)
go build -ldflags "-H windowsgui -s -w" -o "dist/ActaCron-v1.0.0-windows-amd64/actacron.exe" .

# Copy documentation and sample files
Copy-Item ".env.example" "dist/ActaCron-v1.0.0-windows-amd64/.env.example"
Copy-Item "README.md" "dist/ActaCron-v1.0.0-windows-amd64/README.md" -ErrorAction SilentlyContinue
Copy-Item -Recurse "packages/demo-pack/*" "dist/ActaCron-v1.0.0-windows-amd64/packages/demo-pack/"

# Compress into portable zip
Compress-Archive -Path "dist/ActaCron-v1.0.0-windows-amd64" -DestinationPath "dist/ActaCron-v1.0.0-windows-amd64.zip"

Write-Host "Release created successfully: dist/ActaCron-v1.0.0-windows-amd64.zip" -ForegroundColor Green
```

- [ ] **Step 2: Create sample .env.example and starter demo scripts**

Create `.env.example`:
```ini
# ActaCron Global Configuration
PORT=8080
TIMEOUT_SECONDS=30
LOG_RETENTION_DAYS=7
ALLOW_SHELL=false

# Secrets & Custom API Keys
# TELEGRAM_BOT_TOKEN=
# DISCORD_WEBHOOK_URL=
```

Create `packages/demo-pack/check_health.js`:
```javascript
/**
 * @name check_health
 * @description Periodic system uptime and health check
 * @cron */30 * * * *
 * @mcp false
 */
function main() {
    console.log("Health check executed at:", new Date().toISOString());
    return { status: "healthy", timestamp: Date.now() };
}
```

Create `packages/demo-pack/math_tool.js`:
```javascript
/**
 * @name add_numbers
 * @description Adds two numbers together
 * @mcp true
 * @param {number} a - First number
 * @param {number} b - Second number
 */
function main(params) {
    const a = (params && params.a) || 0;
    const b = (params && params.b) || 0;
    return { result: a + b };
}
```

- [ ] **Step 3: Execute build script and verify release**

Run: `powershell -ExecutionPolicy Bypass -File scripts/build.ps1`
Expected: Generates `dist/ActaCron-v1.0.0-windows-amd64.zip` with `actacron.exe` under 20MB.

- [ ] **Step 4: Commit**

```powershell
git add scripts/build.ps1 .env.example packages/demo-pack/check_health.js packages/demo-pack/math_tool.js
git commit -m "chore(release): add production build script, starter package, and packaging automation"
```


