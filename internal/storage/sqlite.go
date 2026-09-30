package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"actacron/internal/domain"
	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

func New(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Optimize for concurrency
	db.SetMaxOpenConns(1) // SQLite single-writer safety

	s := &DB{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return s, nil
}

func (s *DB) Close() error {
	return s.db.Close()
}

func (s *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS execution_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		execution_id TEXT NOT NULL,
		package_name TEXT NOT NULL,
		function_name TEXT NOT NULL,
		trigger_type TEXT NOT NULL,
		status TEXT NOT NULL,
		input_params TEXT,
		output_data TEXT,
		error_message TEXT,
		console_logs TEXT,
		duration_ms INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_logs_func ON execution_logs(function_name, created_at);
	CREATE INDEX IF NOT EXISTS idx_logs_status ON execution_logs(status);
	CREATE INDEX IF NOT EXISTS idx_logs_created ON execution_logs(created_at);

	CREATE TABLE IF NOT EXISTS function_state (
		package_name TEXT NOT NULL,
		function_name TEXT NOT NULL,
		is_enabled BOOLEAN DEFAULT 1,
		last_run_at DATETIME,
		last_status TEXT,
		run_count INTEGER DEFAULT 0,
		cron_start TEXT,
		cron_end TEXT,
		timezone TEXT,
		max_runs INTEGER DEFAULT 0,
		PRIMARY KEY (package_name, function_name)
	);

	CREATE TABLE IF NOT EXISTS key_value_store (
		package_name TEXT NOT NULL,
		store_key TEXT NOT NULL,
		store_value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (package_name, store_key)
	);

	CREATE TABLE IF NOT EXISTS app_settings (
		setting_key TEXT PRIMARY KEY,
		setting_value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Safe migration: check for missing columns in existing function_state table
	cols := make(map[string]bool)
	rows, err := s.db.Query("PRAGMA table_info(function_state)")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dflt sql.NullString
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err == nil {
				cols[name] = true
			}
		}
	}

	newCols := []struct {
		name string
		def  string
	}{
		{"run_count", "INTEGER DEFAULT 0"},
		{"cron_start", "TEXT"},
		{"cron_end", "TEXT"},
		{"timezone", "TEXT"},
		{"max_runs", "INTEGER DEFAULT 0"},
	}
	for _, col := range newCols {
		if !cols[col.name] {
			_, _ = s.db.Exec(fmt.Sprintf("ALTER TABLE function_state ADD COLUMN %s %s", col.name, col.def))
		}
	}

	return nil
}

