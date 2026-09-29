package api

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/mcp"
	"actacron/internal/scheduler"
	"actacron/internal/settings"
	"actacron/internal/storage"
)

func NewRouter(
	mgr *manager.Manager,
	sched *scheduler.Scheduler,
	db *storage.DB,
	gitSvc *gitmgr.GitService,
	settingsSvc *settings.Service,
	staticFS fs.FS,
) http.Handler {
	handler := &APIHandler{
		mgr:         mgr,
		sched:       sched,
		db:          db,
		gitSvc:      gitSvc,
		settingsSvc: settingsSvc,
		startTime:   time.Now(),
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/health", handler.handleHealth)
	mux.HandleFunc("/api/packages", handler.handleListPackages)
	mux.HandleFunc("/api/packages/clone", handler.handleGitClone)
	mux.HandleFunc("/api/packages/pull", handler.handleGitPull)
	mux.HandleFunc("/api/packages/sync", handler.handleGitSyncAll)
	mux.HandleFunc("/api/packages/commit", handler.handleGitCommit)

	mux.HandleFunc("/api/functions", handler.handleListFunctions)
	mux.HandleFunc("/api/functions/code", handler.handleGetFunctionCode)
	mux.HandleFunc("/api/functions/save", handler.handleSaveFunctionCode)
	mux.HandleFunc("/api/run", handler.handleRunFunction)

	mux.HandleFunc("/api/cron", handler.handleCron)
	mux.HandleFunc("/api/logs", handler.handleLogs)
	mux.HandleFunc("/api/settings", handler.handleSettings)
	mux.HandleFunc("/api/env", handler.handleEnv)
	mux.HandleFunc("/api/mcp/tools", handler.handleMCPTools)

	// MCP SSE endpoints
	if mgr != nil {
		sseHandler := mcp.NewSSEHandler(mgr)
		mux.Handle("/mcp/", sseHandler)
	}

	// Static UI assets
	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/mcp/") {
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
