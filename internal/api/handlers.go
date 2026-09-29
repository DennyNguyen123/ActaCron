package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"actacron/internal/domain"
	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
	"actacron/internal/settings"
	"actacron/internal/storage"
)

type APIHandler struct {
	mgr         *manager.Manager
	sched       *scheduler.Scheduler
	db          *storage.DB
	gitSvc      *gitmgr.GitService
	settingsSvc *settings.Service
	startTime   time.Time
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := int64(time.Since(h.startTime).Seconds())
	allocMB := float64(m.Alloc) / 1024 / 1024
	sysMB := float64(m.Sys) / 1024 / 1024

	var jobsCount int
	if h.sched != nil {
		jobsCount = len(h.sched.GetJobs())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": uptime,
		"memory_alloc_mb": fmt.Sprintf("%.2f MB", allocMB),
		"memory_sys_mb":   fmt.Sprintf("%.2f MB", sysMB),
		"active_cron_jobs": jobsCount,
		"timestamp":      time.Now().Format(time.RFC3339),
	})
}

func (h *APIHandler) handleListPackages(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	pkgs := h.mgr.ListPackages()
	if h.gitSvc != nil {
		for _, p := range pkgs {
			if p.IsGit {
				status, _ := h.gitSvc.GetStatus(p.Path)
				p.Status = status
			}
		}
	}
	writeJSON(w, http.StatusOK, pkgs)
}

func (h *APIHandler) handleGitClone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string `json:"url"`
		TargetName string `json:"target_name"`
		Branch     string `json:"branch"`
		Token      string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.URL == "" || req.TargetName == "" {
		writeError(w, http.StatusBadRequest, "url and target_name are required")
		return
	}

	targetPath := fmt.Sprintf("packages/%s", req.TargetName)
	if err := h.gitSvc.Clone(req.URL, targetPath, req.Branch, req.Token); err != nil {
		writeError(w, http.StatusInternalServerError, "clone failed: "+err.Error())
		return
	}

	h.mgr.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cloned"})
}

func (h *APIHandler) handleGitPull(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PackageName string `json:"package_name"`
		Token       string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pkg := h.mgr.GetPackage(req.PackageName)
	if pkg == nil {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	res, err := h.gitSvc.Pull(pkg.Path, req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pull failed: "+err.Error())
		return
	}

	h.mgr.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": res})
}

func (h *APIHandler) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PackageName string `json:"package_name"`
		Message     string `json:"message"`
		AuthorName  string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
		Token       string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pkg := h.mgr.GetPackage(req.PackageName)
	if pkg == nil {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	err := h.gitSvc.CommitAndPush(pkg.Path, req.Message, req.AuthorName, req.AuthorEmail, req.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "commit failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "committed"})
}

func (h *APIHandler) handleListFunctions(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, h.mgr.ListFunctions())
}

func (h *APIHandler) handleGetFunctionCode(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key query param is required")
		return
	}

	code, err := h.mgr.GetFunctionCode(key)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"key": key, "code": code})
}

func (h *APIHandler) handleSaveFunctionCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Package  string `json:"package"`
		Filename string `json:"filename"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.mgr.SaveFunctionCode(req.Package, req.Filename, req.Code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.sched != nil {
		h.sched.Reschedule()
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *APIHandler) handleRunFunction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key    string      `json:"key"`
		Params interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	start := time.Now()
	res, err := h.mgr.CallWithTrigger(ctx, req.Key, req.Params, "manual_test")
	duration := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":      "failed",
			"error":       err.Error(),
			"duration_ms": duration,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"output":      res,
		"duration_ms": duration,
	})
}

func (h *APIHandler) handleCron(w http.ResponseWriter, r *http.Request) {
	if h.sched == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	switch r.Method {
	case "GET":
		writeJSON(w, http.StatusOK, h.sched.GetJobs())
	case "POST":
		var req struct {
			Action string `json:"action"` // "toggle", "run"
			Target string `json:"target"`
			Enable bool   `json:"enable"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}

		if req.Action == "toggle" {
			h.sched.ToggleJob(req.Target, req.Enable)
			writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
			return
		}
		if req.Action == "run" {
			if err := h.sched.RunJobNow(req.Target); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "executed"})
			return
		}
		writeError(w, http.StatusBadRequest, "unknown action")
	}
}

func (h *APIHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"logs": []interface{}{}, "total": 0})
		return
	}

	funcName := r.URL.Query().Get("func")
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	logs, total, err := h.db.QueryLogs(funcName, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": total,
	})
}

func (h *APIHandler) handleSettings(w http.ResponseWriter, r *http.Request) {
	if h.settingsSvc == nil {
		writeError(w, http.StatusInternalServerError, "settings service not initialized")
		return
	}

	switch r.Method {
	case "GET":
		cfg, err := h.settingsSvc.Get()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case "POST":
		var cfg settings.AppSettings
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if err := h.settingsSvc.Save(&cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	}
}

func (h *APIHandler) handleEnv(w http.ResponseWriter, r *http.Request) {
	if h.settingsSvc == nil {
		writeError(w, http.StatusInternalServerError, "settings service not initialized")
		return
	}

	switch r.Method {
	case "GET":
		envMap, err := h.settingsSvc.GetEnv()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, envMap)
	case "POST":
		var envMap map[string]string
		if err := json.NewDecoder(r.Body).Decode(&envMap); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if err := h.settingsSvc.SaveEnv(envMap); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
	}
}

func (h *APIHandler) handleMCPTools(w http.ResponseWriter, r *http.Request) {
	if h.mgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"tools": []interface{}{}})
		return
	}

	funcs := h.mgr.ListFunctions()
	var mcpFuncs []*domain.FunctionMeta
	for _, fn := range funcs {
		if fn.IsMCP {
			mcpFuncs = append(mcpFuncs, fn)
		}
	}

	// Generate sample Claude Desktop config snippet
	claudeConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"actacron": map[string]interface{}{
				"command": "d:\\Personal_Sources\\ActaCron\\actacron.exe",
				"args":    []string{"mcp"},
			},
		},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tools":         mcpFuncs,
		"claude_config": claudeConfig,
	})
}
