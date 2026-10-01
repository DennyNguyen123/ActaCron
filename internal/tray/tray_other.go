//go:build !windows

package tray

import (
	"os"
	"os/signal"
	"syscall"

	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
)

// TrayHandler manages the headless lifecycle on non-Windows environments.
type TrayHandler struct {
	dashboardURL string
	sched        *scheduler.Scheduler
	gitSvc       *gitmgr.GitService
	mgr          *manager.Manager
	onExit       func()
}

// New creates a new TrayHandler instance.
func New(dashboardURL string, sched *scheduler.Scheduler, gitSvc *gitmgr.GitService, mgr *manager.Manager, onExit func()) *TrayHandler {
	return &TrayHandler{
		dashboardURL: dashboardURL,
		sched:        sched,
		gitSvc:       gitSvc,
		mgr:          mgr,
		onExit:       onExit,
	}
}

// Run blocks until OS interrupt or SIGTERM signal on non-Windows headless environments.
func (th *TrayHandler) Run() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	if th.onExit != nil {
		th.onExit()
	}
}
