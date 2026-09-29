package tray

import (
	"log"

	"github.com/getlantern/systray"

	"actacron/internal/gitmgr"
	"actacron/internal/manager"
	"actacron/internal/scheduler"
)

// TrayHandler manages the Windows System Tray menu and event loop.
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

// Run starts the systray main loop. This blocks until systray exits.
func (th *TrayHandler) Run() {
	systray.Run(th.onReady, th.onExitInternal)
}

func (th *TrayHandler) onReady() {
	systray.SetIcon(generateDefaultIcon())
	systray.SetTitle("ActaCron")
	systray.SetTooltip("ActaCron - Dynamic JS Engine & Cron Hub")

	mOpen := systray.AddMenuItem("Open Dashboard", "Open ActaCron Edge Web UI")
	mSync := systray.AddMenuItem("Sync All Git", "Pull and sync all Git packages")

	var mPause *systray.MenuItem
	if th.sched != nil {
		mPause = systray.AddMenuItem("Pause Cron", "Pause/Resume cron schedules")
	}

	systray.AddSeparator()
	mExit := systray.AddMenuItem("Exit ActaCron", "Shut down background daemon")

	cronPaused := false

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				if err := OpenDashboard(th.dashboardURL); err != nil {
					log.Printf("[Tray] Failed to open dashboard: %v", err)
				}

			case <-mSync.ClickedCh:
				log.Println("[Tray] Syncing all packages...")
				if th.mgr != nil && th.gitSvc != nil {
					for _, pkg := range th.mgr.ListPackages() {
						if _, err := th.gitSvc.Pull(pkg.Path, ""); err != nil {
							log.Printf("[Tray] Failed sync for %s: %v", pkg.Name, err)
						}
					}
					th.mgr.Reload()
				}

			case <-mExit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	if mPause != nil {
		go func() {
			for range mPause.ClickedCh {
				cronPaused = !cronPaused
				if cronPaused {
					th.sched.Stop()
					mPause.SetTitle("Resume Cron")
					systray.SetTooltip("ActaCron (Cron Paused)")
				} else {
					th.sched.Start()
					mPause.SetTitle("Pause Cron")
					systray.SetTooltip("ActaCron - Dynamic JS Engine & Cron Hub")
				}
			}
		}()
	}
}

func (th *TrayHandler) onExitInternal() {
	if th.onExit != nil {
		th.onExit()
	}
}
