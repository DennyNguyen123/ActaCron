# ActaCron: Architecture & UI/UX Design Specification

**Date:** 2026-09-29  
**Target Project:** ActaCron  
**Platform:** Windows (with cross-platform daemon support for Linux/macOS)  
**Binary Footprint:** Standalone Go binary (< 20MB), background RAM ~15-25MB  

---

## 1. Executive Summary & Goals

**ActaCron** is an ultra-lightweight, single-binary automation runtime and tool gateway written in Go. It empowers developers and AI agents to:
1. Dynamically write and manage automation functions in pure JavaScript (ECMAScript 5.1/ES6 via Goja).
2. Group and manage scripts into **one or multiple Git repositories (Packages)** with built-in auto-pull, push, and conflict detection.
3. Automatically expose functions as **Model Context Protocol (MCP)** tools (via both StdIO for AI IDEs like Claude Desktop/Cursor and HTTP/SSE for remote AI agents).
4. Run functions on flexible **Cron schedules** (using standard cron expressions).
5. Inter-call functions across packages (`call('package/function', params)`).
6. Provide an **embedded, high-performance Web UI** operating either on-demand as a native-feeling Windows app window (or browser tab) with a Windows **System Tray** daemon, ensuring zero background RAM waste.
7. Record all executions (Cron, MCP, Manual test, cross-calls) into a unified, queryable **SQLite** log with search, status filtering, and auto-cleanup.

---

## 2. System Architecture & Components

```
+-----------------------------------------------------------------------------------+
|                                 ActaCron Runtime                                  |
|                                                                                   |
|  +--------------------+   +-----------------------+   +------------------------+  |
|  |   System Tray      |   |   Embedded Web UI     |   |       MCP Server       |  |
|  |   (Go Systray)     |   |   (HTML/CSS/JS Embed) |   | (StdIO & HTTP/SSE)     |  |
|  +---------+----------+   +-----------+-----------+   +-----------+------------+  |
|            |                          |                           |               |
|            +------------------+       |       +-------------------+               |
|                               |       |       |                                   |
|                               v       v       v                                   |
|                       +-------------------------------+                           |
|                       |   Core Controller & Router    |                           |
|                       +---------------+---------------+                           |
|                                       |                                           |
|       +-------------------------------+-------------------------------+           |
|       |                               |                               |           |
|       v                               v                               v           |
|  +---------+                  +---------------+               +---------------+   |
|  | Cron    |                  |  Multi-Git    |               |  SQLite DB    |   |
|  | Engine  |                  |  Manager      |               |  (Logs, State,|   |
|  | (robfig)|                  |  (go-git)     |               |   Job Status) |   |
|  +----+----+                  +-------+-------+               +-------+-------+   |
|       |                               |                               |           |
|       |   +---------------------------+-------------------------------+           |
|       v   v                                                                       |
|  +--------------------------------------------------------------------+           |
|  |                   Unified JS Execution Pipeline                    |           |
|  |               (Goja Isolates, Timeout, Call Depth)                 |           |
|  |                                                                    |           |
|  |   Built-in Helpers: fetch, storage, crypto, base64, sleep, call    |           |
|  +--------------------------------------------------------------------+           |
|                                       |                                           |
|                                       v                                           |
|                       +-------------------------------+                           |
|                       | File System: packages/ & .env |                           |
|                       +-------------------------------+                           |
+-----------------------------------------------------------------------------------+
```

### 2.1 Technology Stack
* **Language:** Go 1.23+ (Pure Go, CGO disabled for effortless cross-compilation and 0 external DLL dependencies).
* **JavaScript Engine:** `github.com/dop251/goja` (Pure Go ECMAScript runtime).
* **Database:** `modernc.org/sqlite` (Pure Go SQLite driver, zero CGO, single file `data/actacron.db`).
* **Cron Engine:** `github.com/robfig/cron/v3`.
* **Git Engine:** `github.com/go-git/go-git/v5` (Pure Go Git implementation, handles clone/pull/push/commit without requiring `git.exe`).
* **File Watcher:** `github.com/fsnotify/fsnotify` (Hot-reload scripts on disk change).
* **System Tray:** `github.com/getlantern/systray` (Native Windows Tray icon and context menu).
* **UI Frontend:** Vanilla ES6 + Modern CSS (Custom Design System based on `ui-ux-pro-max`), bundled directly into Go binary via `//go:embed`.

