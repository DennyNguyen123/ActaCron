# Comprehensive Cron Management (Package B) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement comprehensive cron management in ActaCron including UI toggle switches, start/end datetime effective windows, timezone awareness, overlap prevention, automatic retry on failure, max execution limits, and full observability with status badges and next-run countdowns.

**Architecture:** Distributed into two parallel feature tracks: (1) Backend track implementing JSDoc annotations parsing, SQLite schema extension, and Scheduler lifecycle & overlap guard; (2) Frontend track implementing DOM controls in Script Inspector & Overview table, CSS status badges, and i18n keys. These tracks merge into a cohesive integration task that wires `web/js/app.js` with the API and verifies the full platform.

**Tech Stack:** Go 1.22+, `github.com/robfig/cron/v3`, SQLite (mattn/go-sqlite3), Vanilla JavaScript (ES6+), HTML5 `<input type="datetime-local">`, CSS3 Design Tokens.

**Spec:** [`docs/superpowers/specs/2026-09-30-comprehensive-cron-management-design.md`](file:///d:/Personal_Sources/ActaCron/docs/superpowers/specs/2026-09-30-comprehensive-cron-management-design.md)

## Global Constraints

- Preserve existing JSDoc parsing rules and backward compatibility for `@cron <expr>`.
- JSDoc annotations remain the single source of truth for scheduling metadata across git repositories.
- When `NoOverlap` is active, concurrent executions of the same script must be skipped and logged without error or panic.
- Timezone parsing must fallback gracefully to `time.Local` if an unrecognized timezone identifier is supplied.
- All Go unit, embed, and package tests (`go test ./...`) must pass with 0 regressions.

## Review Focus

1. **Malformed Date/Time Strings**: If a user supplies an invalid date format in `@cron_start` or `@cron_end` (e.g. `2026-99-99`), the parser must ignore the invalid boundary and log a warning rather than panicking or failing to parse the script.
2. **Invalid Timezone String**: If `@timezone Unknown/Place` is given, the scheduler must fallback to `time.Local` and continue scheduling the job rather than dropping it.
3. **Concurrent Overlap Race**: When a job triggers while its previous execution is still running, it must release execution immediately, log `skipped_overlap`, and not block other scheduled jobs.
4. **Time Window Transitions**: When current time transitions past `@cron_end`, the job must transition to `Expired` and not fire even if the cron timer triggers.
5. **Bidirectional JSDoc Sync**: When changing inputs in the Script Inspector, comments in the code editor must update cleanly without destroying other existing JSDoc tags.

---

### Task 1: Backend JSDoc Parser, Storage Schema & Scheduler Engine

**Worktree / Branch:** `.worktrees/feat-cron-backend` on branch `feat/cron-backend`  
**Files:**
- Modify: `internal/domain/models.go`
- Modify: `internal/parser/parser.go`, `internal/parser/parser_test.go`
- Modify: `internal/storage/sqlite.go`, `internal/storage/storage_test.go`
- Modify: `internal/scheduler/scheduler.go`, `internal/scheduler/scheduler_test.go`
- Modify: `internal/api/handlers.go`, `internal/api/api_test.go`

**Interfaces:**
- Consumes: `domain.FunctionMeta`, `storage.DB`, `cron.Cron`
- Produces:
  - `FunctionMeta.CronStart`, `FunctionMeta.CronEnd`, `FunctionMeta.Timezone`, `FunctionMeta.RetryCount`, `FunctionMeta.RetryDelay`, `FunctionMeta.MaxRuns`, `FunctionMeta.NoOverlap`
  - `CronJobInfo.Status`, `CronJobInfo.Timezone`, `CronJobInfo.CronStart`, `CronJobInfo.CronEnd`, `CronJobInfo.MaxRuns`, `CronJobInfo.RunCount`
  - `Scheduler.ToggleJob(target, enabled)`, `Scheduler.GetJobs() []CronJobInfo`
  - Overlap guard skipping concurrent runs
  - `GET /api/cron` enriched JSON schema

- [ ] **Step 1: Write failing parser and scheduler unit tests**

In `internal/parser/parser_test.go`:
```go
func TestParseComprehensiveCronAnnotations(t *testing.T) {
	code := `/**
 * @cron */5 * * * *
 * @cron_start 2026-10-01 08:00:00
 * @cron_end 2026-10-10 18:00:00
 * @timezone Asia/Ho_Chi_Minh
 * @retry 3 5s
 * @max_runs 50
 * @no_overlap true
 */
function handle() {}`

	meta, err := Parse(code, "test.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CronExpr != "*/5 * * * *" {
		t.Errorf("expected cron expr, got %s", meta.CronExpr)
	}
	if meta.CronStart != "2026-10-01 08:00:00" {
		t.Errorf("expected cron start, got %s", meta.CronStart)
	}
	if meta.CronEnd != "2026-10-10 18:00:00" {
		t.Errorf("expected cron end, got %s", meta.CronEnd)
	}
	if meta.Timezone != "Asia/Ho_Chi_Minh" {
		t.Errorf("expected timezone, got %s", meta.Timezone)
	}
	if meta.RetryCount != 3 || meta.RetryDelay != "5s" {
		t.Errorf("expected retry 3 5s, got %d %s", meta.RetryCount, meta.RetryDelay)
	}
	if meta.MaxRuns != 50 {
		t.Errorf("expected max runs 50, got %d", meta.MaxRuns)
	}
	if !meta.NoOverlap {
		t.Errorf("expected no_overlap true")
	}
}
```

In `internal/scheduler/scheduler_test.go`:
```go
func TestSchedulerLifecycleAndOverlap(t *testing.T) {
	// Test that Pending jobs before CronStart are skipped
	// Test that Expired jobs after CronEnd are skipped
	// Test that runningJobs map prevents overlap
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/parser/... ./internal/scheduler/...`  
Expected: FAIL (fields not defined on `FunctionMeta`).

- [ ] **Step 3: Implement minimal domain, parser, storage, scheduler, and API changes**

1. Update `internal/domain/models.go`:
```go
type FunctionMeta struct {
	Name         string            `json:"name"`
	Package      string            `json:"package"`
	FilePath     string            `json:"file_path"`
	CronExpr     string            `json:"cron_expr"`
	CronStart    string            `json:"cron_start,omitempty"`
	CronEnd      string            `json:"cron_end,omitempty"`
	Timezone     string            `json:"timezone,omitempty"`
	RetryCount   int               `json:"retry_count,omitempty"`
	RetryDelay   string            `json:"retry_delay,omitempty"`
	MaxRuns      int               `json:"max_runs,omitempty"`
	RunCount     int               `json:"run_count,omitempty"`
	NoOverlap    bool              `json:"no_overlap"`
	Timeout      time.Duration     `json:"timeout"`
	IsMcp        bool              `json:"is_mcp"`
	McpDesc      string            `json:"mcp_desc"`
	IsEnabled    bool              `json:"is_enabled"`
	LastStatus   string            `json:"last_status,omitempty"`
	Params       []ParamMeta       `json:"params,omitempty"`
}
```

2. Update `internal/parser/parser.go`:
Parse regexes for `@cron_start`, `@cron_end`, `@timezone`, `@retry`, `@max_runs`, `@no_overlap`. Default `NoOverlap` to `true` when `@cron` is present unless explicitly set to `false`.

3. Update `internal/storage/sqlite.go`:
Add columns in `initSchema`:
```sql
ALTER TABLE function_state ADD COLUMN run_count INTEGER DEFAULT 0;
ALTER TABLE function_state ADD COLUMN cron_start TEXT;
ALTER TABLE function_state ADD COLUMN cron_end TEXT;
ALTER TABLE function_state ADD COLUMN timezone TEXT;
ALTER TABLE function_state ADD COLUMN max_runs INTEGER DEFAULT 0;
```
Implement `IncrementRunCount(pkg, funcName string) error`.

4. Update `internal/scheduler/scheduler.go`:
Add `runningJobs map[string]bool` with mutex.
Compute status:
- `"running"` if `runningJobs[fullKey]`
- `"paused"` if `!fn.IsEnabled`
- `"pending"` if `fn.CronStart != ""` and `now < start`
- `"expired"` if `fn.CronEnd != ""` and `now > end`
- `"completed"` if `fn.MaxRuns > 0 && fn.RunCount >= fn.MaxRuns`
- `"active"` otherwise.
Apply `time.LoadLocation(fn.Timezone)` to cron scheduling.
In execution callback: check start, end, max_runs, and overlap guard.

5. Update `internal/api/handlers.go`:
Return `status`, `next_run`, `prev_run`, `timezone`, `cron_start`, `cron_end`, `max_runs`, `run_count` in `GET /api/cron`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/parser/... ./internal/storage/... ./internal/scheduler/... ./internal/api/...`  
Expected: PASS.

- [ ] **Step 5: Commit backend changes**

```bash
git add internal/domain internal/parser internal/storage internal/scheduler internal/api
git commit -m "feat(backend): implement comprehensive cron lifecycle, timezone, and overlap guard"
```

---

### Task 2: Frontend HTML Templates, CSS Styles & i18n Dictionary

**Worktree / Branch:** `.worktrees/feat-cron-frontend-ui` on branch `feat/cron-frontend-ui`  
**Files:**
- Modify: `web/index.html`
- Modify: `web/css/style.css`
- Modify: `web/js/i18n.js`
- Modify: `web/embed_test.go`

**Interfaces:**
- Produces:
  - Script Inspector elements: `#inspCronToggle`, `#inspTimezone`, `#inspAdvScheduleToggle`, `#inspAdvScheduleContent`, `#inspCronStart`, `#inspCronEnd`, `#inspMaxRuns`, `#inspRetryCount`, `#inspNoOverlap`
  - Overview Table elements: `#overviewCronTable` columns for Function, Schedule & TZ, Status, Next Run, Actions
  - CSS badge classes: `.badge-cron-active`, `.badge-cron-paused`, `.badge-cron-pending`, `.badge-cron-expired`, `.badge-cron-running`
  - i18n keys for EN and VI: `cron_toggle_label`, `timezone_label`, `adv_schedule_label`, `cron_start_label`, `cron_end_label`, `max_runs_label`, `retry_label`, `no_overlap_label`, `status_active`, `status_paused`, `status_pending`, `status_expired`, `status_running`.

- [ ] **Step 1: Write failing embed test for new UI elements and i18n keys**

In `web/embed_test.go`:
```go
func TestComprehensiveCronUIElementsAndI18n(t *testing.T) {
	htmlData, err := web.Assets.ReadFile("index.html")
	if err != nil {
		t.Fatalf("failed reading index.html: %v", err)
	}
	htmlStr := string(htmlData)

	elements := []string{
		`id="inspCronToggle"`,
		`id="inspTimezone"`,
		`id="inspAdvScheduleContent"`,
		`id="inspCronStart"`,
		`id="inspCronEnd"`,
		`id="inspMaxRuns"`,
		`id="inspRetryCount"`,
		`id="inspNoOverlap"`,
	}
	for _, el := range elements {
		if !strings.Contains(htmlStr, el) {
			t.Fatalf("expected index.html to contain %s", el)
		}
	}

	i18nData, err := web.Assets.ReadFile("js/i18n.js")
	if err != nil {
		t.Fatalf("failed reading js/i18n.js: %v", err)
	}
	i18nStr := string(i18nData)
	keys := []string{
		"cron_toggle_label",
		"adv_schedule_label",
		"status_active",
		"status_paused",
		"status_pending",
		"status_expired",
	}
	for _, k := range keys {
		if !strings.Contains(i18nStr, `"`+k+`"`) {
			t.Fatalf("expected js/i18n.js to contain translation key %s", k)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./web/...`  
Expected: FAIL (missing DOM elements and i18n keys).

- [ ] **Step 3: Implement HTML elements, CSS classes, and i18n dictionary**

1. In `web/index.html`:
Add Cron Toggle switch right above `inspCronExpr`.
Add Timezone selector/input.
Add collapsible `<details class="adv-schedule-accordion">` containing:
- Start Datetime (`<input type="datetime-local" id="inspCronStart">`)
- End Datetime (`<input type="datetime-local" id="inspCronEnd">`)
- Max Runs (`<input type="number" id="inspMaxRuns" min="0">`)
- Retry (`<input type="number" id="inspRetryCount" min="0">`)
- Overlap Prevention checkbox (`<input type="checkbox" id="inspNoOverlap" checked>`).
In Overview view: Update `Active Scheduled Crons` table header to include Status, Next Run, and Actions (Toggle + Run Now).

2. In `web/css/style.css`:
Add styles for `.badge-cron-active`, `.badge-cron-paused`, `.badge-cron-pending`, `.badge-cron-expired`, `.badge-cron-running`.
Add styling for `.adv-schedule-accordion`.

3. In `web/js/i18n.js`:
Add English and Vietnamese dictionaries for all new keys.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./web/...`  
Expected: PASS.

- [ ] **Step 5: Commit frontend UI changes**

```bash
git add web/index.html web/css/style.css web/js/i18n.js web/embed_test.go
git commit -m "feat(ui): add comprehensive cron inspector controls, status badges, and i18n keys"
```

---

### Task 3: Integration Merge & Client JavaScript Wiring

**Worktree / Branch:** Main working tree on branch `feat/comprehensive-cron-management`  
**Files:**
- Modify: `web/js/app.js`
- Modify: `web/embed_test.go`

**Interfaces:**
- Consumes: Task 1 API (`GET /api/cron`, `POST /api/cron`), Task 2 DOM elements & CSS badges
- Produces:
  - Bidirectional sync between code JSDoc and Inspector controls:
    - `@cron`, `@cron_start`, `@cron_end`, `@timezone`, `@retry`, `@max_runs`, `@no_overlap`
  - Toggle switch handler calling `POST /api/cron` with `{ action: "toggle", target: pkg/func, enable: bool }`
  - Dynamic Overview table rendering with live countdown to `Next Run` and colored status badges
  - Tree pane badge updates (`CRON` green, `PAUSED` gray, `PENDING` blue, `EXPIRED` red)

- [ ] **Step 1: Merge branches `feat/cron-backend` and `feat/cron-frontend-ui`**

```bash
git checkout -b feat/comprehensive-cron-management master
git merge feat/cron-backend -m "merge: integrate cron backend engine"
git merge feat/cron-frontend-ui -m "merge: integrate cron frontend ui components"
```

- [ ] **Step 2: Write failing test in `web/embed_test.go` for `app.js` wiring**

```go
func TestAppJSComprehensiveCronWiring(t *testing.T) {
	data, err := web.Assets.ReadFile("js/app.js")
	if err != nil {
		t.Fatalf("failed reading js/app.js: %v", err)
	}
	appStr := string(data)
	required := []string{
		"inspCronToggle",
		"inspCronStart",
		"inspCronEnd",
		"inspTimezone",
		"inspNoOverlap",
		"badge-cron-paused",
	}
	for _, req := range required {
		if !strings.Contains(appStr, req) {
			t.Fatalf("expected js/app.js to reference %s", req)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test -v ./web/...`  
Expected: FAIL (missing references in `app.js`).

- [ ] **Step 4: Implement client-side logic in `web/js/app.js`**

1. Wire up inspector controls:
   - On selecting a script: populate `#inspCronToggle`, `#inspCronExpr`, `#inspCronStart`, `#inspCronEnd`, `#inspTimezone`, `#inspMaxRuns`, `#inspRetryCount`, `#inspNoOverlap` from `activeScript`.
   - On modifying inspector fields: update `activeScript` properties and update code editor JSDoc annotations accordingly.
   - On typing code in editor: parse JSDoc annotations and update inspector inputs.
2. Wire up `#inspCronToggle`:
   - Send `POST /api/cron` with `{ action: "toggle", target: activeScript.package + "/" + activeScript.name, enable: isChecked }`.
   - Trigger `loadFunctions()` or refresh tree item badge without reloading entire page.
3. Update `renderOverview()`:
   - Query `/api/cron` to get rich job data (`status`, `next_run`, `prev_run`, `timezone`).
   - Render status badges and inline toggle switches in the `Active Scheduled Crons` table.
4. Update Tree list rendering:
   - Display colored badge based on status (`CRON`, `PAUSED`, `PENDING`, `EXPIRED`).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v ./web/...`  
Expected: PASS.

- [ ] **Step 6: Commit client wiring changes**

```bash
git add web/js/app.js web/embed_test.go
git commit -m "feat(ui): wire client app with comprehensive cron controls, sync, and status badges"
```

---

### Task 4: End-to-End Verification & Sanity Check

**Worktree / Branch:** `feat/comprehensive-cron-management`  
**Files:**
- Test: Full project test suite across all 15 packages.
- Build: Verify build artifact `dist/actacron_test.exe`.

- [ ] **Step 1: Run full Go test suite**

Run: `go test -v ./...`  
Expected: All 15 packages pass with 0 errors.

- [ ] **Step 2: Verify binary compilation**

Run: `go build -o dist/actacron_test.exe .`  
Expected: Clean compilation with 0 warnings or errors.

- [ ] **Step 3: Clean up temporary test binary**

Run: `Remove-Item dist/actacron_test.exe`

- [ ] **Step 4: Commit any test adjustments or finalize plan**

```bash
git commit --allow-empty -m "chore: verify comprehensive cron management end-to-end"
```
