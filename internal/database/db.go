package database

import (
	"database/sql"
	"fmt"
	"strings"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL,
		updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	)`,
	`CREATE TABLE IF NOT EXISTS sync_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		source_path TEXT NOT NULL,
		dest_path TEXT NOT NULL,
		completion_rule TEXT NOT NULL DEFAULT 'keep',
		replace_rule TEXT NOT NULL DEFAULT 'skip',
		scan_interval_sec INTEGER NOT NULL DEFAULT 300,
		enabled INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'idle',
		last_scan_at INTEGER,
		last_sync_at INTEGER,
		error TEXT,
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),
		updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	)`,
	`CREATE TABLE IF NOT EXISTS sync_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL REFERENCES sync_tasks(id) ON DELETE CASCADE,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		details TEXT,
		created_at INTEGER NOT NULL DEFAULT (unixepoch())
	)`,
	`CREATE INDEX IF NOT EXISTS sync_logs_task_idx ON sync_logs(task_id)`,
	`CREATE TABLE IF NOT EXISTS copy_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL REFERENCES sync_tasks(id) ON DELETE CASCADE,
		file_name TEXT NOT NULL,
		src_dir TEXT NOT NULL,
		dst_dir TEXT NOT NULL,
		openlist_task_id TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		retry_count INTEGER NOT NULL DEFAULT 0,
		error TEXT,
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),
		completed_at INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS copy_jobs_task_status_idx ON copy_jobs(task_id, status)`,
	`ALTER TABLE sync_tasks ADD COLUMN match_mode TEXT NOT NULL DEFAULT 'exact'`,
	`ALTER TABLE sync_tasks ADD COLUMN delete_empty_dirs INTEGER NOT NULL DEFAULT 0`,

	// 监控处理服务：单一全局配置（id 恒为 1）
	`CREATE TABLE IF NOT EXISTS monitor_config (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		enabled INTEGER NOT NULL DEFAULT 0,
		scan_interval_sec INTEGER NOT NULL DEFAULT 1800,
		last_run_at INTEGER,
		last_status TEXT,
		updated_at INTEGER NOT NULL DEFAULT (unixepoch())
	)`,
	`CREATE TABLE IF NOT EXISTS monitor_dir (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL UNIQUE,
		kind TEXT NOT NULL,
		created_at INTEGER NOT NULL DEFAULT (unixepoch())
	)`,
	`CREATE INDEX IF NOT EXISTS monitor_dir_kind_idx ON monitor_dir(kind)`,
	// 监控日志独立成表（不复用 sync_logs，避免外键约束冲突：
	// sync_logs.task_id REFERENCES sync_tasks(id)，不存在 id=0 的任务）。
	`CREATE TABLE IF NOT EXISTS monitor_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		details TEXT,
		created_at INTEGER NOT NULL DEFAULT (unixepoch())
	)`,
	`CREATE INDEX IF NOT EXISTS monitor_logs_created_idx ON monitor_logs(id DESC)`,
}

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=1")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !isDuplicateColumn(err) {
				return nil, fmt.Errorf("migration: %w", err)
			}
		}
	}

	return db, nil
}

func isDuplicateColumn(err error) bool {
	return strings.Contains(err.Error(), "duplicate column name")
}
