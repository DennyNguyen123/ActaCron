package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"actacron/internal/domain"
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
	Status       string    `json:"status"` // "active", "paused", "pending", "expired", "completed", "running"
	Timezone     string    `json:"timezone,omitempty"`
	CronStart    string    `json:"cron_start,omitempty"`
	CronEnd      string    `json:"cron_end,omitempty"`
	MaxRuns      int       `json:"max_runs,omitempty"`
	RunCount     int       `json:"run_count,omitempty"`
	IsRunning    bool      `json:"is_running"`
}

type Scheduler struct {
	mgr         *manager.Manager
	db          *storage.DB
	cron        *cron.Cron
	parser      cron.Parser
	entryKeys   map[cron.EntryID]string // entryID -> "pkg/func"
	runningJobs map[string]int          // "pkg/func" -> active execution count
	jobsMu      sync.RWMutex
	mu          sync.RWMutex
	running     bool
}

func New(mgr *manager.Manager, db *storage.DB) *Scheduler {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	c := cron.New(cron.WithParser(parser))

	s := &Scheduler{
		mgr:         mgr,
		db:          db,
		cron:        c,
		parser:      parser,
		entryKeys:   make(map[cron.EntryID]string),
		runningJobs: make(map[string]int),
	}
	s.Reschedule()
	return s
}

func getTimeLocation(tz string) *time.Location {
	if tz == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("[Scheduler] Warning: unrecognized timezone '%s', falling back to local time", tz)
		return time.Local
	}
	return loc
}

func parseBoundaryTime(str string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	str = strings.TrimSpace(str)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, str, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", str)
}

func (s *Scheduler) computeStatus(fn *domain.FunctionMeta, isRunning bool) string {
	if isRunning {
		return "running"
	}
	loc := getTimeLocation(fn.Timezone)
	now := time.Now().In(loc)

	if fn.CronEnd != "" {
		if end, err := parseBoundaryTime(fn.CronEnd, loc); err == nil && now.After(end) {
			return "expired"
		}
	}
	if fn.MaxRuns > 0 && fn.RunCount >= fn.MaxRuns {
		return "completed"
	}
	if !fn.IsEnabled {
		return "paused"
	}
	if fn.CronStart != "" {
		if start, err := parseBoundaryTime(fn.CronStart, loc); err == nil && now.Before(start) {
			return "pending"
		}
	}
	return "active"
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

		pkgName := fn.Package
		funcName := fn.Name

		loc := getTimeLocation(fn.Timezone)

		parsedSchedule, err := s.parser.Parse(fn.CronExpr)
		if err != nil {
			log.Printf("[Scheduler] Error parsing cron '%s' for %s: %v", fn.CronExpr, fullKey, err)
			continue
		}

		if specSched, ok := parsedSchedule.(*cron.SpecSchedule); ok {
			specSched.Location = loc
		}

		jobCallback := func() {
			s.executeScheduledJob(targetKey, pkgName, funcName)
		}

		entryID := s.cron.Schedule(parsedSchedule, cron.FuncJob(jobCallback))
		s.entryKeys[entryID] = targetKey
	}
}

