package settings

import (
	"os"
	"strconv"
	"strings"

	"actacron/internal/storage"
)

type AppSettings struct {
	Port             int    `json:"port"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	LogRetentionDays int    `json:"log_retention_days"`
	RetentionDays    int    `json:"retention_days"` // alias for frontend compatibility
	StartWithWindows bool   `json:"start_with_windows"`
	RunOnStartup     bool   `json:"run_on_startup"` // alias for frontend compatibility
	AllowShellExec   bool   `json:"allow_shell_exec"`
	AllowShell       bool   `json:"allow_shell"` // alias for frontend compatibility
	WindowMode       string `json:"window_mode"` // "edge_app", "browser", "none"
	GitAuthorName    string `json:"git_author_name"`
	GitAuthorEmail   string `json:"git_author_email"`
	GitDefaultToken  string `json:"git_default_token"`
	GitToken         string `json:"git_token"` // alias for frontend compatibility
	NotifyOnFailure  bool   `json:"notify_on_failure"`
	ThemeAccent      string `json:"theme_accent"`   // default: #22C55E
	Density          string `json:"density"`        // compact, balanced, spacious
	EditorFontSize   int    `json:"editor_font_size"` // 12, 14, 16
	Language         string `json:"language"`         // en (default), vi
}

type Service struct {
	db      *storage.DB
	envPath string
}

func New(db *storage.DB, envPath string) *Service {
	_ = LoadEnv(envPath)
	return &Service{
		db:      db,
		envPath: envPath,
	}
}

func (s *Service) Get() (*AppSettings, error) {
	// Defaults
	cfg := &AppSettings{
		Port:             8080,
		TimeoutSeconds:   30,
		LogRetentionDays: 7,
		StartWithWindows: false,
		AllowShellExec:   false,
		WindowMode:       "edge_app",
		ThemeAccent:      "#22C55E",
		Density:          "balanced",
		EditorFontSize:   14,
		Language:         "en",
	}

	if s.db == nil {
		cfg.syncAliases()
		return cfg, nil
	}

	allSettings, err := s.db.GetAllSettings()
	if err != nil {
		cfg.syncAliases()
		return cfg, err
	}

	if val, ok := allSettings["port"]; ok {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			cfg.Port = p
		}
	}
	if val, ok := allSettings["timeout_seconds"]; ok {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			cfg.TimeoutSeconds = t
		}
	}
	if val, ok := allSettings["log_retention_days"]; ok {
		if r, err := strconv.Atoi(val); err == nil && r > 0 {
			cfg.LogRetentionDays = r
		}
	}
	if val, ok := allSettings["start_with_windows"]; ok {
		cfg.StartWithWindows = val == "true"
	} else if IsAutoStartEnabled("ActaCron") {
		cfg.StartWithWindows = true
	}
	if val, ok := allSettings["allow_shell_exec"]; ok {
		cfg.AllowShellExec = val == "true"
	}
	if val, ok := allSettings["window_mode"]; ok && val != "" {
		cfg.WindowMode = val
	}
	if val, ok := allSettings["git_author_name"]; ok {
		cfg.GitAuthorName = val
	}
	if val, ok := allSettings["git_author_email"]; ok {
		cfg.GitAuthorEmail = val
	}
	if val, ok := allSettings["git_default_token"]; ok {
		cfg.GitDefaultToken = val
	}
	if val, ok := allSettings["notify_on_failure"]; ok {
		cfg.NotifyOnFailure = val == "true"
	}
	if val, ok := allSettings["theme_accent"]; ok && val != "" {
		cfg.ThemeAccent = val
	}
	if val, ok := allSettings["density"]; ok && val != "" {
		cfg.Density = val
	}
	if val, ok := allSettings["editor_font_size"]; ok {
		if s, err := strconv.Atoi(val); err == nil && s > 0 {
			cfg.EditorFontSize = s
		}
	}
	if val, ok := allSettings["language"]; ok && val != "" {
		cfg.Language = val
	}

	cfg.syncAliases()
	return cfg, nil
}

func (cfg *AppSettings) syncAliases() {
	if cfg.RetentionDays > 0 && cfg.LogRetentionDays == 0 {
		cfg.LogRetentionDays = cfg.RetentionDays
	} else {
		cfg.RetentionDays = cfg.LogRetentionDays
	}

	if cfg.RunOnStartup && !cfg.StartWithWindows {
		cfg.StartWithWindows = true
	}
	cfg.RunOnStartup = cfg.StartWithWindows

	if cfg.AllowShell && !cfg.AllowShellExec {
		cfg.AllowShellExec = true
	}
	cfg.AllowShell = cfg.AllowShellExec

	if cfg.GitToken != "" && cfg.GitDefaultToken == "" {
		cfg.GitDefaultToken = cfg.GitToken
	}
	cfg.GitToken = cfg.GitDefaultToken
}

func (s *Service) Save(cfg *AppSettings) error {
	if s.db == nil {
		return nil
	}

	cfg.syncAliases()

	pairs := map[string]string{
		"port":               strconv.Itoa(cfg.Port),
		"timeout_seconds":    strconv.Itoa(cfg.TimeoutSeconds),
		"log_retention_days": strconv.Itoa(cfg.LogRetentionDays),
		"start_with_windows": strconv.FormatBool(cfg.StartWithWindows),
		"allow_shell_exec":   strconv.FormatBool(cfg.AllowShellExec),
		"window_mode":        cfg.WindowMode,
		"git_author_name":    cfg.GitAuthorName,
		"git_author_email":   cfg.GitAuthorEmail,
		"git_default_token":  cfg.GitDefaultToken,
		"notify_on_failure":  strconv.FormatBool(cfg.NotifyOnFailure),
		"theme_accent":       cfg.ThemeAccent,
		"density":            cfg.Density,
		"editor_font_size":   strconv.Itoa(cfg.EditorFontSize),
		"language":           cfg.Language,
	}

	for k, v := range pairs {
		if err := s.db.SetSetting(k, v); err != nil {
			return err
		}
	}

	// Windows autostart update if running binary exists
	if exe, err := os.Executable(); err == nil {
		_ = SetAutoStart("ActaCron", exe, cfg.StartWithWindows)
	}

	return nil
}

func (s *Service) SaveMap(updates map[string]interface{}) (*AppSettings, error) {
	if s.db == nil {
		return s.Get()
	}

	var autostartChanged bool
	var autostartEnabled bool

	for k, v := range updates {
		switch k {
		case "port":
			if p, ok := toInt(v); ok && p > 0 {
				_ = s.db.SetSetting("port", strconv.Itoa(p))
			}
		case "timeout_seconds":
			if t, ok := toInt(v); ok && t > 0 {
				_ = s.db.SetSetting("timeout_seconds", strconv.Itoa(t))
			}
		case "log_retention_days", "retention_days":
			if r, ok := toInt(v); ok && r > 0 {
				_ = s.db.SetSetting("log_retention_days", strconv.Itoa(r))
			}
		case "start_with_windows", "run_on_startup":
			if b, ok := toBool(v); ok {
				_ = s.db.SetSetting("start_with_windows", strconv.FormatBool(b))
				autostartChanged = true
				autostartEnabled = b
			}
		case "allow_shell_exec", "allow_shell":
			if b, ok := toBool(v); ok {
				_ = s.db.SetSetting("allow_shell_exec", strconv.FormatBool(b))
			}
		case "window_mode":
			if str, ok := v.(string); ok && str != "" {
				_ = s.db.SetSetting("window_mode", str)
			}
		case "git_author_name":
			if str, ok := v.(string); ok {
				_ = s.db.SetSetting("git_author_name", str)
			}
		case "git_author_email":
			if str, ok := v.(string); ok {
				_ = s.db.SetSetting("git_author_email", str)
			}
		case "git_default_token", "git_token":
			if str, ok := v.(string); ok {
				_ = s.db.SetSetting("git_default_token", str)
			}
		case "notify_on_failure":
			if b, ok := toBool(v); ok {
				_ = s.db.SetSetting("notify_on_failure", strconv.FormatBool(b))
			}
		case "theme_accent":
			if str, ok := v.(string); ok && str != "" {
				_ = s.db.SetSetting("theme_accent", str)
			}
		case "density":
			if str, ok := v.(string); ok && str != "" {
				_ = s.db.SetSetting("density", str)
			}
		case "editor_font_size":
			if sz, ok := toInt(v); ok && sz > 0 {
				_ = s.db.SetSetting("editor_font_size", strconv.Itoa(sz))
			}
		case "language":
			if str, ok := v.(string); ok && str != "" {
				_ = s.db.SetSetting("language", str)
			}
		}
	}

	if autostartChanged {
		if exe, err := os.Executable(); err == nil {
			_ = SetAutoStart("ActaCron", exe, autostartEnabled)
		}
	}

	return s.Get()
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case string:
		trimmed := strings.TrimSpace(strings.TrimSuffix(val, "px"))
		if n, err := strconv.Atoi(trimmed); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return int(f), true
		}
	}
	return 0, false
}

func toBool(v interface{}) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(val))
		return b, err == nil
	}
	return false, false
}

func (s *Service) GetEnv() (map[string]string, error) {
	return ReadEnv(s.envPath)
}

func (s *Service) SaveEnv(envMap map[string]string) error {
	for k, v := range envMap {
		os.Setenv(k, v)
	}
	return WriteEnv(s.envPath, envMap)
}
