# ActaCron

> **Dynamic JavaScript Scripting Engine, Multi-Git Manager, Cron Scheduler & AI MCP Hub in a Standalone Windows Daemon.**

ActaCron is an ultra-lightweight developer platform and daemon built in pure Go (zero CGO). It executes dynamic ECMAScript functions in isolated sandboxes, manages scripts across multiple Git repositories, provides an embedded cron scheduler, surfaces functions as Model Context Protocol (MCP) tools for AI agents (Claude Desktop, Cursor IDE), and features an embedded Dark OLED Web Dashboard with Windows System Tray integration.

---

## Key Features

- **Dynamic Scripting (Goja Engine):** Execute JavaScript with built-in sandbox helpers (`storage`, `crypto`, `base64`, `http`/`fetch`, `env`, `sleep`, `exec`, and cross-function `call`).
- **MCP Server (StdIO & SSE):** Seamlessly expose dynamic functions to AI agents like Claude Desktop and Cursor with automatic JSON Schema generation from JSDoc comments.
- **Git Multi-Repo Sync:** Pure Go Git manager (`go-git/v5`) supporting multiple repositories with automated pull, push, and conflict management.
- **Embedded Web UI (Dark OLED):** Zero-CDN responsive 3-pane interface (Package tree, Code editor with hotkeys `Ctrl+S` / `Ctrl+Enter`, Inspector with live test runner).
- **Internationalization (i18n):** Native English (default) and Vietnamese (secondary) toggle with humanized Cron schedule translation.
- **System Tray & Edge App Window:** Runs quietly in the Windows notification area with a low RAM footprint (~15-25MB) and launches frameless native-like window mode on demand.
- **SQLite Audit Logs:** Automatic execution history and console log retention with customizable cleanup.

---

## Getting Started

### 1. Run in Development Mode

```powershell
# Run the daemon with Web UI and tray integration
go run .

# Run headless (ideal for Linux / CI / server environments)
go run . headless

# Run in MCP StdIO Mode (used by Claude Desktop / Cursor)
go run . mcp
```

### 2. Connect Claude Desktop to ActaCron MCP

Add the following to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "actacron": {
      "command": "C:\\path\\to\\actacron.exe",
      "args": ["mcp"]
    }
  }
}
```

### 3. Writing Functions with JSDoc

Create any `.js` file inside `packages/<pack-name>/`:

```javascript
/**
 * @name calculate_discount
 * @description Computes discounted product price
 * @cron 0 9 * * 1-5
 * @mcp true
 * @param {number} price - Original product price
 * @param {number} discount - Percentage discount (0-100)
 */
function main(params) {
    const price = params.price || 0;
    const discount = params.discount || 0;
    const finalPrice = price * (1 - discount / 100);

    console.log(`Calculated discount: ${price} -> ${finalPrice}`);
    storage.set("last_calculation", { price, finalPrice, timestamp: Date.now() });

    return { final_price: finalPrice };
}
```

---

## Production Build & Distribution

To create a production distribution bundle without terminal popups:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```

This compiles a stripped, GUI-subsystem binary (`actacron.exe`) under 20MB and packages it into `dist/ActaCron-v1.0.0-windows-amd64.zip`.

---

## Architecture

```
ActaCron Monolith (Pure Go, CGO_ENABLED=0)
├── internal/
│   ├── api/        # REST API & Static Asset Server
│   ├── config/     # Baseline Configuration
│   ├── domain/     # Models (FunctionMeta, ExecutionLog, AppSettings)
│   ├── engine/     # Goja ECMAScript Runtime & Sandboxed Helpers
│   ├── gitmgr/     # Multi-Repository Pure Go Git Operations
│   ├── manager/    # Script Discovery, File Watcher & Cross-Calling
│   ├── mcp/        # Model Context Protocol (StdIO & SSE Server)
│   ├── parser/     # JSDoc AST Metadata Parser
│   ├── scheduler/  # Cron Engine (robfig/cron/v3)
│   ├── settings/   # .env Manager & Windows Registry Autostart
│   ├── storage/    # ModernC SQLite (Logs, Key-Value, Settings)
│   └── tray/       # Windows Systray Loop & Edge App Launcher
├── packages/       # Script Packages
└── web/            # Embedded Dark OLED Web Dashboard (HTML5/CSS3/Vanilla JS)
```

## License

MIT