func (s *DB) InsertLog(log *domain.ExecutionLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO execution_logs (
		execution_id, package_name, function_name, trigger_type, status,
		input_params, output_data, error_message, console_logs, duration_ms, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	createdAt := log.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	res, err := s.db.Exec(query,
		log.ExecutionID, log.PackageName, log.FunctionName, log.TriggerType, log.Status,
		log.InputParams, log.OutputData, log.ErrorMessage, log.ConsoleLogs, log.DurationMs, createdAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		log.ID = id
	}
	return nil
}

func (s *DB) QueryLogs(funcName, status string, limit, offset int) ([]domain.ExecutionLog, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if funcName != "" {
		whereClause += " AND function_name = ?"
		args = append(args, funcName)
	}
	if status != "" {
		whereClause += " AND status = ?"
		args = append(args, status)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM execution_logs %s", whereClause)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
	SELECT id, execution_id, package_name, function_name, trigger_type, status,
	       COALESCE(input_params, ''), COALESCE(output_data, ''), COALESCE(error_message, ''),
	       COALESCE(console_logs, ''), duration_ms, created_at
	FROM execution_logs %s
	ORDER BY id DESC LIMIT ? OFFSET ?`, whereClause)

	queryArgs := append(args, limit, offset)
	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []domain.ExecutionLog
	for rows.Next() {
		var l domain.ExecutionLog
		var createdAt string
		err := rows.Scan(
			&l.ID, &l.ExecutionID, &l.PackageName, &l.FunctionName, &l.TriggerType, &l.Status,
			&l.InputParams, &l.OutputData, &l.ErrorMessage, &l.ConsoleLogs, &l.DurationMs, &createdAt,
		)
		if err != nil {
			return nil, 0, err
		}
		parsedTime, _ := time.Parse("2006-01-02 15:04:05", createdAt)
		if parsedTime.IsZero() {
			parsedTime, _ = time.Parse(time.RFC3339, createdAt)
		}
		l.CreatedAt = parsedTime
		logs = append(logs, l)
	}
	return logs, total, nil
}

func (s *DB) SetKV(pkg, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO key_value_store (package_name, store_key, store_value, updated_at)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(package_name, store_key) DO UPDATE SET
		store_value = excluded.store_value,
		updated_at = CURRENT_TIMESTAMP`

	_, err := s.db.Exec(query, pkg, key, value)
	return err
}

func (s *DB) GetKV(pkg, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT store_value FROM key_value_store WHERE package_name = ? AND store_key = ?`
	var val string
	err := s.db.QueryRow(query, pkg, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (s *DB) DeleteKV(pkg, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM key_value_store WHERE package_name = ? AND store_key = ?`
	_, err := s.db.Exec(query, pkg, key)
	return err
}

func (s *DB) SetSetting(key, val string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO app_settings (setting_key, setting_value, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(setting_key) DO UPDATE SET
		setting_value = excluded.setting_value,
		updated_at = CURRENT_TIMESTAMP`

	_, err := s.db.Exec(query, key, val)
	return err
}

func (s *DB) GetSetting(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT setting_value FROM app_settings WHERE setting_key = ?`
	var val string
	err := s.db.QueryRow(query, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func (s *DB) GetAllSettings() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT setting_key, setting_value FROM app_settings`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, nil
}

func (s *DB) SetFunctionState(pkg, funcName string, enabled bool, lastStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO function_state (package_name, function_name, is_enabled, last_run_at, last_status)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)
	ON CONFLICT(package_name, function_name) DO UPDATE SET
		is_enabled = excluded.is_enabled,
		last_run_at = CURRENT_TIMESTAMP,
		last_status = excluded.last_status`

	_, err := s.db.Exec(query, pkg, funcName, enabled, lastStatus)
	return err
}

func (s *DB) SetFunctionEnabled(pkg, funcName string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO function_state (package_name, function_name, is_enabled)
	VALUES (?, ?, ?)
	ON CONFLICT(package_name, function_name) DO UPDATE SET
		is_enabled = excluded.is_enabled`

	_, err := s.db.Exec(query, pkg, funcName, enabled)
	return err
}

func (s *DB) GetFunctionState(pkg, funcName string) (bool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT is_enabled, COALESCE(last_status, '') FROM function_state WHERE package_name = ? AND function_name = ?`
	var enabled bool
	var lastStatus string
	err := s.db.QueryRow(query, pkg, funcName).Scan(&enabled, &lastStatus)
	if err == sql.ErrNoRows {
		return true, "", nil // Default enabled
	}
	return enabled, lastStatus, err
}

func (s *DB) IncrementRunCount(pkg, funcName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO function_state (package_name, function_name, run_count, is_enabled)
	VALUES (?, ?, 1, 1)
	ON CONFLICT(package_name, function_name) DO UPDATE SET
		run_count = COALESCE(function_state.run_count, 0) + 1`

	_, err := s.db.Exec(query, pkg, funcName)
	return err
}

func (s *DB) ResetRunCount(pkg, funcName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO function_state (package_name, function_name, run_count, is_enabled)
	VALUES (?, ?, 0, 1)
	ON CONFLICT(package_name, function_name) DO UPDATE SET
		run_count = 0`

	_, err := s.db.Exec(query, pkg, funcName)
	return err
}

func (s *DB) GetRunCount(pkg, funcName string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT COALESCE(run_count, 0) FROM function_state WHERE package_name = ? AND function_name = ?`
	var count int
	err := s.db.QueryRow(query, pkg, funcName).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

func (s *DB) DeleteOldLogs(days int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf("DELETE FROM execution_logs WHERE created_at < datetime('now', '-%d days')", days)
	res, err := s.db.Exec(query)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
