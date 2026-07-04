package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/openlist-sync/internal/auth"
	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/monitor"
	"github.com/user/openlist-sync/internal/openlist"
	"github.com/user/openlist-sync/internal/scheduler"
	"github.com/user/openlist-sync/internal/sync"
)

type Handlers struct {
	db      *sql.DB
	client  *openlist.Client
	engine  *sync.Engine
	sched   *scheduler.Scheduler
	monitor *monitor.Service
	onTGBot func(token string, chatID int64)
}

func NewHandlers(db *sql.DB, client *openlist.Client, engine *sync.Engine, sched *scheduler.Scheduler, mon *monitor.Service) *Handlers {
	return &Handlers{db: db, client: client, engine: engine, sched: sched, monitor: mon}
}

func (h *Handlers) SetTGBotCallback(fn func(token string, chatID int64)) {
	h.onTGBot = fn
}

func (h *Handlers) GetSettings(c *gin.Context) {
	settings, err := database.GetAllSettings(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	result := gin.H{
		"openlist_base_url": "",
		"openlist_token":    "",
		"tg_bot_token":      "",
		"tg_chat_id":        "",
		"auth_username":     "",
	}
	for k, v := range settings {
		if k != "auth_password" {
			result[k] = v
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) UpdateSettings(c *gin.Context) {
	var body map[string]string
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	allowed := map[string]bool{
		"openlist_base_url": true,
		"openlist_token":    true,
		"tg_bot_token":      true,
		"tg_chat_id":        true,
		"auth_username":     true,
		"auth_password":     true,
	}

	for k, v := range body {
		if !allowed[k] || v == "" {
			continue
		}
		val := v
		if k == "auth_password" {
			val = auth.HashPassword(v)
		}
		if err := database.UpsertSetting(h.db, k, val); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	}

	if tgToken, ok := body["tg_bot_token"]; ok {
		tgChatIDStr, _ := database.GetSetting(h.db, "tg_chat_id")
		var chatID int64
		if tgChatIDStr != "" {
			chatID, _ = strconv.ParseInt(tgChatIDStr, 10, 64)
		}
		if h.onTGBot != nil {
			h.onTGBot(tgToken, chatID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) TestSettings(c *gin.Context) {
	err := h.client.TestConnection()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Connection successful"})
}

func (h *Handlers) ListTasks(c *gin.Context) {
	tasks, err := database.GetAllTasks(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	result := make([]database.SyncTaskJSON, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, t.ToJSON())
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) CreateTask(c *gin.Context) {
	var req database.TaskCreateRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	task, err := database.CreateTask(h.db, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	h.sched.StartTask(task.ID, task.ScanIntervalSec)
	c.JSON(http.StatusOK, task.ToJSON())
}

func (h *Handlers) GetTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	task, err := database.GetTaskByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task.ToJSON())
}

func (h *Handlers) UpdateTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	var req database.TaskCreateRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	task, err := database.UpdateTask(h.db, id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if task.Enabled {
		h.sched.RestartTask(task.ID, task.ScanIntervalSec)
	} else {
		h.sched.StopTask(task.ID)
	}
	c.JSON(http.StatusOK, task.ToJSON())
}

func (h *Handlers) DeleteTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	h.sched.StopTask(id)
	if err := database.DeleteTask(h.db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) StartTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	database.UpdateTaskEnabled(h.db, id, true, "idle")
	task, _ := database.GetTaskByID(h.db, id)
	h.sched.StartTask(task.ID, task.ScanIntervalSec)
	c.JSON(http.StatusOK, task.ToJSON())
}

func (h *Handlers) StopTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	database.UpdateTaskEnabled(h.db, id, false, "paused")
	task, _ := database.GetTaskByID(h.db, id)
	h.sched.StopTask(id)
	c.JSON(http.StatusOK, task.ToJSON())
}

func (h *Handlers) TriggerTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}

	task, err := database.GetTaskByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "task not found"})
		return
	}
	if task.Status == "running" {
		c.JSON(http.StatusConflict, gin.H{"message": "task is already running"})
		return
	}

	if !h.sched.TriggerSync(id) {
		c.JSON(http.StatusConflict, gin.H{"message": "task is already running"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sync started", "task_id": id})
}

func (h *Handlers) GetTaskLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	page := openlist.ParseInt(c.DefaultQuery("page", "1"), 1)
	perPage := openlist.ParseInt(c.DefaultQuery("per_page", "50"), 50)

	logs, total, err := database.GetLogsByTask(h.db, id, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	items := make([]database.SyncLogJSON, 0, len(logs))
	for _, l := range logs {
		items = append(items, l.ToJSON())
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *Handlers) GetTaskJobs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	jobs, err := database.GetCopyJobsByTask(h.db, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	items := make([]database.CopyJobJSON, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, j.ToJSON())
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handlers) DeleteTaskJob(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid task id"})
		return
	}
	jobID, err := strconv.ParseInt(c.Param("jobId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid job id"})
		return
	}

	deleted, err := database.DeleteCopyJobByTask(h.db, taskID, jobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"message": "copy job not found"})
		return
	}

	database.InsertLog(h.db, taskID, "warn",
		fmt.Sprintf("手动删除本地复制记录 #%d", jobID), nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) ClearLogs(c *gin.Context) {
	if err := database.ClearAllLogs(h.db); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) BrowseList(c *gin.Context) {
	var body struct {
		Path    string `json:"path"`
		Page    *int   `json:"page"`
		PerPage *int   `json:"per_page"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	page := 1
	if body.Page != nil {
		page = *body.Page
	}
	perPage := 100
	if body.PerPage != nil {
		perPage = *body.PerPage
	}

	resp, err := h.client.ListDir(body.Path, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) BrowseDirs(c *gin.Context) {
	var body struct {
		Path string `json:"path"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	resp, err := h.client.ListDirs(body.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handlers) SyncStatus(c *gin.Context) {
	tasks, err := database.GetAllTasks(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	type taskStatus struct {
		ID         int64   `json:"id"`
		Name       string  `json:"name"`
		Status     string  `json:"status"`
		LastSyncAt *string `json:"lastSyncAt"`
	}

	runningCount := 0
	var taskList []taskStatus
	for _, t := range tasks {
		if t.Status == "running" {
			runningCount++
		}
		taskList = append(taskList, taskStatus{
			ID:         t.ID,
			Name:       t.Name,
			Status:     t.Status,
			LastSyncAt: formatUnixPtr(t.LastSyncAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"runningCount": runningCount,
		"tasks":        taskList,
	})
}

func (h *Handlers) OpenListCopyTasks(c *gin.Context) {
	copyTasks, err := h.client.GetCopyTasks()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"tasks": []gin.H{}, "error": err.Error()})
		return
	}

	tasks := make([]gin.H, 0, len(copyTasks))
	for _, t := range copyTasks {
		tasks = append(tasks, gin.H{
			"id":         t.ID,
			"name":       t.Name,
			"state":      t.State,
			"status":     t.Status,
			"progress":   t.Progress,
			"totalBytes": t.TotalBytes,
			"error":      t.Error,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
	})
}

func (h *Handlers) SyncProgress(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}

	if !h.sched.IsRunning(id) {
		c.JSON(http.StatusOK, gin.H{"running": false})
		return
	}

	copyTasks, err := h.client.GetCopyTasks()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"running": true, "progress": nil, "error": err.Error()})
		return
	}

	var active []gin.H
	for _, t := range copyTasks {
		active = append(active, gin.H{
			"id":         t.ID,
			"name":       t.Name,
			"state":      t.State,
			"status":     t.Status,
			"progress":   t.Progress,
			"totalBytes": t.TotalBytes,
			"error":      t.Error,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"running":   true,
		"copyTasks": active,
		"taskCount": len(active),
	})
}

