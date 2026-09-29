package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"actacron/internal/storage"
	"github.com/dop251/goja"
)

type ExecutionResult struct {
	Status      string      `json:"status"` // success, failed, timeout
	Output      interface{} `json:"output"`
	Error       string      `json:"error,omitempty"`
	ConsoleLogs []string    `json:"console_logs"`
	DurationMs  int64       `json:"duration_ms"`
}

type Runner struct {
	db                *storage.DB
	defaultTimeoutSec int
	allowShell        bool
	mu                sync.RWMutex
}

func New(db *storage.DB, defaultTimeoutSec int, allowShell bool) *Runner {
	if defaultTimeoutSec <= 0 {
		defaultTimeoutSec = 30
	}
	return &Runner{
		db:                db,
		defaultTimeoutSec: defaultTimeoutSec,
		allowShell:        allowShell,
	}
}

func (r *Runner) SetAllowShell(allow bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allowShell = allow
}

func (r *Runner) Execute(
	ctx context.Context,
	pkgName string,
	scriptPath string,
	code string,
	params interface{},
) (*ExecutionResult, error) {
	startTime := time.Now()
	var consoleLogs []string

	vm := goja.New()

	// Register built-in helper APIs
	registerConsole(vm, &consoleLogs)
	registerStorage(vm, r.db, pkgName)
	registerCrypto(vm)
	registerBase64(vm)
	registerHTTP(vm)
	registerSleep(vm)
	registerEnv(vm)

	r.mu.RLock()
	allowShell := r.allowShell
	r.mu.RUnlock()
	registerExec(vm, allowShell)

	// Timeout / cancellation guard
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			vm.Interrupt("execution timeout or cancelled")
		case <-done:
		}
	}()

	// 1. Run script source code
	_, err := vm.RunScript(scriptPath, code)
	if err != nil {
		duration := time.Since(startTime).Milliseconds()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || stringsContains(err.Error(), "timeout") {
			return &ExecutionResult{
				Status:      "timeout",
				Error:       "execution timeout exceeded",
				ConsoleLogs: consoleLogs,
				DurationMs:  duration,
			}, nil
		}
		return &ExecutionResult{
			Status:      "failed",
			Error:       err.Error(),
			ConsoleLogs: consoleLogs,
			DurationMs:  duration,
		}, nil
	}

	// 2. Locate main function
	mainVal := vm.Get("main")
	if mainVal == nil || goja.IsNull(mainVal) || goja.IsUndefined(mainVal) {
		duration := time.Since(startTime).Milliseconds()
		return &ExecutionResult{
			Status:      "failed",
			Error:       "script does not define a 'main' function",
			ConsoleLogs: consoleLogs,
			DurationMs:  duration,
		}, nil
	}

	mainFn, ok := goja.AssertFunction(mainVal)
	if !ok {
		duration := time.Since(startTime).Milliseconds()
		return &ExecutionResult{
			Status:      "failed",
			Error:       "'main' is not a callable function",
			ConsoleLogs: consoleLogs,
			DurationMs:  duration,
		}, nil
	}

	// 3. Invoke main(params)
	paramVal := vm.ToValue(params)
	resultVal, err := mainFn(goja.Undefined(), paramVal)
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || stringsContains(err.Error(), "timeout") {
			return &ExecutionResult{
				Status:      "timeout",
				Error:       "execution timeout exceeded",
				ConsoleLogs: consoleLogs,
				DurationMs:  duration,
			}, nil
		}
		return &ExecutionResult{
			Status:      "failed",
			Error:       err.Error(),
			ConsoleLogs: consoleLogs,
			DurationMs:  duration,
		}, nil
	}

	var output interface{}
	if resultVal != nil && !goja.IsUndefined(resultVal) && !goja.IsNull(resultVal) {
		output = resultVal.Export()
	}

	return &ExecutionResult{
		Status:      "success",
		Output:      output,
		ConsoleLogs: consoleLogs,
		DurationMs:  duration,
	}, nil
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
