package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"actacron/internal/api"
	"actacron/internal/engine"
	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/mcp"
	"actacron/internal/scheduler"
	"actacron/internal/settings"
	"actacron/internal/storage"
	"actacron/internal/tray"
	"actacron/internal/version"
	"actacron/web"
)

func main() {
	log.Printf("Starting ActaCron %s (%s, %s)", version.Version, version.GitCommit, version.BuildDate)

	// Base directories: if running under 'go run' (temp go-build), use current working directory
	cwd, _ := os.Getwd()
	execDir, err := os.Executable()
	var appDir string
	if err == nil && !strings.Contains(execDir, "go-build") {
		appDir = filepath.Dir(execDir)
		// Silently cleanup any .old binary leftover from self-update
		_ = os.Remove(execDir + ".old")
	} else {
		appDir = cwd
	}

	packagesDir := filepath.Join(appDir, "packages")
	_ = os.MkdirAll(packagesDir, 0755)

	dbPath := filepath.Join(appDir, "actacron.db")
	envPath := filepath.Join(appDir, ".env")

	// 1. Initialize SQLite Database
	db, err := storage.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 2. Initialize Settings & Env
	settingsSvc := settings.New(db, envPath)
	cfg, err := settingsSvc.Get()
	if err != nil {
		log.Printf("Warning: failed to load settings: %v", err)
	}

	// 3. Initialize JS Engine & Package Manager
	runner := engine.New(db, cfg.TimeoutSeconds, cfg.AllowShellExec)
	mgr := manager.New(packagesDir, runner, db)

	if err := mgr.Reload(); err != nil {
		log.Printf("Warning: initial package load error: %v", err)
	}

	// Subcommand: actacron mcp (StdIO MCP Server for AI Agents)
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		// Pure StdIO mode: No log output to os.Stdout
		log.SetOutput(os.Stderr)
		if err := mcp.RunStdio(mgr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Subcommand: actacron headless (Daemon without GUI tray)
	isHeadless := len(os.Args) > 1 && os.Args[1] == "headless"

	// 4. Initialize Scheduler & Git Service
	sched := scheduler.New(mgr, db)
	sched.Start()
	defer sched.Stop()

	gitSvc := gitmgr.New()

	// 5. Initialize API & Embedded Web Dashboard
	router := api.NewRouter(mgr, sched, db, gitSvc, settingsSvc, web.Assets)

	port := cfg.Port
	if port <= 0 {
		port = 8080
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	go func() {
		log.Printf("ActaCron HTTP Server listening on http://127.0.0.1:%d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	dashboardURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Clean shutdown helper
	shutdownFunc := func() {
		log.Println("Shutting down ActaCron...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		sched.Stop()
		_ = db.Close()
		os.Exit(0)
	}

	if isHeadless {
		// Wait for OS interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		shutdownFunc()
		return
	}

	// 6. Windows System Tray Mode (Default)
	trayHandler := tray.New(dashboardURL, sched, gitSvc, mgr, shutdownFunc)

	// Open dashboard automatically on first start
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = tray.OpenDashboard(dashboardURL)
	}()

	// Blocks on Windows tray message loop
	trayHandler.Run()
}
