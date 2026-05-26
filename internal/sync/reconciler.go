package sync

import (
	"database/sql"
	"fmt"
	"log"
	"path"
	"strings"
	stdsync "sync"
	"time"

	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
)

type PendingReconciler struct {
	db       *sql.DB
	client   *openlist.Client
	interval time.Duration
	stopCh   chan struct{}
	mu       stdsync.Mutex
	running  bool
}

func NewPendingReconciler(db *sql.DB, client *openlist.Client, interval time.Duration) *PendingReconciler {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &PendingReconciler{
		db:       db,
		client:   client,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (r *PendingReconciler) Start() {
	ticker := time.NewTicker(r.interval)
	go func() {
		defer ticker.Stop()
		r.runSafe()
		for {
			select {
			case <-ticker.C:
				r.runSafe()
			case <-r.stopCh:
				return
			}
		}
	}()
	log.Printf("[reconciler] pending copy reconciler started every %s", r.interval)
}

func (r *PendingReconciler) Stop() {
	close(r.stopCh)
}

func (r *PendingReconciler) runSafe() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	defer func() {
		if v := recover(); v != nil {
			log.Printf("[reconciler] PANIC: %v", v)
		}
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	if err := r.ReconcileOnce(); err != nil {
		log.Printf("[reconciler] reconcile pending copies: %v", err)
	}
}

func (r *PendingReconciler) ReconcileOnce() error {
	jobs, err := database.GetPendingCopyJobs(r.db)
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return nil
	}

	taskCache := make(map[int64]*database.SyncTask)
	destCache := make(map[string][]openlist.FileEntry)

	for _, job := range jobs {
		task, ok := taskCache[job.TaskID]
		if !ok {
			task, err = database.GetTaskByID(r.db, job.TaskID)
			if err != nil {
				database.InsertLog(r.db, job.TaskID, "warn",
					fmt.Sprintf("后台确认复制完成失败，任务不存在: %v", err), nil)
				continue
			}
			taskCache[job.TaskID] = task
		}

		destFiles, ok := destCache[job.DstDir]
		if !ok {
			destFiles, err = r.listFiles(job.DstDir)
			if err != nil {
				database.InsertLog(r.db, job.TaskID, "warn",
					fmt.Sprintf("后台确认复制完成失败，扫描目标目录 %s: %v", job.DstDir, err), nil)
				continue
			}
			destCache[job.DstDir] = destFiles
		}

		if !pendingCopyReady(job.FileName, destFiles) {
			continue
		}

		// Rename in dest if needed (e.g., "195 4K.mp4" or "195 4K.mp4.cas" → "S01E195.mp4")
		if newName := RenameTarget(job.FileName); newName != "" {
			currentName, alreadyRenamed := pendingCopyCurrentName(job.FileName, destFiles)
			if alreadyRenamed {
				database.InsertLog(r.db, job.TaskID, "info",
					fmt.Sprintf("目标文件已是重命名后格式: %s", currentName), nil)
			} else {
				filePath := path.Join(job.DstDir, currentName)
				if err := r.client.Rename(filePath, newName); err != nil {
					database.InsertLog(r.db, job.TaskID, "error",
						fmt.Sprintf("重命名失败: %s → %s: %v", currentName, newName, err), nil)
					continue
				}
				destFiles = replaceCachedName(destFiles, currentName, newName)
				destCache[job.DstDir] = destFiles
				database.InsertLog(r.db, job.TaskID, "info",
					fmt.Sprintf("已重命名: %s → %s", currentName, newName), nil)
			}
		}

		if err := database.UpdateCopyJobStatus(r.db, job.ID, "completed", nil, nil); err != nil {
			database.InsertLog(r.db, job.TaskID, "error",
				fmt.Sprintf("更新复制任务完成状态失败: %s → %v", job.FileName, err), nil)
			continue
		}
		database.InsertLog(r.db, job.TaskID, "info",
			fmt.Sprintf("后台确认复制完成: %s", job.FileName), nil)

		if task.CompletionRule == "delete_source" {
			if err := r.client.Remove(job.SrcDir, []string{job.FileName}); err != nil {
				database.InsertLog(r.db, job.TaskID, "error",
					fmt.Sprintf("删除源文件失败: %s → %v", job.FileName, err), nil)
				continue
			}
			database.InsertLog(r.db, job.TaskID, "info",
				fmt.Sprintf("已删除源文件: %s", job.FileName), nil)
		}
	}

	return nil
}

func (r *PendingReconciler) listFiles(dir string) ([]openlist.FileEntry, error) {
	resp, err := r.client.ListDir(dir, 1, 10000)
	if err != nil {
		return nil, err
	}
	files := make([]openlist.FileEntry, 0, len(resp.Content))
	for _, f := range resp.Content {
		if f.IsDir {
			continue
		}
		files = append(files, openlist.FileEntry{RelPath: f.Name, Size: f.Size})
	}
	return files, nil
}

func fileMatches(fileName string, dest []openlist.FileEntry, matchMode string) bool {
	if matchMode == "smart" && smartMatch(fileName, dest) {
		return true
	}
	return exactMatch(fileName, dest)
}

func pendingCopyReady(fileName string, dest []openlist.FileEntry) bool {
	currentName, _ := pendingCopyCurrentName(fileName, dest)
	return currentName != ""
}

func pendingCopyCurrentName(fileName string, dest []openlist.FileEntry) (currentName string, alreadyRenamed bool) {
	if name, ok := findNameVariant(fileName, dest); ok {
		return name, false
	}

	// If a previous reconcile cycle renamed the file but failed before updating DB,
	// allow the job to be marked completed on the next pass.
	if newName := RenameTarget(fileName); newName != "" {
		if name, ok := findNameVariant(newName, dest); ok {
			return name, true
		}
	}
	return "", false
}

func exactNameExists(fileName string, dest []openlist.FileEntry) bool {
	_, ok := findNameVariant(fileName, dest)
	return ok
}

func findNameVariant(fileName string, dest []openlist.FileEntry) (string, bool) {
	want := strings.ToLower(fileName)
	wantCAS := want + ".cas"
	for _, d := range dest {
		name := path.Base(d.RelPath)
		lower := strings.ToLower(name)
		if lower == want || lower == wantCAS {
			return name, true
		}
	}
	return "", false
}

func replaceCachedName(dest []openlist.FileEntry, oldName, newName string) []openlist.FileEntry {
	oldLower := strings.ToLower(oldName)
	for i := range dest {
		if strings.ToLower(path.Base(dest[i].RelPath)) == oldLower {
			dest[i].RelPath = newName
			return dest
		}
	}
	return dest
}
