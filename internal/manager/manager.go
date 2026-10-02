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
	"actacron/internal/settings"
	"actacron/internal/storage"
	"github.com/dop251/goja"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

type depthKeyType struct{}

var depthKey = depthKeyType{}

type Manager struct {
	packagesDir      string
	runner           *engine.Runner
	db               *storage.DB
	mu               sync.RWMutex
	packages         map[string]*domain.PackageInfo
	functions        map[string]*domain.FunctionMeta // key: "pkg/func"
	fileCode         map[string]string               // key: "pkg/func" -> code
	workspaceConfigs map[string]*domain.WorkspaceConfig
	workspaceEnvs    map[string]map[string]string
	globalEnvPath    string
	watcher          *fsnotify.Watcher
	watchedPaths     map[string]bool
	stopChan         chan struct{}
}

func New(packagesDir string, runner *engine.Runner, db *storage.DB) *Manager {
	return &Manager{
		packagesDir:      packagesDir,
		runner:           runner,
		db:               db,
		packages:         make(map[string]*domain.PackageInfo),
		functions:        make(map[string]*domain.FunctionMeta),
		fileCode:         make(map[string]string),
		workspaceConfigs: make(map[string]*domain.WorkspaceConfig),
		workspaceEnvs:    make(map[string]map[string]string),
		watchedPaths:     make(map[string]bool),
		stopChan:         make(chan struct{}),
	}
}

func (m *Manager) PackagesDir() string {
	return m.packagesDir
}

func (m *Manager) Runner() *engine.Runner {
	return m.runner
}

func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.packagesDir, 0755); err != nil {
		return err
	}

	// Auto-scaffold _shared library if not present
	_ = ScaffoldSharedLibrary(filepath.Join(m.packagesDir, "_shared"))

	entries, err := os.ReadDir(m.packagesDir)
	if err != nil {
		return err
	}

	newPackages := make(map[string]*domain.PackageInfo)
	newFunctions := make(map[string]*domain.FunctionMeta)
	newFileCode := make(map[string]string)
	newWorkspaceConfigs := make(map[string]*domain.WorkspaceConfig)
	newWorkspaceEnvs := make(map[string]map[string]string)

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

		if pkgName == "_shared" {
			pkgInfo := &domain.PackageInfo{
				Name:      "_shared",
				Path:      pkgPath,
				IsGit:     isGit,
				Status:    "clean",
				Functions: []string{},
				UpdatedAt: time.Now(),
			}
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
					baseFileName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
					fullKey := "_shared/" + baseFileName
					newFunctions[fullKey] = &domain.FunctionMeta{
						Name:        baseFileName,
						Package:     "_shared",
						FilePath:    file.Name(),
						Description: "Shared Library Module",
						IsEnabled:   false,
						UpdatedAt:   time.Now(),
					}
					newFileCode[fullKey] = code
					pkgInfo.Functions = append(pkgInfo.Functions, baseFileName)
				}
			}
			newPackages["_shared"] = pkgInfo
			continue
		}

		m.scanPackageFolder(pkgName, pkgPath, false, isGit, newPackages, newFunctions, newFileCode, newWorkspaceConfigs, newWorkspaceEnvs)
	}

	currentExternalPaths := make(map[string]bool)

	// Load external workspaces from database
	if m.db != nil {
		if extList, err := m.db.ListExternalWorkspaces(); err == nil {
			for _, extWs := range extList {
				fi, err := os.Stat(extWs.Path)
				if err != nil || !fi.IsDir() {
					// Gracefully register missing external workspace so user can view/unlink it
					pkgInfo := &domain.PackageInfo{
						Name:       extWs.Name,
						Path:       extWs.Path,
						IsExternal: true,
						Status:     "missing",
						Functions:  []string{},
						UpdatedAt:  time.Now(),
					}
					newPackages[extWs.Name] = pkgInfo
					newWorkspaceConfigs[extWs.Name] = &domain.WorkspaceConfig{Name: extWs.Name}
					continue
				}

				isGit := false
				if _, err := os.Stat(filepath.Join(extWs.Path, ".git")); err == nil {
					isGit = true
				}
				m.scanPackageFolder(extWs.Name, extWs.Path, true, isGit, newPackages, newFunctions, newFileCode, newWorkspaceConfigs, newWorkspaceEnvs)
				currentExternalPaths[extWs.Path] = true
				if m.watcher != nil {
					_ = m.watcher.Add(extWs.Path)
				}
			}
		}
	}

	// Unwatch any previously watched external paths that are no longer active
	if m.watcher != nil {
		for oldPath := range m.watchedPaths {
			if !currentExternalPaths[oldPath] {
				_ = m.watcher.Remove(oldPath)
			}
		}
	}
	m.watchedPaths = currentExternalPaths

	m.packages = newPackages
	m.functions = newFunctions
	m.fileCode = newFileCode
	m.workspaceConfigs = newWorkspaceConfigs
	m.workspaceEnvs = newWorkspaceEnvs
	return nil
}

