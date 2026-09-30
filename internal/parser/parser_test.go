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

func TestCheckHealth(t *testing.T) {
	code := `/**
 * @name check_health
 * @description Periodic system uptime and health check
 * @cron 0/30 * * * *
 * @mcp false
 */
function main() {
    return { status: "healthy" };
}`
	meta, err := parser.Parse(code, "check_health.js")
	if err != nil {
		t.Fatalf("check_health parse error: %v", err)
	}
	if meta.CronExpr != "0/30 * * * *" {
		t.Fatalf("expected 0/30 * * * *, got %s", meta.CronExpr)
	}
}

func TestParse_Timeout(t *testing.T) {
	code := `/**
 * @name data_sync
 * @timeout 90
 * @description Syncs large datasets with extended timeout
 */
function main(params) {
    return { ok: true };
}`
	meta, err := parser.Parse(code, "sync.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.TimeoutSeconds != 90 {
		t.Errorf("expected TimeoutSeconds=90, got %d", meta.TimeoutSeconds)
	}
}

func TestParseComprehensiveCronAnnotations(t *testing.T) {
	code := `/**
 * @cron 0/5 * * * *
 * @cron_start 2026-10-01 08:00:00
 * @cron_end 2026-10-10 18:00:00
 * @timezone Asia/Ho_Chi_Minh
 * @retry 3 5s
 * @max_runs 50
 * @no_overlap true
 */
function handle() {}`

	meta, err := parser.Parse(code, "test.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.CronExpr != "0/5 * * * *" {
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

func TestParseCronDefaultsAndInvalidDates(t *testing.T) {
	code := `/**
 * @cron 0 * * * *
 * @cron_start 2026-99-99
 * @retry 2
 */
function run() {}`

	meta, err := parser.Parse(code, "invalid_date.js")
	if err != nil {
		t.Fatalf("parser should not fail on malformed date: %v", err)
	}
	// Invalid start date should be ignored
	if meta.CronStart != "" {
		t.Errorf("expected invalid cron_start to be empty/ignored, got %s", meta.CronStart)
	}
	// @retry 2 defaults delay to 5s
	if meta.RetryCount != 2 || meta.RetryDelay != "5s" {
		t.Errorf("expected retry 2 with default 5s, got %d %s", meta.RetryCount, meta.RetryDelay)
	}
	// Cron present without @no_overlap defaults to true
	if !meta.NoOverlap {
		t.Errorf("expected no_overlap default to true when @cron is present")
	}
}

func TestParseCronNoOverlapExplicitFalse(t *testing.T) {
	code := `/**
 * @cron 0 * * * *
 * @no_overlap false
 */
function run() {}`

	meta, err := parser.Parse(code, "no_overlap.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.NoOverlap {
		t.Errorf("expected no_overlap to be false")
	}
}

