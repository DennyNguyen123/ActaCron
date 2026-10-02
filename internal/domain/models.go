package domain

import "time"

type ParamSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type FunctionMeta struct {
	Name        string        `json:"name"`
	Package     string        `json:"package"`
	FilePath    string        `json:"file_path"`
	Description string        `json:"description"`
	CronExpr       string        `json:"cron_expr,omitempty"`
	CronStart      string        `json:"cron_start,omitempty"`
	CronEnd        string        `json:"cron_end,omitempty"`
	Timezone       string        `json:"timezone,omitempty"`
	RetryCount     int           `json:"retry_count,omitempty"`
	RetryDelay     string        `json:"retry_delay,omitempty"`
	MaxRuns        int           `json:"max_runs,omitempty"`
	RunCount       int           `json:"run_count,omitempty"`
	NoOverlap      bool          `json:"no_overlap"`
	IsMCP          bool          `json:"is_mcp"`
	AllowExec      bool          `json:"allow_exec"`
	Params         []ParamSchema `json:"params"`
	TimeoutSeconds int           `json:"timeout_seconds,omitempty"`
	IsEnabled      bool          `json:"is_enabled"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type WorkspaceConfig struct {
	Name           string `json:"name,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Description    string `json:"description,omitempty"`
}

type ExecutionLog struct {
	ID           int64     `json:"id"`
	ExecutionID  string    `json:"execution_id"`
	PackageName  string    `json:"package_name"`
	FunctionName string    `json:"function_name"`
	TriggerType  string    `json:"trigger_type"` // cron, mcp, manual, call
	Status       string    `json:"status"`       // success, failed, timeout
	InputParams  string    `json:"input_params"`
	OutputData   string    `json:"output_data"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ConsoleLogs  string    `json:"console_logs"`
	DurationMs   int64     `json:"duration_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type ExternalWorkspace struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type PackageInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	IsGit      bool      `json:"is_git"`
	IsExternal bool      `json:"is_external"`
	RemoteURL  string    `json:"remote_url,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	Status     string    `json:"status"` // clean, modified, behind, conflict
	Functions  []string  `json:"functions"`
	UpdatedAt  time.Time `json:"updated_at"`
}