func (m *Manager) scanPackageFolder(
	pkgName, pkgPath string,
	isExternal bool,
	isGit bool,
	newPackages map[string]*domain.PackageInfo,
	newFunctions map[string]*domain.FunctionMeta,
	newFileCode map[string]string,
	newWorkspaceConfigs map[string]*domain.WorkspaceConfig,
	newWorkspaceEnvs map[string]map[string]string,
) {
	// 1. Read workspace.json if present
	wsCfgPath := filepath.Join(pkgPath, "workspace.json")
	wsCfg := &domain.WorkspaceConfig{Name: pkgName}
	if wsData, err := os.ReadFile(wsCfgPath); err == nil {
		_ = json.Unmarshal(wsData, wsCfg)
	}
	newWorkspaceConfigs[pkgName] = wsCfg

	// 2. Read workspace .env if present
	wsEnvPath := filepath.Join(pkgPath, ".env")
	if envMap, err := settings.ReadEnv(wsEnvPath); err == nil && len(envMap) > 0 {
		newWorkspaceEnvs[pkgName] = envMap
	} else {
		wsExampleEnvPath := filepath.Join(pkgPath, ".env.example")
		if envMapExample, errExample := settings.ReadEnv(wsExampleEnvPath); errExample == nil && len(envMapExample) > 0 {
			newWorkspaceEnvs[pkgName] = envMapExample
		}
	}

	pkgInfo := &domain.PackageInfo{
		Name:       pkgName,
		Path:       pkgPath,
		IsGit:      isGit,
		IsExternal: isExternal,
		Status:     "clean",
		Functions:  []string{},
		UpdatedAt:  time.Now(),
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

			// Query enabled state and run count from DB
			if m.db != nil {
				enabled, _, _ := m.db.GetFunctionState(pkgName, meta.Name)
				meta.IsEnabled = enabled
				runCount, _ := m.db.GetRunCount(pkgName, meta.Name)
				meta.RunCount = runCount
			}

			baseFileName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			if meta.Name == "" {
				meta.Name = baseFileName
			}

			fullKey := pkgName + "/" + baseFileName
			newFunctions[fullKey] = meta
			newFileCode[fullKey] = code
			pkgInfo.Functions = append(pkgInfo.Functions, meta.Name)
		}
	}

	newPackages[pkgName] = pkgInfo
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

