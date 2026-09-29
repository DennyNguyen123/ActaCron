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

