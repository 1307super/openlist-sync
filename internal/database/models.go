package database

import (
	"database/sql"
	"time"
)

type SyncTask struct {
	ID              int64
	Name            string
	SourcePath      string
	DestPath        string
	CompletionRule  string
	ReplaceRule     string
	ScanIntervalSec int64
	Enabled         bool
	Status          string
	LastScanAt      *int64
	LastSyncAt      *int64
	Error           *string
	CreatedAt       int64
	UpdatedAt       int64
}

type SyncTaskJSON struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	SourcePath      string  `json:"sourcePath"`
	DestPath        string  `json:"destPath"`
	CompletionRule  string  `json:"completionRule"`
	ReplaceRule     string  `json:"replaceRule"`
	ScanIntervalSec int64   `json:"scanIntervalSec"`
	Enabled         bool    `json:"enabled"`
	Status          string  `json:"status"`
	LastScanAt      *string `json:"lastScanAt"`
	LastSyncAt      *string `json:"lastSyncAt"`
	Error           *string `json:"error"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type TaskCreateRequest struct {
	Name            *string `json:"name"`
	SourcePath      *string `json:"sourcePath"`
	DestPath        *string `json:"destPath"`
	CompletionRule  *string `json:"completionRule"`
	ReplaceRule     *string `json:"replaceRule"`
	ScanIntervalSec *int64  `json:"scanIntervalSec"`
	Enabled         *bool   `json:"enabled"`
}

func (t *SyncTask) ToJSON() SyncTaskJSON {
	return SyncTaskJSON{
		ID:              t.ID,
		Name:            t.Name,
		SourcePath:      t.SourcePath,
		DestPath:        t.DestPath,
		CompletionRule:  t.CompletionRule,
		ReplaceRule:     t.ReplaceRule,
		ScanIntervalSec: t.ScanIntervalSec,
		Enabled:         t.Enabled,
		Status:          t.Status,
		LastScanAt:      formatUnixPtr(t.LastScanAt),
		LastSyncAt:      formatUnixPtr(t.LastSyncAt),
		Error:           t.Error,
		CreatedAt:       formatUnix(t.CreatedAt),
		UpdatedAt:       formatUnix(t.UpdatedAt),
	}
}

func formatUnix(ts int64) string {
	if ts == 0 {
		return time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func formatUnixPtr(ts *int64) *string {
	if ts == nil {
		return nil
	}
	s := formatUnix(*ts)
	return &s
}

type SyncLog struct {
	ID        int64
	TaskID    int64
	Level     string
	Message   string
	Details   *string
	CreatedAt int64
}

type SyncLogJSON struct {
	ID        int64   `json:"id"`
	TaskID    int64   `json:"taskId"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Details   *string `json:"details"`
	CreatedAt string  `json:"createdAt"`
}

func (l *SyncLog) ToJSON() SyncLogJSON {
	return SyncLogJSON{
		ID:        l.ID,
		TaskID:    l.TaskID,
		Level:     l.Level,
		Message:   l.Message,
		Details:   l.Details,
		CreatedAt: formatUnix(l.CreatedAt),
	}
}

type CopyJob struct {
	ID             int64
	TaskID         int64
	FileName       string
	SrcDir         string
	DstDir         string
	OpenlistTaskID *string
	Status         string
	RetryCount     int64
	Error          *string
	CreatedAt      int64
	CompletedAt    *int64
}

type CopyJobJSON struct {
	ID             int64   `json:"id"`
	TaskID         int64   `json:"taskId"`
	FileName       string  `json:"fileName"`
	SrcDir         string  `json:"srcDir"`
	DstDir         string  `json:"dstDir"`
	OpenlistTaskID *string `json:"openlistTaskId"`
	Status         string  `json:"status"`
	RetryCount     int64   `json:"retryCount"`
	Error          *string `json:"error"`
	CreatedAt      string  `json:"createdAt"`
	CompletedAt    *string `json:"completedAt"`
}