---

## 3. Directory Layout & File Organization

```text
actacron/
├── actacron.exe                  # Standalone executable
├── .env                          # Global environment variables & secrets
├── data/
│   └── actacron.db               # SQLite database (Execution logs, KV state, Cron toggles)
└── packages/                     # Script packages (each can be an independent Git repo)
    ├── dev-utils/
    │   ├── .git/                 # Git repository 1
    │   ├── package.json          # Optional metadata (git remote, description)
    │   ├── helpers.js            # Shared utility module (required by other scripts)
    │   └── format_json.js        # Executable function
    ├── crypto-alerts/
    │   ├── .git/                 # Git repository 2
    │   └── btc_tracker.js        # Cron job function
    └── local/                    # Unversioned local scratchpad
        └── test_tool.js
```

---

## 4. Script Format & Metadata Definition

Scripts are standard `.js` files. Metadata for Cron schedules and MCP Tool schemas is declared via JSDoc comments at the top of the file:

```javascript
/**
 * @name check_btc_price
 * @description Fetches current Bitcoin price from CoinGecko and triggers an alert if above threshold
 * @cron */15 * * * *
 * @mcp true
 * @param {string} currency - Target fiat currency (default: "usd")
 * @param {number} alert_above - Alert threshold price
 * @allowExec false
 */
function main(params) {
    const currency = (params && params.currency) || "usd";
    const resp = http.get(`https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=${currency}`);
    if (resp.status !== 200) {
        throw new Error(`Failed to fetch price: HTTP ${resp.status}`);
    }
    const data = JSON.parse(resp.body);
    const price = data.bitcoin[currency];
    console.log(`Current BTC Price: ${price} ${currency.toUpperCase()}`);

    // Stateful KV check
    const lastPrice = storage.get("last_btc_price") || 0;
    storage.set("last_btc_price", price);

    // Call internal helper
    call("dev-utils/send_alert", {
        subject: "BTC Price Update",
        message: `BTC moved from ${lastPrice} to ${price}`
    });

    return { currency, price, previous_price: lastPrice };
}
```

### 4.1 Parser Rules
* `@name`: Function identifier (defaults to filename without `.js`).
* `@description`: Summary string exposed to LLM in MCP tools list.
* `@cron`: Standard 5-field cron expression (`minute hour dom month dow`). If omitted, function is not scheduled.
* `@mcp`: Boolean (`true`/`false`). If true, registered as an MCP tool.
* `@param {type} name - description`: Parsed into JSON Schema properties for MCP tool input.
* `@allowExec`: Boolean (`true`/`false`, default `false`). Controls whether `exec()` is permitted.

---

## 5. JavaScript Runtime Environment & Built-ins

To keep the system lightweight while remaining highly capable for automation, Goja is provisioned with the following global APIs:

1. **HTTP Client:**
   - `http.get(url, [headers])` -> `{ status: number, headers: object, body: string }`
   - `http.post(url, body, [headers])` -> `{ status: number, headers: object, body: string }`
   - `fetch(url, options)` -> Promise-like synchronous wrapper.
2. **Cross-Function Calling:**
   - `call(targetFunction, params)`: Runs another function by name (`package/func` or `func`). Includes **Call Depth Counter** (max depth 10) and context propagation to prevent infinite recursion.
3. **Internal Package Module Loading:**
   - `require('./path.js')`: Loads and executes a peer `.js` file within the same package, returning its `module.exports` or `exports`.
4. **Persistent State Storage (Key-Value):**
   - `storage.get(key)`: Retrieves a string/number/object stored in SQLite.
   - `storage.set(key, value)`: Stores a JSON-serializable value in SQLite.
   - `storage.delete(key)`: Removes a key.
5. **Environment & Secrets:**
   - `env(key)` or `process.env[key]`: Reads from the `.env` configuration.
6. **Utility Helpers:**
   - `sleep(ms)`: Non-blocking sleep on the Go goroutine.
   - `crypto.md5(str)`, `crypto.sha256(str)`, `crypto.uuid()`.
   - `base64.encode(str)`, `base64.decode(str)`.
   - `console.log(...)`, `console.warn(...)`, `console.error(...)`: Captured per execution into DB.
