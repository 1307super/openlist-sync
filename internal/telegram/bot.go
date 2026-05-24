package telegram

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	stdsync "sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/user/openlist-sync/internal/database"
	syncengine "github.com/user/openlist-sync/internal/sync"
)

type Bot struct {
	api        *tgbotapi.BotAPI
	db         *sql.DB
	engine     *syncengine.Engine
	chatID     int64
	sessions   map[int64]*createSession
	sessionsMu stdsync.Mutex
}

type createSession struct {
	step    int
	name    string
	srcPath string
	dstPath string
}

func NewBot(token string, chatID int64, db *sql.DB, engine *syncengine.Engine) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{
		api:      api,
		db:       db,
		engine:   engine,
		chatID:   chatID,
		sessions: make(map[int64]*createSession),
	}, nil
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			if update.Message != nil {
				b.handleMessage(update.Message)
			} else if update.CallbackQuery != nil {
				b.handleCallback(update.CallbackQuery)
			}
		}
	}()

	log.Printf("[telegram] bot @%s started (chat_id=%d)", b.api.Self.UserName, b.chatID)
}

func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

func (b *Bot) isAllowed(chatID int64) bool {
	if b.chatID == 0 {
		return true
	}
	return chatID == b.chatID
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.Text == "" {
		return
	}
	if !b.isAllowed(msg.Chat.ID) {
		return
	}

	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	parts := strings.Fields(text)
	cmd := parts[0]

	switch cmd {
	case "/start":
		b.send(chatID, "OpenList 同步机器人\n\n命令列表：\n/tasks - 任务列表\n/task <id> - 任务详情\n/create [名称 源路径 目标路径 [smart]] - 创建任务\n/delete <id> - 删除任务\n/start_task <id> - 启动任务\n/stop_task <id> - 停止任务\n/trigger <id> - 立即同步\n/logs <id> - 最近日志\n/clearlogs - 清空所有日志\n/settings - 查看设置")

	case "/tasks":
		b.handleTasks(chatID)

	case "/task":
		if len(parts) < 2 {
			b.send(chatID, "用法: /task <id>")
			return
		}
		b.handleTaskDetail(chatID, parts[1])

	case "/create":
		b.handleCreate(chatID, parts[1:])

	case "/delete":
		if len(parts) < 2 {
			b.send(chatID, "用法: /delete <id>")
			return
		}
		b.handleDelete(chatID, parts[1])

	case "/start_task":
		if len(parts) < 2 {
			b.send(chatID, "用法: /start_task <id>")
			return
		}
		b.handleStartTask(chatID, parts[1])

	case "/stop_task":
		if len(parts) < 2 {
			b.send(chatID, "用法: /stop_task <id>")
			return
		}
		b.handleStopTask(chatID, parts[1])

	case "/trigger":
		if len(parts) < 2 {
			b.send(chatID, "用法: /trigger <id>")
			return
		}
		b.handleTrigger(chatID, parts[1])

	case "/logs":
		if len(parts) < 2 {
			b.send(chatID, "用法: /logs <id>")
			return
		}
		b.handleLogs(chatID, parts[1])

	case "/clearlogs":
		b.handleClearLogs(chatID)

	case "/settings":
		b.handleSettings(chatID)

	case "/cancel":
		b.sessionsMu.Lock()
		delete(b.sessions, chatID)
		b.sessionsMu.Unlock()
		b.send(chatID, "已取消")

	default:
		b.sessionsMu.Lock()
		session, inSession := b.sessions[chatID]
		b.sessionsMu.Unlock()
		if inSession {
			b.handleCreateStep(chatID, session, text)
			return
		}
		b.send(chatID, "未知命令，发送 /start 查看帮助")
	}
}

func (b *Bot) handleCreate(chatID int64, args []string) {
	if len(args) >= 3 {
		name := args[0]
		src := args[1]
		dst := args[2]
		matchMode := "exact"
		if len(args) >= 4 && (args[3] == "smart" || args[3] == "exact") {
			matchMode = args[3]
		}
		req := database.TaskCreateRequest{
			Name:       &name,
			SourcePath: &src,
			DestPath:   &dst,
			MatchMode:  &matchMode,
		}
		task, err := database.CreateTask(b.db, req)
		if err != nil {
			b.send(chatID, fmt.Sprintf("创建失败: %v", err))
			return
		}
		b.send(chatID, fmt.Sprintf("任务已创建：\n#%d %s\n%s → %s\n完成规则: 保留文件\n替换规则: 跳过已存在\n匹配模式: %s\n扫描间隔: 300s",
			task.ID, task.Name, task.SourcePath, task.DestPath, matchModeLabel(task.MatchMode)))
		return
	}

	b.sessionsMu.Lock()
	b.sessions[chatID] = &createSession{step: 1}
	b.sessionsMu.Unlock()

	b.send(chatID, "创建新任务（发送 /cancel 取消）\n\n请输入任务名称：")
}

