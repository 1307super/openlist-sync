package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
	"github.com/user/openlist-sync/internal/sync"
)

type Handlers struct {
	db       *sql.DB
	client   *openlist.Client
	engine   *sync.Engine
	onTGBot func(token string, chatID int64)
}

func NewHandlers(db *sql.DB, client *openlist.Client, engine *sync.Engine) *Handlers {
	return &Handlers{db: db, client: client, engine: engine}
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
	}
	for k, v := range settings {
		result[k] = v
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
	}

	for k, v := range body {
		if allowed[k] {
			if err := database.UpsertSetting(h.db, k, v); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
				return
			}
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
	c.JSON(http.StatusOK, task.ToJSON())
}

func (h *Handlers) DeleteTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid id"})
		return
	}
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

	go h.engine.RunSync(id)

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
		ID        int64   `json:"id"`
		Name      string  `json:"name"`
		Status    string  `json:"status"`
		LastSyncAt *string `json:"lastSyncAt"`
	}

	runningCount := 0
	var taskList []taskStatus
	for _, t := range tasks {
		if t.Status == "running" {
			runningCount++
		}
		taskList = append(taskList, taskStatus{
			ID:        t.ID,
			Name:      t.Name,
			Status:    t.Status,
			LastSyncAt: formatUnixPtr(t.LastSyncAt),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"runningCount": runningCount,
		"tasks":        taskList,
	})
}

func formatUnixPtr(ts *int64) *string {
	if ts == nil {
		return nil
	}
	s := time.Unix(*ts, 0).UTC().Format(time.RFC3339)
	return &s
}