7. **System Command Execution (Guarded):**
   - `exec(command, [args])` -> `{ stdout: string, stderr: string, exitCode: number }`. Blocked unless `@allowExec true` and global shell setting is enabled.

---

## 6. MCP (Model Context Protocol) Implementation

ActaCron implements the standard Model Context Protocol (MCP) specification:

1. **StdIO Transport (`actacron mcp`):**
   - Communicates via JSON-RPC 2.0 over `stdin` and `stdout`.
   - **Crucial Rule:** Zero non-JSON output on `stdout`! All logs and system diagnostics are directed to `stderr` or SQLite.
   - Ideal for: Claude Desktop config (`claude_desktop_config.json`), Cursor AI, Antigravity.
2. **HTTP / SSE Transport (`actacron daemon`):**
   - Exposes `/mcp/sse` and `/mcp/messages` endpoints.
   - Allows network-accessible agents to discover and invoke tools remotely.
3. **Dynamic Schema Generation:**
   - Automatically inspects loaded functions marked `@mcp true` and presents them under `tools/list`.
   - Incoming `tools/call` executes the target script through the unified pipeline and returns the result in MCP tool response format.

---

## 7. Multi-Git Package Management & Sync

* **Package Discovery:** Scans `packages/*` on startup and via `fsnotify`.
* **Git Operations:**
  - Pure Go using `go-git`.
  - Operations supported: `Clone`, `Pull`, `Commit`, `Push`, `Status`.
  - Status Indicators per package:
    - 🟢 `Synced`: Clean working tree, matches remote.
    - 🟡 `Modified`: Local changes present, prompt for commit message.
    - 🔵 `Behind`: Remote has newer commits, 1-click Pull.
    - 🔴 `Conflict`: Merge conflicts detected, provides resolve choices.
* **Global Action:** "Sync All" button triggers parallel fetch/pull across all configured remote packages.

---

## 8. Unified Logging & SQLite Schema

Every invocation (Cron, MCP, Test Run, Cross-call) is recorded into `actacron.db`.

```sql
CREATE TABLE IF NOT EXISTS execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id TEXT NOT NULL,
    package_name TEXT NOT NULL,
    function_name TEXT NOT NULL,
    trigger_type TEXT NOT NULL, -- 'cron', 'mcp', 'manual', 'call'
    status TEXT NOT NULL,       -- 'success', 'failed', 'timeout'
    input_params TEXT,          -- JSON string
    output_data TEXT,           -- JSON string or return text
    error_message TEXT,         -- Exception stack trace
    console_logs TEXT,          -- Captured console lines with level & timestamp
    duration_ms INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_logs_func ON execution_logs(function_name, created_at);
CREATE INDEX IF NOT EXISTS idx_logs_status ON execution_logs(status);
CREATE INDEX IF NOT EXISTS idx_logs_created ON execution_logs(created_at);

CREATE TABLE IF NOT EXISTS function_state (
    package_name TEXT NOT NULL,
    function_name TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT 1,
    last_run_at DATETIME,
    last_status TEXT,
    PRIMARY KEY (package_name, function_name)
);

CREATE TABLE IF NOT EXISTS key_value_store (
    package_name TEXT NOT NULL,
    store_key TEXT NOT NULL,
    store_value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (package_name, store_key)
);

CREATE TABLE IF NOT EXISTS app_settings (
    setting_key TEXT PRIMARY KEY,
    setting_value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 8.1 Log Retention Policy
* Automated cleanup task runs daily.
* Configurable retention period (default: 7 days).
* Query: `DELETE FROM execution_logs WHERE created_at < datetime('now', '-7 days')`.

---

## 9. UI/UX Design Specification (UI-UX-Pro-Max Aligned)

### 9.1 Design Tokens (Dark Mode OLED)
* **Background Canvas:** `#0F172A` (Midnight Slate)
* **Card / Pane Surface:** `#1B2336` (Deep Navy Card)
* **Active / Border Stroke:** `#334155` (Subtle Divider)
* **Primary / Success:** `#22C55E` (Emerald Run Green)
* **Destructive / Error:** `#EF4444` (Coral Red)
* **Warning / Git Pending:** `#F59E0B` (Amber)
* **Typography:**
  - UI Labels & Headers: `IBM Plex Sans`, `-apple-system`, `Segoe UI`, sans-serif
  - Code, Logs & JSON: `JetBrains Mono`, `Consolas`, monospace