func (j *CopyJob) ToJSON() CopyJobJSON {
	return CopyJobJSON{
		ID:             j.ID,
		TaskID:         j.TaskID,
		FileName:       j.FileName,
		SrcDir:         j.SrcDir,
		DstDir:         j.DstDir,
		OpenlistTaskID: j.OpenlistTaskID,
		Status:         j.Status,
		RetryCount:     j.RetryCount,
		Error:          j.Error,
		CreatedAt:      formatUnix(j.CreatedAt),
		CompletedAt:    formatUnixPtr(j.CompletedAt),
	}
}

func GetAllSettings(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

func UpsertSetting(db *sql.DB, key, value string) error {
	now := time.Now().Unix()
	_, err := db.Exec(
		"INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at",
		key, value, now,
	)
	return err
}

func GetSetting(db *sql.DB, key string) (string, error) {
	var v string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func scanTask(row interface{ Scan(...interface{}) error }) (*SyncTask, error) {
	t := &SyncTask{}
	var enabled int64
	err := row.Scan(
		&t.ID, &t.Name, &t.SourcePath, &t.DestPath,
		&t.CompletionRule, &t.ReplaceRule, &t.ScanIntervalSec,
		&enabled, &t.Status, &t.LastScanAt, &t.LastSyncAt,
		&t.Error, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.Enabled = enabled != 0
	return t, nil
}

func GetAllTasks(db *sql.DB) ([]SyncTask, error) {
	rows, err := db.Query(`SELECT id, name, source_path, dest_path, completion_rule, replace_rule,
		scan_interval_sec, enabled, status, last_scan_at, last_sync_at, error, created_at, updated_at
		FROM sync_tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []SyncTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

func GetTaskByID(db *sql.DB, id int64) (*SyncTask, error) {
	row := db.QueryRow(`SELECT id, name, source_path, dest_path, completion_rule, replace_rule,
		scan_interval_sec, enabled, status, last_scan_at, last_sync_at, error, created_at, updated_at
		FROM sync_tasks WHERE id = ?`, id)
	return scanTask(row)
}

func CreateTask(db *sql.DB, req TaskCreateRequest) (*SyncTask, error) {
	now := time.Now().Unix()
	name := valStr(req.Name, "")
	sourcePath := valStr(req.SourcePath, "")
	destPath := valStr(req.DestPath, "")
	completionRule := valStr(req.CompletionRule, "keep")
	replaceRule := valStr(req.ReplaceRule, "skip")
	scanInterval := valInt64(req.ScanIntervalSec, 300)

	res, err := db.Exec(`INSERT INTO sync_tasks
		(name, source_path, dest_path, completion_rule, replace_rule, scan_interval_sec, enabled, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, 'idle', ?, ?)`,
		name, sourcePath, destPath, completionRule, replaceRule, scanInterval, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetTaskByID(db, id)
}

func UpdateTask(db *sql.DB, id int64, req TaskCreateRequest) (*SyncTask, error) {
	existing, err := GetTaskByID(db, id)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	name := existing.Name
	sourcePath := existing.SourcePath
	destPath := existing.DestPath
	completionRule := existing.CompletionRule
	replaceRule := existing.ReplaceRule
	scanInterval := existing.ScanIntervalSec

	if req.Name != nil {
		name = *req.Name
	}
	if req.SourcePath != nil {
		sourcePath = *req.SourcePath
	}
	if req.DestPath != nil {
		destPath = *req.DestPath
	}
	if req.CompletionRule != nil {
		completionRule = *req.CompletionRule
	}
	if req.ReplaceRule != nil {
		replaceRule = *req.ReplaceRule
	}
	if req.ScanIntervalSec != nil {
		scanInterval = *req.ScanIntervalSec
	}

	_, err = db.Exec(`UPDATE sync_tasks SET name=?, source_path=?, dest_path=?,
		completion_rule=?, replace_rule=?, scan_interval_sec=?, updated_at=? WHERE id=?`,
		name, sourcePath, destPath, completionRule, replaceRule, scanInterval, now, id)
	if err != nil {
		return nil, err
	}
	return GetTaskByID(db, id)
}

func DeleteTask(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM sync_tasks WHERE id = ?", id)
	return err
}

func UpdateTaskStatus(db *sql.DB, id int64, status string, taskErr *string) error {
	now := time.Now().Unix()
	_, err := db.Exec("UPDATE sync_tasks SET status=?, error=?, updated_at=? WHERE id=?",
		status, taskErr, now, id)
	return err
}

func UpdateTaskEnabled(db *sql.DB, id int64, enabled bool, status string) error {
	now := time.Now().Unix()
	e := 0
	if enabled {
		e = 1
	}
	_, err := db.Exec("UPDATE sync_tasks SET enabled=?, status=?, updated_at=? WHERE id=?",
		e, status, now, id)
	return err
}

func UpdateTaskLastSync(db *sql.DB, id int64) error {
	now := time.Now().Unix()
	_, err := db.Exec("UPDATE sync_tasks SET last_sync_at=?, last_scan_at=?, updated_at=? WHERE id=?",
		now, now, now, id)
	return err
}

func InsertLog(db *sql.DB, taskID int64, level, message string, details *string) error {
	_, err := db.Exec("INSERT INTO sync_logs (task_id, level, message, details) VALUES (?, ?, ?, ?)",
		taskID, level, message, details)
	return err
}

func GetLogsByTask(db *sql.DB, taskID int64, page, perPage int) ([]SyncLog, int, error) {
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM sync_logs WHERE task_id = ?", taskID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := db.Query(`SELECT id, task_id, level, message, details, created_at
		FROM sync_logs WHERE task_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		taskID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []SyncLog
	for rows.Next() {
		var l SyncLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.Level, &l.Message, &l.Details, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func GetCopyJobsByTask(db *sql.DB, taskID int64) ([]CopyJob, error) {
	rows, err := db.Query(`SELECT id, task_id, file_name, src_dir, dst_dir, openlist_task_id,
		status, retry_count, error, created_at, completed_at
		FROM copy_jobs WHERE task_id = ? ORDER BY id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []CopyJob
	for rows.Next() {
		var j CopyJob
		if err := rows.Scan(&j.ID, &j.TaskID, &j.FileName, &j.SrcDir, &j.DstDir,
			&j.OpenlistTaskID, &j.Status, &j.RetryCount, &j.Error, &j.CreatedAt, &j.CompletedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func InsertCopyJob(db *sql.DB, taskID int64, fileName, srcDir, dstDir string) (int64, error) {
	res, err := db.Exec(`INSERT INTO copy_jobs (task_id, file_name, src_dir, dst_dir, status)
		VALUES (?, ?, ?, ?, 'pending')`, taskID, fileName, srcDir, dstDir)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateCopyJobStatus(db *sql.DB, id int64, status string, openlistTaskID *string, jobErr *string) error {
	now := time.Now().Unix()
	if status == "completed" || status == "failed" {
		_, err := db.Exec(`UPDATE copy_jobs SET status=?, openlist_task_id=?, error=?, completed_at=? WHERE id=?`,
			status, openlistTaskID, jobErr, now, id)
		return err
	}
	_, err := db.Exec(`UPDATE copy_jobs SET status=?, openlist_task_id=?, error=? WHERE id=?`,
		status, openlistTaskID, jobErr, id)
	return err
}

func IncrementCopyJobRetry(db *sql.DB, id int64) error {
	_, err := db.Exec("UPDATE copy_jobs SET retry_count = retry_count + 1, status = 'pending' WHERE id = ?", id)
	return err
}

func GetEnabledTasks(db *sql.DB) ([]SyncTask, error) {
	rows, err := db.Query(`SELECT id, name, source_path, dest_path, completion_rule, replace_rule,
		scan_interval_sec, enabled, status, last_scan_at, last_sync_at, error, created_at, updated_at
		FROM sync_tasks WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []SyncTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

func valStr(ptr *string, def string) string {
	if ptr != nil {
		return *ptr
	}
	return def
}

func valInt64(ptr *int64, def int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return def
}
