package sync

import (
	"database/sql"
	"fmt"
	"log"
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
}

func NewEngine(db *sql.DB, client *openlist.Client) *Engine {
	return &Engine{
		db:     db,
		client: client,
		copier: NewCopier(client),
	}
}

func (e *Engine) RunSync(taskID int64) SyncResult {
	start := time.Now()
	result := SyncResult{TaskID: taskID}

	database.UpdateTaskStatus(e.db, taskID, "running", nil)
	database.InsertLog(e.db, taskID, "info", "Sync cycle started", nil)

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

	missing := CompareFilesRecursive(sourceFiles, destFiles)
	result.Missing = len(missing)
	result.Skipped = result.Scanned - result.Missing

	if len(missing) == 0 {
		database.InsertLog(e.db, taskID, "info",
			fmt.Sprintf("No missing files found (%d skipped)", result.Skipped), nil)
		e.finishSync(taskID, &result, start)
		return result
	}

	database.InsertLog(e.db, taskID, "info",
		fmt.Sprintf("Found %d missing files (%d skipped)", len(missing), result.Skipped), nil)

	overwrite := task.ReplaceRule == "overwrite"
	skipExisting := task.ReplaceRule == "skip"

	var items []CopyItem
	for _, f := range missing {
		srcDir, dstDir, fileName := RelPathToCopyDirs(f.RelPath, task.SourcePath, task.DestPath)
		items = append(items, CopyItem{
			FileName: fileName,
			SrcDir:   srcDir,
			DstDir:   dstDir,
		})
	}

	copyResults := e.copier.CopyFiles(items, overwrite, skipExisting)

	var deletedNames []string
	var deletedSrcDirs []string
	for i, cr := range copyResults {
		jobID, _ := database.InsertCopyJob(e.db, taskID, items[i].FileName, items[i].SrcDir, items[i].DstDir)

		if cr.Error != nil {
			result.Failed++
			errStr := cr.Error.Error()
			database.UpdateCopyJobStatus(e.db, jobID, "failed", strPtr(cr.TaskID), &errStr)
			database.InsertLog(e.db, taskID, "error",
				fmt.Sprintf("Copy failed: %s: %v", items[i].FileName, cr.Error), nil)
		} else {
			result.Copied++
			database.UpdateCopyJobStatus(e.db, jobID, "completed", strPtr(cr.TaskID), nil)
			deletedNames = append(deletedNames, items[i].FileName)
			deletedSrcDirs = append(deletedSrcDirs, items[i].SrcDir)
		}
	}

	if task.CompletionRule == "delete_source" && len(deletedNames) > 0 {
		if err := e.deleteSourceFiles(deletedSrcDirs, deletedNames); err != nil {
			database.InsertLog(e.db, taskID, "error",
				fmt.Sprintf("Delete source failed: %v", err), nil)
		} else {
			result.Deleted = len(deletedNames)
			database.InsertLog(e.db, taskID, "info",
				fmt.Sprintf("Deleted %d source files", len(deletedNames)), nil)
		}
	}

	e.finishSync(taskID, &result, start)
	return result
}

func (e *Engine) deleteSourceFiles(srcDirs, fileNames []string) error {
	grouped := make(map[string][]string)
	for i, dir := range srcDirs {
		grouped[dir] = append(grouped[dir], fileNames[i])
	}
	for dir, names := range grouped {
		if err := e.client.Remove(dir, names); err != nil {
			return fmt.Errorf("remove from %s: %w", dir, err)
		}
	}
	return nil
}

func (e *Engine) finishSync(taskID int64, result *SyncResult, start time.Time) {
	result.DurationMs = time.Since(start).Milliseconds()

	var taskErr *string
	if result.Error != "" {
		taskErr = &result.Error
		database.UpdateTaskStatus(e.db, taskID, "error", taskErr)
		database.InsertLog(e.db, taskID, "error", result.Error, nil)
	} else {
		database.UpdateTaskStatus(e.db, taskID, "idle", nil)
		database.InsertLog(e.db, taskID, "info",
			fmt.Sprintf("Sync completed: %d scanned, %d skipped, %d copied, %d failed, %d deleted in %dms",
				result.Scanned, result.Skipped, result.Copied, result.Failed, result.Deleted, result.DurationMs), nil)
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