func (m *Manager) GetPackageDir(pkgName string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if pkg, ok := m.packages[pkgName]; ok && pkg.Path != "" {
		return pkg.Path, nil
	}
	return filepath.Join(m.packagesDir, pkgName), nil
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

	// Try resolving with package/funcName or without package prefix
	for k, f := range m.functions {
		if k == key || f.Package+"/"+f.Name == key || strings.HasSuffix(k, "/"+key) || f.Name == key {
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
		f := m.functions[k]
		if k == key || (f != nil && f.Package+"/"+f.Name == key) || strings.HasSuffix(k, "/"+key) || (f != nil && f.Name == key) {
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

	pkgPath, _ := m.GetPackageDir(pkgName)
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
	if _, hasDeadline := newCtx.Deadline(); !hasDeadline {
		timeout := m.GetEffectiveTimeout(fn.Package, fn.Name)
		var cancel context.CancelFunc
		newCtx, cancel = context.WithTimeout(newCtx, timeout)
		defer cancel()
	}
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

		// 2. Internal package & _shared require API
		pkgDir, _ := m.GetPackageDir(fn.Package)
		currentDir := pkgDir

		var executeRequire func(resolvedPath, scriptName string) (goja.Value, error)
		executeRequire = func(resolvedPath, scriptName string) (goja.Value, error) {
			cleanPackagesDir := filepath.Clean(m.packagesDir)
			cleanPkgDir := filepath.Clean(pkgDir)
			cleanSharedDir := filepath.Clean(filepath.Join(m.packagesDir, "_shared"))

			isInsidePkg := strings.HasPrefix(resolvedPath, cleanPkgDir+string(filepath.Separator)) || resolvedPath == cleanPkgDir
			isInsidePackages := strings.HasPrefix(resolvedPath, cleanPackagesDir+string(filepath.Separator)) || resolvedPath == cleanPackagesDir
			isInsideShared := strings.HasPrefix(resolvedPath, cleanSharedDir+string(filepath.Separator)) || resolvedPath == cleanSharedDir

			if !isInsidePkg && !isInsidePackages && !isInsideShared {
				return nil, fmt.Errorf("security error: path traversal outside workspace or _shared not permitted")
			}

			reqCode, reqErr := os.ReadFile(resolvedPath)
			if reqErr != nil {
				return nil, fmt.Errorf("cannot read required file %s: %v", scriptName, reqErr)
			}

			oldDir := currentDir
			currentDir = filepath.Dir(resolvedPath)
			defer func() { currentDir = oldDir }()

			moduleObj := vm.NewObject()
			exportsObj := vm.NewObject()
			moduleObj.Set("exports", exportsObj)

			prevModule := vm.Get("module")
			prevExports := vm.Get("exports")

			vm.Set("module", moduleObj)
			vm.Set("exports", exportsObj)

			_, reqExecErr := vm.RunScript(scriptName, string(reqCode))

			if prevModule != nil {
				vm.Set("module", prevModule)
			}
			if prevExports != nil {
				vm.Set("exports", prevExports)
			}

			if reqExecErr != nil {
				return nil, fmt.Errorf("error executing %s: %v", scriptName, reqExecErr)
			}

			return moduleObj.Get("exports"), nil
		}

		resolveRequirePath := func(rawPath string) (string, error) {
			var resolvedPath string
			if rawPath == "_shared" || strings.HasPrefix(rawPath, "_shared/") || strings.HasPrefix(rawPath, "../_shared") {
				sub := rawPath
				if strings.HasPrefix(sub, "../_shared") {
					sub = strings.TrimPrefix(sub, "../_shared")
					sub = strings.TrimPrefix(sub, "/")
				} else {
					sub = strings.TrimPrefix(sub, "_shared/")
					if sub == "_shared" {
						sub = ""
					}
				}
				if sub == "" {
					sub = "index.js"
				}
				if !strings.HasSuffix(sub, ".js") {
					sub += ".js"
				}
				resolvedPath = filepath.Clean(filepath.Join(m.packagesDir, "_shared", sub))
			} else {
				sub := rawPath
				if !strings.HasSuffix(sub, ".js") {
					targetDir := filepath.Join(currentDir, sub)
					if stat, err := os.Stat(targetDir); err == nil && stat.IsDir() {
						sub = filepath.Join(sub, "index.js")
					} else {
						sub += ".js"
					}
				}
				resolvedPath = filepath.Clean(filepath.Join(currentDir, sub))
			}
			return resolvedPath, nil
		}

		vm.Set("require", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) == 0 {
				panic(vm.ToValue("require() requires a path argument"))
			}
			rawPath := call.Arguments[0].String()
			resolvedPath, err := resolveRequirePath(rawPath)
			if err != nil {
				panic(vm.ToValue(err.Error()))
			}

			exports, err := executeRequire(resolvedPath, rawPath)
			if err != nil {
				panic(vm.ToValue(err.Error()))
			}
			return exports
		})

		// 3. Scoped environment variables
		resolver, allVars := m.getMergedEnv(fn.Package)
		engine.RegisterEnv(vm, resolver, allVars)

		// 4. Inject global shared object
		sharedObj := vm.NewObject()
		sharedDir := filepath.Join(m.packagesDir, "_shared")
		indexFile := filepath.Join(sharedDir, "index.js")
		if _, err := os.Stat(indexFile); err == nil {
			if idxVal, err := executeRequire(indexFile, "_shared/index.js"); err == nil && idxVal != nil {
				if idxObj, ok := idxVal.(*goja.Object); ok {
					for _, k := range idxObj.Keys() {
						sharedObj.Set(k, idxObj.Get(k))
					}
				}
			}
		} else {
			if entries, err := os.ReadDir(sharedDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
						base := strings.TrimSuffix(e.Name(), ".js")
						fullP := filepath.Join(sharedDir, e.Name())
						if modVal, err := executeRequire(fullP, "_shared/"+e.Name()); err == nil && modVal != nil {
							sharedObj.Set(base, modVal)
						}
					}
				}
			}
		}
		vm.Set("shared", sharedObj)
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

	if err := watcher.Add(m.packagesDir); err != nil {
		return err
	}
	m.mu.RLock()
	for path := range m.watchedPaths {
		_ = watcher.Add(path)
	}
	m.mu.RUnlock()
	return nil
}

func (m *Manager) StopWatcher() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopChan != nil {
		select {
		case <-m.stopChan:
		default:
			close(m.stopChan)
		}
	}
	if m.watcher != nil {
		_ = m.watcher.Close()
		m.watcher = nil
	}
}

// DeleteFunction deletes a script file from disk and reloads packages.
func (m *Manager) DeleteFunction(pkgName, funcFileName string) error {
	funcFileName = filepath.Base(funcFileName)
	if !strings.HasSuffix(funcFileName, ".js") {
		funcFileName += ".js"
	}
	pkgPath, _ := m.GetPackageDir(pkgName)
	filePath := filepath.Join(pkgPath, funcFileName)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file '%s' not found in package '%s'", funcFileName, pkgName)
		}
		return err
	}

	return m.Reload()
}

