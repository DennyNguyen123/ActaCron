package settings

import (
	"os"
	"strconv"

	"actacron/internal/storage"
)

type AppSettings struct {
	Port             int    `json:"port"`
	TimeoutSeconds   int    `json:"timeout_seconds"`
	LogRetentionDays int    `json:"log_retention_days"`
	StartWithWindows bool   `json:"start_with_windows"`
	AllowShellExec   bool   `json:"allow_shell_exec"`
	WindowMode       string `json:"window_mode"` // "edge_app", "browser", "none"
	GitAuthorName    string `json:"git_author_name"`
	GitAuthorEmail   string `json:"git_author_email"`
	GitDefaultToken  string `json:"git_default_token"`
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
		return cfg, nil
	}

	allSettings, err := s.db.GetAllSettings()
	if err != nil {
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

	return cfg, nil
}

func (s *Service) Save(cfg *AppSettings) error {
	if s.db == nil {
		return nil
	}

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

func (s *Service) GetEnv() (map[string]string, error) {
	return ReadEnv(s.envPath)
}

func (s *Service) SaveEnv(envMap map[string]string) error {
	for k, v := range envMap {
		os.Setenv(k, v)
	}
	return WriteEnv(s.envPath, envMap)
}
