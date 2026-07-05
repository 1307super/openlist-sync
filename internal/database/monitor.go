package database

import (
	"database/sql"
	"time"
)

// MonitorConfig 是监控处理服务的单一全局配置（数据库中仅一行，id=1）。
type MonitorConfig struct {
	Enabled         bool
	ScanIntervalSec int64
	LastRunAt       *int64
	LastStatus      *string
	UpdatedAt       int64
}

type MonitorConfigJSON struct {
	Enabled         bool    `json:"enabled"`
	ScanIntervalSec int64   `json:"scanIntervalSec"`
	LastRunAt       *string `json:"lastRunAt"`
	LastStatus      *string `json:"lastStatus"`
}

func (c *MonitorConfig) ToJSON() MonitorConfigJSON {
	return MonitorConfigJSON{
		Enabled:         c.Enabled,
		ScanIntervalSec: c.ScanIntervalSec,
		LastRunAt:       formatUnixPtr(c.LastRunAt),
		LastStatus:      c.LastStatus,
	}
}

// MonitorDir 是一个被监控的目录（主目录或追更目录）。
type MonitorDir struct {
	ID        int64
	Path      string
	Kind      string // "main" 主目录 / "chasing" 追更目录
	CreatedAt int64
}

type MonitorDirJSON struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
	Kind string `json:"kind"`
}

func (d *MonitorDir) ToJSON() MonitorDirJSON {
	return MonitorDirJSON{ID: d.ID, Path: d.Path, Kind: d.Kind}
}

func GetMonitorConfig(db *sql.DB) (*MonitorConfig, error) {
	// 确保单行配置存在
	if _, err := db.Exec(`INSERT OR IGNORE INTO monitor_config (id, enabled, scan_interval_sec) VALUES (1, 0, 1800)`); err != nil {
		return nil, err
	}

	c := &MonitorConfig{}
	var enabled int64
	err := db.QueryRow(`SELECT enabled, scan_interval_sec, last_run_at, last_status, updated_at
		FROM monitor_config WHERE id = 1`).
		Scan(&enabled, &c.ScanIntervalSec, &c.LastRunAt, &c.LastStatus, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Enabled = enabled != 0
	return c, nil
}

type MonitorConfigUpdate struct {
	Enabled         *bool
	ScanIntervalSec *int64
}

func UpsertMonitorConfig(db *sql.DB, upd MonitorConfigUpdate) error {
	now := time.Now().Unix()
	// 确保单行配置存在
	if _, err := db.Exec(`INSERT OR IGNORE INTO monitor_config (id, enabled, scan_interval_sec) VALUES (1, 0, 1800)`); err != nil {
		return err
	}

	if upd.Enabled != nil {
		e := 0
		if *upd.Enabled {
			e = 1
		}
		if _, err := db.Exec(`UPDATE monitor_config SET enabled=?, updated_at=? WHERE id=1`, e, now); err != nil {
			return err
		}
	}
	if upd.ScanIntervalSec != nil {
		if _, err := db.Exec(`UPDATE monitor_config SET scan_interval_sec=?, updated_at=? WHERE id=1`, *upd.ScanIntervalSec, now); err != nil {
			return err
		}
	}
	return nil
}

func UpdateMonitorRunResult(db *sql.DB, status string) error {
	now := time.Now().Unix()
	_, err := db.Exec(`UPDATE monitor_config SET last_run_at=?, last_status=?, updated_at=? WHERE id=1`,
		now, status, now)
	return err
}

func ListMonitorDirs(db *sql.DB) ([]MonitorDir, error) {
	rows, err := db.Query(`SELECT id, path, kind, created_at FROM monitor_dir ORDER BY kind, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []MonitorDir
	for rows.Next() {
		var d MonitorDir
		if err := rows.Scan(&d.ID, &d.Path, &d.Kind, &d.CreatedAt); err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	return dirs, rows.Err()
}

func ListMonitorDirsByKind(db *sql.DB, kind string) ([]MonitorDir, error) {
	rows, err := db.Query(`SELECT id, path, kind, created_at FROM monitor_dir WHERE kind=? ORDER BY id`, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []MonitorDir
	for rows.Next() {
		var d MonitorDir
		if err := rows.Scan(&d.ID, &d.Path, &d.Kind, &d.CreatedAt); err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	return dirs, rows.Err()
}

func AddMonitorDir(db *sql.DB, dirPath, kind string) (*MonitorDir, error) {
	now := time.Now().Unix()
	res, err := db.Exec(`INSERT INTO monitor_dir (path, kind, created_at) VALUES (?, ?, ?)`,
		dirPath, kind, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &MonitorDir{ID: id, Path: dirPath, Kind: kind, CreatedAt: now}, nil
}

func DeleteMonitorDir(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM monitor_dir WHERE id=?", id)
	return err
}

// GetMonitorLogs 查询监控服务的日志（monitor_logs 表，独立于 sync_logs）。
func GetMonitorLogs(db *sql.DB, page, perPage int) ([]MonitorLog, int, error) {
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM monitor_logs").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := db.Query(`SELECT id, level, message, details, created_at
		FROM monitor_logs ORDER BY id DESC LIMIT ? OFFSET ?`,
		perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []MonitorLog
	for rows.Next() {
		var l MonitorLog
		if err := rows.Scan(&l.ID, &l.Level, &l.Message, &l.Details, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

// MonitorLog 是监控处理服务的一条日志（独立表，不归属同步任务）。
type MonitorLog struct {
	ID        int64
	Level     string
	Message   string
	Details   *string
	CreatedAt int64
}

type MonitorLogJSON struct {
	ID        int64   `json:"id"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Details   *string `json:"details"`
	CreatedAt string  `json:"createdAt"`
}

func (l *MonitorLog) ToJSON() MonitorLogJSON {
	return MonitorLogJSON{
		ID:        l.ID,
		Level:     l.Level,
		Message:   l.Message,
		Details:   l.Details,
		CreatedAt: formatUnix(l.CreatedAt),
	}
}

// InsertMonitorLog 写入一条监控日志。
func InsertMonitorLog(db *sql.DB, level, message string, details *string) error {
	_, err := db.Exec("INSERT INTO monitor_logs (level, message, details) VALUES (?, ?, ?)",
		level, message, details)
	return err
}

// ClearMonitorLogs 清空监控日志。
func ClearMonitorLogs(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM monitor_logs")
	return err
}
