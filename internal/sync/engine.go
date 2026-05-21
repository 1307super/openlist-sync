package sync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path"
	stdsync "sync"
	"time"

	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
)

type SyncResult struct {
	TaskID     int64  `json:"taskId"`
	Scanned    int    `json:"scanned"`
	Skipped    int    `json:"skipped"`
	Missing    int    `json:"missing"`
	Copied     int    `json:"copied"`
	Failed     int    `json:"failed"`
	Deleted    int    `json:"deleted"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
}

type Engine struct {
	db     *sql.DB
	client *openlist.Client
	copier *Copier

	taskMu   map[int64]*stdsync.Mutex
	taskMuMu stdsync.Mutex
}

func NewEngine(db *sql.DB, client *openlist.Client) *Engine {
	return &Engine{
		db:     db,
		client: client,
		copier: NewCopier(client),
		taskMu: make(map[int64]*stdsync.Mutex),
	}
}

func (e *Engine) getTaskMu(taskID int64) *stdsync.Mutex {
	e.taskMuMu.Lock()
	defer e.taskMuMu.Unlock()
	if mu, ok := e.taskMu[taskID]; ok {
		return mu
	}
	mu := &stdsync.Mutex{}
	e.taskMu[taskID] = mu
	return mu
}

func (e *Engine) RunSync(taskID int64) SyncResult {
	mu := e.getTaskMu(taskID)
	if !mu.TryLock() {
		return SyncResult{TaskID: taskID, Error: "task already running"}
	}
	defer mu.Unlock()
	start := time.Now()
	result := SyncResult{TaskID: taskID}

	database.UpdateTaskStatus(e.db, taskID, "running", nil)
	database.InsertLog(e.db, taskID, "info", "同步周期开始", nil)

	task, err := database.GetTaskByID(e.db, taskID)
	if err != nil {
		result.Error = fmt.Sprintf("load task: %v", err)
		e.finishSync(taskID, &result, start)
		return result
	}

	sourceFiles, err := e.client.ScanAllFilesRecursive(task.SourcePath)
	if err != nil {
		result.Error = fmt.Sprintf("scan source: %v", err)
		e.finishSync(taskID, &result, start)
		return result
	}
	result.Scanned = len(sourceFiles)

	destFiles, err := e.client.ScanAllFilesRecursive(task.DestPath)
	if err != nil {
		result.Error = fmt.Sprintf("scan dest: %v", err)
		e.finishSync(taskID, &result, start)
		return result
	}

	pendingFiles, _ := database.GetPendingCopyFiles(e.db, taskID)

	missing := CompareFilesRecursive(sourceFiles, destFiles, task.MatchMode, task.SourcePath, pendingFiles)
	result.Missing = len(missing)
	result.Skipped = result.Scanned - result.Missing

	if len(missing) == 0 {
		database.InsertLog(e.db, taskID, "info",
			fmt.Sprintf("未发现缺失文件（跳过 %d 个已存在）", result.Skipped), nil)
	} else {
		database.InsertLog(e.db, taskID, "info",
			fmt.Sprintf("发现 %d 个缺失文件（跳过 %d 个已存在），开始复制...", len(missing), result.Skipped), nil)

		availableSlots := e.copier.concurrency - len(pendingFiles)
		if availableSlots <= 0 {
			database.InsertLog(e.db, taskID, "info",
				fmt.Sprintf("已有 %d 个复制任务等待完成，达到并发上限 %d，本轮不提交新任务", len(pendingFiles), e.copier.concurrency), nil)
		} else {
			if len(missing) > availableSlots {
				database.InsertLog(e.db, taskID, "info",
					fmt.Sprintf("并发上限 %d，已有 %d 个等待完成，本轮只提交 %d 个新任务", e.copier.concurrency, len(pendingFiles), availableSlots), nil)
				missing = missing[:availableSlots]
			}

			overwrite := task.ReplaceRule == "overwrite"
			skipExisting := task.ReplaceRule == "skip"

			var items []CopyItem
			var jobIDs []int64
			for _, f := range missing {
				srcDir, dstDir, fileName := RelPathToCopyDirs(f.RelPath, task.SourcePath, task.DestPath)
				items = append(items, CopyItem{
					FileName: fileName,
					SrcDir:   srcDir,
					DstDir:   dstDir,
				})
				jobID, _ := database.InsertCopyJob(e.db, taskID, fileName, srcDir, dstDir)
				jobIDs = append(jobIDs, jobID)
			}

			copyResults := e.copier.CopyFiles(items, overwrite, skipExisting)

			for i, cr := range copyResults {
				if cr.Error != nil {
					result.Failed++
					errStr := cr.Error.Error()
					database.UpdateCopyJobStatus(e.db, jobIDs[i], "failed", nil, &errStr)
					database.InsertLog(e.db, taskID, "error",
						fmt.Sprintf("复制失败: %s → %v", items[i].FileName, cr.Error), nil)
				} else {
					result.Skipped++
					result.Missing--
					database.InsertLog(e.db, taskID, "info",
						fmt.Sprintf("复制任务已提交，等待后台确认完成: %s", items[i].FileName), nil)
				}
			}
		}
	}

	// 清理源目录下的空目录
	if task.DeleteEmptyDirs {
		deleted, err := e.cleanEmptyDirs(taskID, task.SourcePath, task.SourcePath)
		if err != nil {
			database.InsertLog(e.db, taskID, "error",
				fmt.Sprintf("清理空目录失败: %v", err), nil)
		} else if deleted > 0 {
			result.Deleted = deleted
			database.InsertLog(e.db, taskID, "info",
				fmt.Sprintf("已清理 %d 个空目录", deleted), nil)
		}
	}

	e.finishSync(taskID, &result, start)
	return result
}

// cleanEmptyDirs 递归删除空目录。rootPath 是源目录（不会被删除），currentPath 是当前递归路径。
func (e *Engine) cleanEmptyDirs(taskID int64, rootPath, currentPath string) (int, error) {
	dirsResp, err := e.client.ListDirs(currentPath)
	if err != nil {
		return 0, fmt.Errorf("list dirs %s: %w", currentPath, err)
	}

	deleted := 0

	// 先递归处理子目录
	for _, d := range dirsResp.Content {
		subPath := path.Join(currentPath, d.Name)
		n, err := e.cleanEmptyDirs(taskID, rootPath, subPath)
		if err != nil {
			return deleted, err
		}
		deleted += n
	}

	// 不删除源目录本身
	if currentPath == rootPath {
		return deleted, nil
	}

	// 检查当前目录是否为空
	resp, err := e.client.ListDir(currentPath, 1, 1)
	if err != nil {
		return deleted, fmt.Errorf("check empty %s: %w", currentPath, err)
	}
	if resp.Total == 0 {
		parentDir := path.Dir(currentPath)
		dirName := path.Base(currentPath)
		if err := e.client.Remove(parentDir, []string{dirName}); err != nil {
			return deleted, fmt.Errorf("remove empty dir %s: %w", currentPath, err)
		}
		deleted++
	}

	return deleted, nil
}

func (e *Engine) finishSync(taskID int64, result *SyncResult, start time.Time) {
	result.DurationMs = time.Since(start).Milliseconds()

	summary := map[string]interface{}{
		"scanned":    result.Scanned,
		"missing":    result.Missing,
		"skipped":    result.Skipped,
		"copied":     result.Copied,
		"failed":     result.Failed,
		"deleted":    result.Deleted,
		"durationMs": result.DurationMs,
	}
	summaryJSON, _ := json.Marshal(summary)

	var taskErr *string
	if result.Error != "" {
		taskErr = &result.Error
		database.UpdateTaskStatus(e.db, taskID, "error", taskErr)
		database.InsertLog(e.db, taskID, "error", result.Error, nil)
	} else {
		database.UpdateTaskStatus(e.db, taskID, "idle", nil)
		msg := fmt.Sprintf("同步完成：扫描 %d | 缺失 %d | 跳过 %d | 已复制 %d | 失败 %d | 已删除 %d | 耗时 %dms",
			result.Scanned, result.Missing, result.Skipped, result.Copied, result.Failed, result.Deleted, result.DurationMs)
		database.InsertLog(e.db, taskID, "info", msg, strPtr(string(summaryJSON)))
	}

	database.UpdateTaskLastSync(e.db, taskID)
	log.Printf("[sync] task=%d scanned=%d skipped=%d missing=%d copied=%d failed=%d deleted=%d dur=%dms",
		taskID, result.Scanned, result.Skipped, result.Missing, result.Copied, result.Failed, result.Deleted, result.DurationMs)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
