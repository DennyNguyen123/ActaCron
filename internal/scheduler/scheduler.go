package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"actacron/internal/manager"
	"actacron/internal/storage"
	"github.com/robfig/cron/v3"
)

type CronJobInfo struct {
	EntryID      int       `json:"entry_id"`
	PackageName  string    `json:"package_name"`
	FunctionName string    `json:"function_name"`
	CronExpr     string    `json:"cron_expr"`
	IsEnabled    bool      `json:"is_enabled"`
	NextRun      time.Time `json:"next_run"`
	PrevRun      time.Time `json:"prev_run"`
}

type Scheduler struct {
	mgr       *manager.Manager
	db        *storage.DB
	cron      *cron.Cron
	entryKeys map[cron.EntryID]string // entryID -> "pkg/func"
	mu        sync.RWMutex
	running   bool
}

func New(mgr *manager.Manager, db *storage.DB) *Scheduler {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	c := cron.New(cron.WithParser(parser))

	s := &Scheduler{
		mgr:       mgr,
		db:        db,
		cron:      c,
		entryKeys: make(map[cron.EntryID]string),
	}
	s.Reschedule()
	return s
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		s.cron.Start()
		s.running = true
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.cron.Stop()
		s.running = false
	}
}

func (s *Scheduler) Reschedule() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear current entries
	for _, entry := range s.cron.Entries() {
		s.cron.Remove(entry.ID)
	}
	s.entryKeys = make(map[cron.EntryID]string)

	funcs := s.mgr.ListFunctions()
	for _, fn := range funcs {
		if fn.CronExpr == "" || !fn.IsEnabled {
			continue
		}

		fullKey := fn.Package + "/" + fn.Name
		targetKey := fullKey

		entryID, err := s.cron.AddFunc(fn.CronExpr, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			s.mgr.CallWithTrigger(ctx, targetKey, nil, "cron")
		})

		if err == nil {
			s.entryKeys[entryID] = targetKey
		}
	}
}

func (s *Scheduler) GetJobs() []CronJobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobs []CronJobInfo
	entries := s.cron.Entries()
	entryMap := make(map[string]cron.Entry)
	for _, e := range entries {
		if k, ok := s.entryKeys[e.ID]; ok {
			entryMap[k] = e
		}
	}

	funcs := s.mgr.ListFunctions()
	for _, fn := range funcs {
		if fn.CronExpr == "" {
			continue
		}

		fullKey := fn.Package + "/" + fn.Name
		job := CronJobInfo{
			PackageName:  fn.Package,
			FunctionName: fn.Name,
			CronExpr:     fn.CronExpr,
			IsEnabled:    fn.IsEnabled,
		}

		if e, exists := entryMap[fullKey]; exists {
			job.EntryID = int(e.ID)
			job.NextRun = e.Next
			job.PrevRun = e.Prev
		}

		jobs = append(jobs, job)
	}
	return jobs
}

func (s *Scheduler) RunJobNow(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := s.mgr.CallWithTrigger(ctx, target, nil, "cron_manual")
	return err
}

func (s *Scheduler) ToggleJob(target string, enabled bool) error {
	fn := s.mgr.GetFunction(target)
	if fn == nil {
		return fmt.Errorf("function '%s' not found", target)
	}

	fn.IsEnabled = enabled
	if s.db != nil {
		s.db.SetFunctionState(fn.Package, fn.Name, enabled, "")
	}

	s.Reschedule()
	return nil
}

func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}
