package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"actacron/internal/domain"
	"actacron/internal/engine"
	"actacron/internal/parser"
	"actacron/internal/storage"
	"github.com/dop251/goja"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

type depthKeyType struct{}

var depthKey = depthKeyType{}

type Manager struct {
	packagesDir string
	runner      *engine.Runner
	db          *storage.DB
	mu          sync.RWMutex
	packages    map[string]*domain.PackageInfo
	functions   map[string]*domain.FunctionMeta // key: "pkg/func"
	fileCode    map[string]string               // key: "pkg/func" -> code
	watcher     *fsnotify.Watcher
	stopChan    chan struct{}
}

func New(packagesDir string, runner *engine.Runner, db *storage.DB) *Manager {
	return &Manager{
		packagesDir: packagesDir,
		runner:      runner,
		db:          db,
		packages:    make(map[string]*domain.PackageInfo),
		functions:   make(map[string]*domain.FunctionMeta),
		fileCode:    make(map[string]string),
		stopChan:    make(chan struct{}),
	}
}

func (m *Manager) PackagesDir() string {
	return m.packagesDir
}

func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.packagesDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(m.packagesDir)
	if err != nil {
		return err
	}

	newPackages := make(map[string]*domain.PackageInfo)
	newFunctions := make(map[string]*domain.FunctionMeta)
	newFileCode := make(map[string]string)

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		pkgName := entry.Name()
		pkgPath := filepath.Join(m.packagesDir, pkgName)
		isGit := false
		if _, err := os.Stat(filepath.Join(pkgPath, ".git")); err == nil {
			isGit = true
		}

		pkgInfo := &domain.PackageInfo{
			Name:      pkgName,
			Path:      pkgPath,
			IsGit:     isGit,
			Status:    "clean",
			Functions: []string{},
			UpdatedAt: time.Now(),
		}

		// Read .js files in package
		files, err := os.ReadDir(pkgPath)
		if err == nil {
			for _, file := range files {
				if file.IsDir() || filepath.Ext(file.Name()) != ".js" {
					continue
				}

				filePath := filepath.Join(pkgPath, file.Name())
				codeBytes, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}

				code := string(codeBytes)
				meta, err := parser.Parse(code, file.Name())
				if err != nil {
					// Syntax error or parse error, skip or log
					continue
				}

				meta.Package = pkgName
				meta.FilePath = file.Name()

				// Query enabled state from DB
				if m.db != nil {
					enabled, _, _ := m.db.GetFunctionState(pkgName, meta.Name)
					meta.IsEnabled = enabled
				}

				fullKey := pkgName + "/" + meta.Name
				newFunctions[fullKey] = meta
				newFileCode[fullKey] = code
				pkgInfo.Functions = append(pkgInfo.Functions, meta.Name)
			}
		}

		newPackages[pkgName] = pkgInfo
	}

	m.packages = newPackages
	m.functions = newFunctions
	m.fileCode = newFileCode
	return nil
}

func (m *Manager) ListPackages() []*domain.PackageInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var res []*domain.PackageInfo
	for _, p := range m.packages {
		res = append(res, p)
	}
	return res
}

func (m *Manager) GetPackage(name string) *domain.PackageInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.packages[name]
}

func (m *Manager) ListFunctions() []*domain.FunctionMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var res []*domain.FunctionMeta
	for _, f := range m.functions {
		res = append(res, f)
	}
	return res
}

func (m *Manager) GetFunction(key string) *domain.FunctionMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if f, exists := m.functions[key]; exists {
		return f
	}

	// Try resolving without package prefix
	for k, f := range m.functions {
		if strings.HasSuffix(k, "/"+key) || f.Name == key {
			return f
		}
	}
	return nil
}

func (m *Manager) GetFunctionCode(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if code, exists := m.fileCode[key]; exists {
		return code, nil
	}

	for k, code := range m.fileCode {
		if strings.HasSuffix(k, "/"+key) {
			return code, nil
		}
	}
	return "", fmt.Errorf("function '%s' not found", key)
}

func (m *Manager) SaveFunctionCode(pkgName, funcFileName, code string) error {
	funcFileName = filepath.Base(funcFileName)
	// First validate syntax with AST parser
	_, err := parser.Parse(code, funcFileName)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	pkgPath := filepath.Join(m.packagesDir, pkgName)
	if err := os.MkdirAll(pkgPath, 0755); err != nil {
		return err
	}

	if !strings.HasSuffix(funcFileName, ".js") {
		funcFileName += ".js"
	}

	filePath := filepath.Join(pkgPath, funcFileName)
	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		return err
	}

	return m.Reload()
}