### 9.2 Navigation & Layout
The interface uses a **3-Pane IDE Layout** within a clean Windows App Window:

```
+-------------------------------------------------------------------------------------------------------+
|  ActaCron v1.0.0    [● Running (18.4MB)]  [⏰ 3 Cron Active]  [🤖 5 MCP Tools]         [Sync All 🔄]   |
+-------------------------------------------------------------------------------------------------------+
|  [Sidebar]  | [Package / File Tree]  | [Center Code Editor & Metadata Header]  | [Right Inspector]    |
|             |                        |                                         |                      |
|  📊 Overview| 📁 packages            | /**                                     | [Tab: Test Runner]   |
|  ⚡ Worksp. |   ▼ dev-utils (🟢)     |  * @name format_json                    | Input JSON:          |
|  📜 Logs    |       format_json.js   |  * @cron 0 * * * *                      | { "data": "test" }   |
|  🤖 MCP Hub |       helpers.js       |  */                                     |                      |
|  ⚙️ Settings|   ▶ crypto-alerts (🟡) | function main(params) {                 | [▶ Run Now (Ctrl+↵)] |
|             |   ▶ local              |     console.log("Formatting...");       |                      |
|             |                        |     return JSON.stringify(params);      | Output (24ms):       |
|             | [+ New Script]         | }                                       | "{\"data\":\"test\"}"|
|             | [+ Add Git Repo]       |                                         |                      |
|             |                        | Status: [⏰ Every hour] [🤖 MCP Active] | Logs:                |
|             |                        | [💾 Save (Ctrl+S)]                      | [INFO] Formatting... |
+-------------+------------------------+-----------------------------------------+----------------------+
|  Status Bar: Ready | Memory: 18.2MB | SQLite: Healthy | System Tray: Active                           |
+-------------------------------------------------------------------------------------------------------+
```

### 9.3 Key Screens & Workflows

1. **Workspace Screen (Core):**
   - Left: Tree view of packages and `.js` files with real-time Git status pills.
   - Center: Monaco/CodeJar-style code editor with syntax coloring, line numbers, and error badges. Top header displays live metadata chips (`Cron Active`, `MCP Tool: name`, `Git: Synced`).
   - Right: Collapsible Inspector containing:
     - **Quick Test Runner:** Input JSON textarea, Run button, Output display, execution time, and real-time console log stream.
     - **Schedule Manager:** Next Run countdown, Enable/Disable toggle, Force-run now.
2. **Execution Logs Screen:**
   - Fast filter chips: `[All]`, `[Failed Only]`, `[Cron]`, `[MCP]`, `[Manual Test]`.
   - Date range selector and text search.
   - Clean tabular view with execution duration, trigger source, and status badges.
   - Detail drawer on row click displaying formatted JSON input/output, color-coded console logs (`info`/`warn`/`error`), and stack trace.
3. **MCP Tools Hub Screen:**
   - Visual catalog of all exported MCP tools with parameter schemas.
   - 1-Click buttons: **"Copy Claude Desktop Config"** and **"Copy Cursor Config"**.
   - Interactive Tool Tester.