func (h *Handlers) GetMonitorConfig(c *gin.Context) {
	cfg, err := database.GetMonitorConfig(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg.ToJSON())
}

func (h *Handlers) UpdateMonitorConfig(c *gin.Context) {
	var body struct {
		Enabled         *bool  `json:"enabled"`
		ScanIntervalSec *int64 `json:"scanIntervalSec"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	prev, _ := database.GetMonitorConfig(h.db)

	upd := database.MonitorConfigUpdate{Enabled: body.Enabled, ScanIntervalSec: body.ScanIntervalSec}
	if err := database.UpsertMonitorConfig(h.db, upd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	cfg, _ := database.GetMonitorConfig(h.db)

	// 根据启用状态与间隔变化，调整调度
	intervalChanged := body.ScanIntervalSec != nil && (prev == nil || cfg.ScanIntervalSec != prev.ScanIntervalSec)
	if cfg.Enabled {
		if prev == nil || !prev.Enabled || intervalChanged {
			h.monitor.Restart()
		}
	} else {
		if prev != nil && prev.Enabled {
			h.monitor.Stop()
		}
	}

	c.JSON(http.StatusOK, cfg.ToJSON())
}

func (h *Handlers) ListMonitorDirs(c *gin.Context) {
	dirs, err := database.ListMonitorDirs(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	main := make([]database.MonitorDirJSON, 0)
	chasing := make([]database.MonitorDirJSON, 0)
	for _, d := range dirs {
		if d.Kind == "main" {
			main = append(main, d.ToJSON())
		} else if d.Kind == "chasing" {
			chasing = append(chasing, d.ToJSON())
		}
	}
	c.JSON(http.StatusOK, gin.H{"main": main, "chasing": chasing})
}

func (h *Handlers) AddMonitorDir(c *gin.Context) {
	var body struct {
		Path string `json:"path"`
		Kind string `json:"kind"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if body.Path == "" || (body.Kind != "main" && body.Kind != "chasing") {
		c.JSON(http.StatusBadRequest, gin.H{"message": "path 和 kind(main/chasing) 必填"})
		return
	}

	dir, err := database.AddMonitorDir(h.db, body.Path, body.Kind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dir.ToJSON())
}

func (h *Handlers) DeleteMonitorDir(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
	if err := database.DeleteMonitorDir(h.db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) TriggerMonitor(c *gin.Context) {
	if h.monitor.IsRunning() {
		c.JSON(http.StatusConflict, gin.H{"message": "监控处理正在运行中"})
		return
	}
	if !h.monitor.TriggerOnce() {
		c.JSON(http.StatusConflict, gin.H{"message": "监控处理正在运行中"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "监控处理已触发", "running": true})
}

func (h *Handlers) MonitorStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"running": h.monitor.IsRunning()})
}

func (h *Handlers) GetMonitorLogs(c *gin.Context) {
	page := openlist.ParseInt(c.DefaultQuery("page", "1"), 1)
	perPage := openlist.ParseInt(c.DefaultQuery("per_page", "50"), 50)

	logs, total, err := database.GetMonitorLogs(h.db, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	items := make([]database.SyncLogJSON, 0, len(logs))
	for _, l := range logs {
		items = append(items, l.ToJSON())
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func formatUnixPtr(ts *int64) *string {
	if ts == nil {
		return nil
	}
	s := time.Unix(*ts, 0).UTC().Format(time.RFC3339)
	return &s
}