func (m *Manager) Call(ctx context.Context, target string, params interface{}) (interface{}, error) {
	return m.callWithTrigger(ctx, target, params, "manual")
}

func (m *Manager) CallWithTrigger(ctx context.Context, target string, params interface{}, trigger string) (interface{}, error) {
	return m.callWithTrigger(ctx, target, params, trigger)
}

func (m *Manager) callWithTrigger(ctx context.Context, target string, params interface{}, trigger string) (interface{}, error) {
	fn := m.GetFunction(target)
	if fn == nil {
		return nil, fmt.Errorf("function '%s' not found", target)
	}

	fullKey := fn.Package + "/" + fn.Name
	code, err := m.GetFunctionCode(fullKey)
	if err != nil {
		return nil, err
	}

	depth := 0
	if val := ctx.Value(depthKey); val != nil {
		if d, ok := val.(int); ok {
			depth = d
		}
	}
	if depth > 10 {
		return nil, fmt.Errorf("maximum call stack depth of 10 exceeded (cyclic call detected on %s)", target)
	}

	newCtx := context.WithValue(ctx, depthKey, depth+1)
	execID := uuid.New().String()

	// Inject `call` and `require` into runtime
	setupFn := func(vm *goja.Runtime) {
		// 1. Cross-calling API
		vm.Set("call", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 {
				panic(vm.ToValue("call() requires a function name argument"))
			}
			targetName := call.Arguments[0].String()
			var innerParams interface{}
			if len(call.Arguments) > 1 {
				innerParams = call.Arguments[1].Export()
			}

			innerRes, innerErr := m.callWithTrigger(newCtx, targetName, innerParams, "call")
			if innerErr != nil {
				panic(vm.ToValue(innerErr.Error()))
			}
			return vm.ToValue(innerRes)
		})

		// 2. Internal package require API
		vm.Set("require", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 {
				panic(vm.ToValue("require() requires a path argument"))
			}
			relPath := call.Arguments[0].String()
			pkgPath := filepath.Join(m.packagesDir, fn.Package)

			resolvedPath := filepath.Clean(filepath.Join(pkgPath, relPath))
			if !strings.HasPrefix(resolvedPath, filepath.Clean(pkgPath)) {
				panic(vm.ToValue("security error: path traversal outside package not permitted"))
			}

			reqCode, reqErr := os.ReadFile(resolvedPath)
			if reqErr != nil {
				panic(vm.ToValue(fmt.Sprintf("cannot read required file %s: %v", relPath, reqErr)))
			}

			// Run in module scope
			moduleObj := vm.NewObject()
			exportsObj := vm.NewObject()
			moduleObj.Set("exports", exportsObj)
			vm.Set("module", moduleObj)
			vm.Set("exports", exportsObj)

			_, reqExecErr := vm.RunScript(relPath, string(reqCode))
			if reqExecErr != nil {
				panic(vm.ToValue(fmt.Sprintf("error executing %s: %v", relPath, reqExecErr)))
			}

			return moduleObj.Get("exports")
		})
	}

	res, execErr := m.runner.ExecuteWithSetup(newCtx, fn.Package, fn.FilePath, code, params, setupFn)
	if execErr != nil {
		return nil, execErr
	}

	// Persist log to SQLite
	if m.db != nil {
		paramsJSON, _ := json.Marshal(params)
		outputJSON, _ := json.Marshal(res.Output)
		log := &domain.ExecutionLog{
			ExecutionID:  execID,
			PackageName:  fn.Package,
			FunctionName: fn.Name,
			TriggerType:  trigger,
			Status:       res.Status,
			InputParams:  string(paramsJSON),
			OutputData:   string(outputJSON),
			ErrorMessage: res.Error,
			ConsoleLogs:  strings.Join(res.ConsoleLogs, "\n"),
			DurationMs:   res.DurationMs,
			CreatedAt:    time.Now(),
		}
		m.db.InsertLog(log)
		m.db.SetFunctionState(fn.Package, fn.Name, fn.IsEnabled, res.Status)
	}

	if res.Status != "success" {
		return nil, fmt.Errorf("function execution failed: %s", res.Error)
	}

	return res.Output, nil
}

func (m *Manager) StartWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	m.watcher = watcher

	go func() {
		for {
			select {
			case <-m.stopChan:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					// Hot reload
					m.Reload()
				}
			case <-watcher.Errors:
			}
		}
	}()

	return watcher.Add(m.packagesDir)
}

func (m *Manager) StopWatcher() {
	if m.stopChan != nil {
		close(m.stopChan)
	}
	if m.watcher != nil {
		m.watcher.Close()
	}
}