func (s *Scheduler) executeScheduledJob(targetKey, pkgName, funcName string) {
	fn := s.mgr.GetFunction(targetKey)
	if fn == nil {
		return
	}

	// 1. Check if !fn.IsEnabled: skip.
	if !fn.IsEnabled {
		return
	}

	loc := getTimeLocation(fn.Timezone)
	now := time.Now().In(loc)

	// 2. Check if fn.CronStart != "": parse time. If now < start, skip.
	if fn.CronStart != "" {
		start, err := parseBoundaryTime(fn.CronStart, loc)
		if err == nil {
			if now.Before(start) {
				return
			}
		} else {
			log.Printf("[Scheduler] Warning: invalid cron_start '%s' in %s, ignoring", fn.CronStart, targetKey)
		}
	}

	// 3. Check if fn.CronEnd != "": parse time. If now > end, disable fn.IsEnabled = false and skip.
	if fn.CronEnd != "" {
		end, err := parseBoundaryTime(fn.CronEnd, loc)
		if err == nil {
			if now.After(end) {
				fn.IsEnabled = false
				if s.db != nil {
					_ = s.db.SetFunctionEnabled(fn.Package, fn.Name, false)
				}
				go s.Reschedule()
				return
			}
		} else {
			log.Printf("[Scheduler] Warning: invalid cron_end '%s' in %s, ignoring", fn.CronEnd, targetKey)
		}
	}

	// 4. Check if fn.MaxRuns > 0 && fn.RunCount >= fn.MaxRuns: disable fn.IsEnabled = false and skip.
	if fn.MaxRuns > 0 && fn.RunCount >= fn.MaxRuns {
		fn.IsEnabled = false
		if s.db != nil {
			_ = s.db.SetFunctionEnabled(fn.Package, fn.Name, false)
		}
		go s.Reschedule()
		return
	}

	// 5. Overlap Guard: if fn.NoOverlap && runningJobs[targetKey] > 0: insert log with status "skipped_overlap", skip.
	s.jobsMu.Lock()
	if fn.NoOverlap && s.runningJobs[targetKey] > 0 {
		s.jobsMu.Unlock()
		log.Printf("[Scheduler] Overlap detected for %s. Previous execution still active. Skipping run.", targetKey)
		if s.db != nil {
			_ = s.db.InsertLog(&domain.ExecutionLog{
				ExecutionID:  fmt.Sprintf("cron-skip-%d", time.Now().UnixNano()),
				PackageName:  fn.Package,
				FunctionName: fn.Name,
				TriggerType:  "cron",
				Status:       "skipped_overlap",
				DurationMs:   0,
				CreatedAt:    time.Now(),
			})
		}
		return
	}
	s.runningJobs[targetKey]++
	s.jobsMu.Unlock()

	defer func() {
		s.jobsMu.Lock()
		s.runningJobs[targetKey]--
		if s.runningJobs[targetKey] <= 0 {
			delete(s.runningJobs, targetKey)
		}
		s.jobsMu.Unlock()
	}()

	// 6. Run call: if error and fn.RetryCount > 0, retry with backoff.
	retryDelay := 5 * time.Second
	if fn.RetryDelay != "" {
		if d, err := time.ParseDuration(fn.RetryDelay); err == nil && d > 0 {
			retryDelay = d
		}
	}

	runOnce := func() (interface{}, error) {
		timeout := s.mgr.GetEffectiveTimeout(pkgName, funcName)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return s.mgr.CallWithTrigger(ctx, targetKey, nil, "cron")
	}

	_, runErr := runOnce()
	if runErr != nil && fn.RetryCount > 0 {
		for attempt := 1; attempt <= fn.RetryCount; attempt++ {
			log.Printf("[Scheduler] Retry attempt %d/%d for %s after %v", attempt, fn.RetryCount, targetKey, retryDelay)
			time.Sleep(retryDelay)
			_, runErr = runOnce()
			if runErr == nil {
				break
			}
		}
	}

	// 7. On success: call db.IncrementRunCount(fn.Package, fn.Name).
	if runErr == nil {
		fn.RunCount++
		if s.db != nil {
			_ = s.db.IncrementRunCount(fn.Package, fn.Name)
		}
		if fn.MaxRuns > 0 && fn.RunCount >= fn.MaxRuns {
			fn.IsEnabled = false
			if s.db != nil {
				_ = s.db.SetFunctionEnabled(fn.Package, fn.Name, false)
			}
			go s.Reschedule()
		}
	}
}

