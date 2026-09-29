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
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
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

	retMap, ok := res.Output.(map[string]interface{})
	if !ok || retMap["echo"] != "acta" || retMap["val"] != "stored_val" {
		t.Fatalf("unexpected return data: %v", res.Output)
	}
}

func TestInfiniteLoopTimeout(t *testing.T) {
	dbFile := "test_loop.db"
	defer os.Remove(dbFile)
	db, err := storage.New(dbFile)
	if err != nil {
		t.Fatalf("failed db: %v", err)
	}
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