4. **Settings & Environment Screen:**
   - **Tab 1: General & Daemon:**
     - HTTP Port (`8080`), Default Execution Timeout (`30s`), Log Retention Days (`7d`).
     - "Run on Windows Startup" switch (updates `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`).
     - "Allow Shell Execution (`exec()`)" security switch.
     - Window Open Mode: Dedicated Edge App Window vs Default Browser Tab vs Silent in Tray.
   - **Tab 2: Git Credentials & Sync:**
     - Default Git Author Name & Email.
     - Personal Access Token / SSH Key for private repositories.
     - Auto-pull interval or manual sync only.
   - **Tab 3: Environment Variables (`.env` Manager):**
     - Interactive Key-Value table for global secrets and API tokens.
     - Show/Hide mask toggle for secret values.
     - Two-way sync with root `.env` file.
   - **Tab 4: UI & Appearance (UI-UX-Pro-Max Theme Configuration):**
     - Master design system: `design-system/actacron/MASTER.md`.
     - **Language / i18n:** English (Default) and Vietnamese (Secondary). 1-click toggle `🌐 EN | VI` in header & settings. Translates UI labels, status messages, and natural language Cron explanations (e.g. "Every 15 minutes" / "Chạy mỗi 15 phút").
     - Theme Accent Color: Emerald Run Green (`#22C55E`, default), Electric Blue (`#38BDF8`), Violet (`#A855F7`).
     - UI Density Scale: Compact (`8-32px`), Balanced (`16-64px`), Spacious (`24-96px`).
     - Code Editor Font Size: `12px`, `14px`, `16px`.
     - Instant live preview via CSS custom property overrides (`--color-accent`, `--space-*`, `--editor-font-size`).

### 9.4 Keyboard Shortcuts
* `Ctrl + S`: Save script, validate syntax, reload schedule.
* `Ctrl + Enter`: Execute Test Run immediately.
* `Ctrl + P`: Quick open script/file modal.
* `Esc`: Close drawers or modals.

---

## 10. Process Lifecycle & System Tray Behavior

1. **Startup:**
   - Double-clicking `actacron.exe` or launching via Windows Startup initializes the Go background daemon.
   - Go daemon initializes SQLite, loads `.env`, discovers packages, schedules cron jobs, and starts the local HTTP server on `127.0.0.1:8080`.
   - Places the ActaCron icon in the Windows System Tray.
   - Launches Microsoft Edge in app mode (`msedge.exe --app=http://127.0.0.1:8080`), rendering a borderless, clean desktop app window with no browser tabs or URL bar.
2. **Window Closing:**
   - Closing the window terminates the Edge renderer process, immediately freeing ~100MB RAM.
   - Go daemon remains running in the System Tray consuming only **~15-20MB RAM**, continuing all Cron schedules and MCP requests.
3. **System Tray Actions:**
   - Left-click or double-click: Reopens the Dashboard window.
   - Right-click menu:
     - 🌐 Open Dashboard
     - 🔄 Sync All Git Packages
     - ⏸️ Pause / Resume Cron Scheduler
     - 📜 View Latest Logs
     - ❌ Exit ActaCron (cleanly stops cron, closes DB, exits process)

---

## 11. Packaging & Release Distribution

1. **Production Binary Optimization:**
   - Command: `go build -ldflags "-H windowsgui -s -w" -o actacron.exe .`
   - `-H windowsgui`: Suppresses the Windows command console window (no black terminal box), cleanly launching directly into the System Tray with Edge App on-demand.
   - `-s -w`: Strips debug information and symbol tables, reducing binary size to **~14-16MB**.
2. **Windows Resource Icon:**
   - Embeds native Windows icon (`.ico`) into the executable for File Explorer and Taskbar polish.
3. **Release Bundle Structure (Portable ZIP):**
   ```text
   ActaCron-v1.0.0-windows-amd64/
   ├── actacron.exe             # Standalone binary with embedded UI & icons
   ├── .env.example             # Documented template for environment secrets
   ├── README.md                # Quick-start guide, shortcuts, MCP config snippet
   └── packages/                # Sample starter package
       └── demo-pack/
           ├── check_health_cron.js   # Sample periodic cron job
           └── math_mcp_tool.js       # Sample MCP tool for LLMs
   ```

---

## 12. Security & Guardrails

1. **Execution Timeout:** Hard limit of 30 seconds (configurable) per function call via `context.WithTimeout` to neutralize infinite loops.
2. **Call Stack Limit:** Max depth of 10 for cross-function `call()` to block cyclic recursion deadlocks.
3. **Isolate Runtime:** Each execution gets its own runtime instance; state does not leak between parallel cron jobs.
4. **Shell Execution Lockdown:** The `exec` command is disabled by default and requires explicit opt-in per script (`@allowExec true`) plus global administrative toggle.
5. **StdIO Hygiene:** Guaranteed pure JSON-RPC on `stdout` when in MCP mode.