// DeletePackage removes an internal package directory from disk and reloads.
// It rejects empty names, _shared, path traversals, non-existent packages, and external workspaces.
func (m *Manager) DeletePackage(pkgName string) error {
	cleanName := filepath.Clean(strings.TrimSpace(pkgName))
	if cleanName == "" || cleanName == "." || cleanName == ".." || cleanName == "_shared" {
		return fmt.Errorf("cannot delete package '%s'", pkgName)
	}
	if strings.Contains(cleanName, "/") || strings.Contains(cleanName, "\\") {
		return fmt.Errorf("invalid package name '%s'", pkgName)
	}

	m.mu.RLock()
	if pkg, exists := m.packages[cleanName]; exists && pkg.IsExternal {
		m.mu.RUnlock()
		return fmt.Errorf("package '%s' is an external workspace; use unlink instead", cleanName)
	}
	m.mu.RUnlock()

	pkgDir := filepath.Join(m.packagesDir, cleanName)
	fi, err := os.Stat(pkgDir)
	if os.IsNotExist(err) || !fi.IsDir() {
		return fmt.Errorf("package '%s' does not exist", cleanName)
	}

	if err := os.RemoveAll(pkgDir); err != nil {
		return fmt.Errorf("failed to delete package directory: %w", err)
	}

	return m.Reload()
}


func (m *Manager) GetWorkspaceConfig(pkgName string) *domain.WorkspaceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if cfg, ok := m.workspaceConfigs[pkgName]; ok {
		return cfg
	}
	return &domain.WorkspaceConfig{Name: pkgName}
}

func (m *Manager) GetEffectiveTimeout(pkgName, funcName string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Check Function JSDoc @timeout
	fullKey := pkgName + "/" + funcName
	if fn, ok := m.functions[fullKey]; ok && fn != nil && fn.TimeoutSeconds > 0 {
		return time.Duration(fn.TimeoutSeconds) * time.Second
	}
	for _, fn := range m.functions {
		if fn != nil && fn.Package == pkgName && fn.Name == funcName && fn.TimeoutSeconds > 0 {
			return time.Duration(fn.TimeoutSeconds) * time.Second
		}
	}

	// 2. Check WorkspaceConfig timeout
	if wsCfg, ok := m.workspaceConfigs[pkgName]; ok && wsCfg != nil && wsCfg.TimeoutSeconds > 0 {
		return time.Duration(wsCfg.TimeoutSeconds) * time.Second
	}

	// 3. Fallback to Runner Default Timeout
	if m.runner != nil {
		sec := m.runner.DefaultTimeoutSec()
		if sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}

	return 30 * time.Second
}

func (m *Manager) GetWorkspaceEnv(pkgName string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if envMap, ok := m.workspaceEnvs[pkgName]; ok {
		res := make(map[string]string, len(envMap))
		for k, v := range envMap {
			res[k] = v
		}
		return res
	}
	return make(map[string]string)
}

func (m *Manager) HasWorkspaceEnvFile(pkgName string) bool {
	pkgPath, _ := m.GetPackageDir(pkgName)
	envPath := filepath.Join(pkgPath, ".env")
	info, err := os.Stat(envPath)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func (m *Manager) GetWorkspaceExampleEnv(pkgName string) map[string]string {
	pkgPath, _ := m.GetPackageDir(pkgName)
	envPath := filepath.Join(pkgPath, ".env.example")
	if envMap, err := settings.ReadEnv(envPath); err == nil {
		return envMap
	}
	return make(map[string]string)
}

func (m *Manager) SetGlobalEnvPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globalEnvPath = path
}

func (m *Manager) getMergedEnv(pkgName string) (func(string) string, map[string]string) {
	m.mu.RLock()
	wsEnv := m.workspaceEnvs[pkgName]
	globalPath := m.globalEnvPath
	if globalPath == "" {
		globalPath = filepath.Join(filepath.Dir(m.packagesDir), ".env")
	}
	m.mu.RUnlock()

	globalEnv, _ := settings.ReadEnv(globalPath)

	allVars := make(map[string]string)
	// 1. OS env
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			allVars[parts[0]] = parts[1]
		}
	}
	// 2. Global env overrides OS env
	for k, v := range globalEnv {
		allVars[k] = v
	}
	// 3. Workspace env overrides Global env
	for k, v := range wsEnv {
		allVars[k] = v
	}

	resolver := func(key string) string {
		if wsEnv != nil {
			if val, ok := wsEnv[key]; ok {
				return val
			}
		}
		if globalEnv != nil {
			if val, ok := globalEnv[key]; ok {
				return val
			}
		}
		return os.Getenv(key)
	}

	return resolver, allVars
}