func (b *Bot) handleCreateStep(chatID int64, session *createSession, text string) {
	switch session.step {
	case 1:
		session.name = text
		session.step = 2
		b.send(chatID, fmt.Sprintf("名称: %s\n\n请输入源路径：", text))
	case 2:
		session.srcPath = text
		session.step = 3
		b.send(chatID, fmt.Sprintf("源路径: %s\n\n请输入目标路径：", text))
	case 3:
		session.dstPath = text
		b.sessionsMu.Lock()
		delete(b.sessions, chatID)
		b.sessionsMu.Unlock()

		req := database.TaskCreateRequest{
			Name:       &session.name,
			SourcePath: &session.srcPath,
			DestPath:   &session.dstPath,
		}
		task, err := database.CreateTask(b.db, req)
		if err != nil {
			b.send(chatID, fmt.Sprintf("创建失败: %v", err))
			return
		}
		b.send(chatID, fmt.Sprintf("任务已创建：\n#%d %s\n%s → %s\n完成规则: 保留文件\n替换规则: 跳过已存在\n匹配模式: 精确匹配\n扫描间隔: 300s",
			task.ID, task.Name, task.SourcePath, task.DestPath))
	}
}

func (b *Bot) handleDelete(chatID int64, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.send(chatID, "无效的任务 ID")
		return
	}
	task, err := database.GetTaskByID(b.db, id)
	if err != nil {
		b.send(chatID, "任务不存在")
		return
	}
	if err := database.DeleteTask(b.db, id); err != nil {
		b.send(chatID, fmt.Sprintf("删除失败: %v", err))
		return
	}
	b.send(chatID, fmt.Sprintf("任务 #%d %s 已删除", id, task.Name))
}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	chatID := cb.Message.Chat.ID
	if !b.isAllowed(chatID) {
		callback := tgbotapi.NewCallback(cb.ID, "无权限")
		b.api.Send(callback)
		return
	}

	if strings.HasPrefix(data, "task:") {
		id := strings.TrimPrefix(data, "task:")
		b.handleTaskDetail(chatID, id)
	} else if strings.HasPrefix(data, "start:") {
		id := strings.TrimPrefix(data, "start:")
		b.handleStartTask(chatID, id)
	} else if strings.HasPrefix(data, "stop:") {
		id := strings.TrimPrefix(data, "stop:")
		b.handleStopTask(chatID, id)
	} else if strings.HasPrefix(data, "trigger:") {
		id := strings.TrimPrefix(data, "trigger:")
		b.handleTrigger(chatID, id)
	} else if strings.HasPrefix(data, "logs:") {
		id := strings.TrimPrefix(data, "logs:")
		b.handleLogs(chatID, id)
	} else if data == "create" {
		b.handleCreate(chatID, nil)
	} else if strings.HasPrefix(data, "delete:") {
		id := strings.TrimPrefix(data, "delete:")
		b.handleDelete(chatID, id)
	}

	callback := tgbotapi.NewCallback(cb.ID, "")
	b.api.Send(callback)
}

func (b *Bot) handleTasks(chatID int64) {
	tasks, err := database.GetAllTasks(b.db)
	if err != nil {
		b.send(chatID, "加载任务失败")
		return
	}
	if len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "暂无同步任务")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ 创建任务", "create"),
			),
		)
		b.api.Send(msg)
		return
	}

	var sb strings.Builder
	sb.WriteString("同步任务列表：\n\n")
	buttons := make([][]tgbotapi.InlineKeyboardButton, 0, len(tasks)+1)

	for _, t := range tasks {
		emoji := "⏸"
		if t.Status == "running" {
			emoji = "▶️"
		} else if t.Status == "error" {
			emoji = "❌"
		} else if t.Enabled {
			emoji = "✅"
		}
		statusText := statusLabel(t.Status)
		sb.WriteString(fmt.Sprintf("%s [%d] %s - %s\n", emoji, t.ID, t.Name, statusText))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %s", emoji, t.Name),
				fmt.Sprintf("task:%d", t.ID),
			),
		))
	}

	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("➕ 创建任务", "create"),
	))

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	b.api.Send(msg)
}