func (s *Scheduler) GetJobs() []CronJobInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.jobsMu.RLock()
	runningSnapshot := make(map[string]bool, len(s.runningJobs))
	for k, v := range s.runningJobs {
		runningSnapshot[k] = v > 0
	}
	s.jobsMu.RUnlock()

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
		isRunning := runningSnapshot[fullKey]
		status := s.computeStatus(fn, isRunning)

		job := CronJobInfo{
			PackageName:  fn.Package,
			FunctionName: fn.Name,
			CronExpr:     fn.CronExpr,
			IsEnabled:    fn.IsEnabled,
			Status:       status,
			Timezone:     fn.Timezone,
			CronStart:    fn.CronStart,
			CronEnd:      fn.CronEnd,
			MaxRuns:      fn.MaxRuns,
			RunCount:     fn.RunCount,
			IsRunning:    isRunning,
		}

		if e, exists := entryMap[fullKey]; exists {
			job.EntryID = int(e.ID)
			job.PrevRun = e.Prev
			if status == "paused" || status == "expired" || status == "completed" {
				job.NextRun = time.Time{}
			} else if status == "pending" && fn.CronStart != "" {
				loc := getTimeLocation(fn.Timezone)
				now := time.Now().In(loc)
				if start, err := parseBoundaryTime(fn.CronStart, loc); err == nil && now.Before(start) {
					if sched, err := s.parser.Parse(fn.CronExpr); err == nil {
						job.NextRun = sched.Next(start)
					} else {
						job.NextRun = e.Next
					}
				} else {
					job.NextRun = e.Next
				}
			} else {
				job.NextRun = e.Next
			}
		}

		jobs = append(jobs, job)
	}
	return jobs
}

func (s *Scheduler) RunJobNow(target string) error {
	fn := s.mgr.GetFunction(target)
	targetKey := target
	if fn != nil {
		targetKey = fn.Package + "/" + fn.Name
	}

	s.jobsMu.Lock()
	if fn != nil && fn.NoOverlap && s.runningJobs[targetKey] > 0 {
		s.jobsMu.Unlock()
		log.Printf("[Scheduler] Overlap detected for manual run %s. Previous execution still active. Skipping run.", targetKey)
		if s.db != nil {
			_ = s.db.InsertLog(&domain.ExecutionLog{
				ExecutionID:  fmt.Sprintf("cron-manual-skip-%d", time.Now().UnixNano()),
				PackageName:  fn.Package,
				FunctionName: fn.Name,
				TriggerType:  "cron_manual",
				Status:       "skipped_overlap",
				DurationMs:   0,
				CreatedAt:    time.Now(),
			})
		}
		return fmt.Errorf("job '%s' is already running", target)
	}
	s.runningJobs[targetKey]++
	s.jobsMu.Unlock()

	defer func() {
		s.jobsMu.Lock()
		s.runningJobs[targetKey]--
		if s.runningJobs[targetKey] <= 0 {
			delete(s.runningJobs, targetKey)
		}
		s.jobsMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := s.mgr.CallWithTrigger(ctx, targetKey, nil, "cron_manual")
	return err
}

func (s *Scheduler) ResetRuns(target string) error {
	fn := s.mgr.GetFunction(target)
	if fn == nil {
		return fmt.Errorf("function '%s' not found", target)
	}
	fn.RunCount = 0
	if s.db != nil {
		if err := s.db.ResetRunCount(fn.Package, fn.Name); err != nil {
			return err
		}
	}
	s.Reschedule()
	return nil
}

func (s *Scheduler) ToggleJob(target string, enabled bool) error {
	fn := s.mgr.GetFunction(target)
	if fn == nil {
		return fmt.Errorf("function '%s' not found", target)
	}

	fn.IsEnabled = enabled
	if s.db != nil {
		_ = s.db.SetFunctionEnabled(fn.Package, fn.Name, enabled)
	}

	s.Reschedule()
	return nil
}

func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Scheduler) IsJobRunning(target string) bool {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	return s.runningJobs[target] > 0
}