func (b *Bot) handleTaskDetail(chatID int64, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.send(chatID, "无效的任务 ID")
		return
	}

	task, err := database.GetTaskByID(b.db, id)
	if err != nil {
		b.send(chatID, "任务不存在")
		return
	}

	tj := task.ToJSON()
	text := fmt.Sprintf("任务 #%d: %s\n源路径: %s\n目标路径: %s\n状态: %s\n完成规则: %s\n替换规则: %s\n匹配模式: %s\n扫描间隔: %ds",
		tj.ID, tj.Name, tj.SourcePath, tj.DestPath, statusLabel(tj.Status),
		completionRuleLabel(tj.CompletionRule), replaceRuleLabel(tj.ReplaceRule), matchModeLabel(tj.MatchMode), tj.ScanIntervalSec)

	if tj.LastSyncAt != nil {
		text += fmt.Sprintf("\n上次同步: %s", *tj.LastSyncAt)
	}
	if tj.Error != nil {
		text += fmt.Sprintf("\n错误: %s", *tj.Error)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶ 启动", fmt.Sprintf("start:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("⏹ 停止", fmt.Sprintf("stop:%d", id)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 同步", fmt.Sprintf("trigger:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("📋 日志", fmt.Sprintf("logs:%d", id)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 删除", fmt.Sprintf("delete:%d", id)),
		),
	)
	b.api.Send(msg)
}

func statusLabel(status string) string {
	switch status {
	case "idle":
		return "等待中"
	case "running":
		return "运行中"
	case "paused":
		return "已暂停"
	case "error":
		return "错误"
	default:
		return status
	}
}

func completionRuleLabel(rule string) string {
	switch rule {
	case "keep":
		return "保留文件"
	case "delete_source":
		return "删除源文件"
	default:
		return rule
	}
}

func replaceRuleLabel(rule string) string {
	switch rule {
	case "skip":
		return "跳过已存在"
	case "overwrite":
		return "覆盖已存在"
	default:
		return rule
	}
}

func matchModeLabel(mode string) string {
	switch mode {
	case "smart":
		return "追更匹配"
	case "exact":
		return "精确匹配"
	default:
		return mode
	}
}

func (b *Bot) handleStartTask(chatID int64, idStr string) {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	database.UpdateTaskEnabled(b.db, id, true, "idle")
	b.send(chatID, fmt.Sprintf("任务 #%d 已启动", id))
}

func (b *Bot) handleStopTask(chatID int64, idStr string) {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	database.UpdateTaskEnabled(b.db, id, false, "paused")
	b.send(chatID, fmt.Sprintf("任务 #%d 已停止", id))
}

func (b *Bot) handleTrigger(chatID int64, idStr string) {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	b.send(chatID, fmt.Sprintf("正在触发任务 #%d 同步...", id))

	result := b.engine.RunSync(id)
	text := fmt.Sprintf("任务 #%d 同步完成：\n扫描: %d\n跳过: %d\n缺失: %d\n已复制: %d\n失败: %d\n已删除: %d\n耗时: %dms",
		result.TaskID, result.Scanned, result.Skipped, result.Missing, result.Copied, result.Failed, result.Deleted, result.DurationMs)
	if result.Error != "" {
		text += fmt.Sprintf("\n错误: %s", result.Error)
	}
	b.send(chatID, text)
}

func (b *Bot) handleLogs(chatID int64, idStr string) {
	id, _ := strconv.ParseInt(idStr, 10, 64)
	logs, _, err := database.GetLogsByTask(b.db, id, 1, 10)
	if err != nil || len(logs) == 0 {
		b.send(chatID, "暂无日志")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务 #%d 最近日志：\n\n", id))
	for _, l := range logs {
		emoji := "ℹ️"
		if l.Level == "warn" {
			emoji = "⚠️"
		} else if l.Level == "error" {
			emoji = "❌"
		}
		lj := l.ToJSON()
		sb.WriteString(fmt.Sprintf("%s [%s] %s\n", emoji, lj.CreatedAt, lj.Message))
	}
	b.send(chatID, sb.String())
}

func (b *Bot) handleClearLogs(chatID int64) {
	if err := database.ClearAllLogs(b.db); err != nil {
		b.send(chatID, fmt.Sprintf("清空日志失败: %v", err))
		return
	}
	b.send(chatID, "已清空所有任务日志")
}

func (b *Bot) handleSettings(chatID int64) {
	settings, err := database.GetAllSettings(b.db)
	if err != nil {
		b.send(chatID, "加载设置失败")
		return
	}

	text := fmt.Sprintf("当前设置：\n服务地址: %s\nToken: %s\nTG Bot: 已配置=%v",
		settings["openlist_base_url"],
		mask(settings["openlist_token"]),
		settings["tg_bot_token"] != "")
	b.send(chatID, text)
}

func (b *Bot) send(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("[telegram] send error: %v", err)
	}
}

func mask(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
