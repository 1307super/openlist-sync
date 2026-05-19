package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"github.com/user/openlist-sync/internal/api"
	"github.com/user/openlist-sync/internal/auth"
	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
	"github.com/user/openlist-sync/internal/scheduler"
	"github.com/user/openlist-sync/internal/static"
	syncengine "github.com/user/openlist-sync/internal/sync"
	"github.com/user/openlist-sync/internal/telegram"
)

func main() {
	port := getEnv("PORT", "3000")
	dataDir := getEnv("DATA_DIR", "./data")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "openlist-sync.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer db.Close()

	client := openlist.NewClient(db)
	engine := syncengine.NewEngine(db, client)
	reconciler := syncengine.NewPendingReconciler(db, client, 30*time.Second)
	sched := scheduler.NewScheduler(db, engine)
	handlers := api.NewHandlers(db, client, engine, sched)

	api.InitDefaultCredentials(db)

	if adminPass := os.Getenv("ADMIN_PASSWORD"); adminPass != "" {
		database.UpsertSetting(db, "auth_password", auth.HashPassword(adminPass))
		log.Printf("[main] admin password reset via ADMIN_PASSWORD env")
	}

	authSecret := getEnv("AUTH_SECRET", "openlist-sync-default-secret-change-me")

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	var staticFS fs.FS
	if sub, err := fs.Sub(static.DistFS, "dist"); err == nil {
		staticFS = sub
	}

	api.RegisterRoutes(r, handlers, authSecret, staticFS)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	go func() {
		log.Printf("[main] server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	sched.Start()
	log.Printf("[main] scheduler started")
	reconciler.Start()

	var tgBot *telegram.Bot
	var tgMu sync.Mutex

	applyTGBot := func(token string, chatID int64) {
		tgMu.Lock()
		defer tgMu.Unlock()
		if tgBot != nil {
			tgBot.Stop()
			tgBot = nil
		}
		if token == "" {
			log.Printf("[main] telegram bot disabled")
			return
		}
		bot, err := telegram.NewBot(token, chatID, db, engine)
		if err != nil {
			log.Printf("[main] telegram bot init failed: %v", err)
			return
		}
		tgBot = bot
		bot.Start()
	}

	handlers.SetTGBotCallback(func(token string, chatID int64) {
		applyTGBot(token, chatID)
	})

	tgToken, _ := database.GetSetting(db, "tg_bot_token")
	tgChatIDStr, _ := database.GetSetting(db, "tg_chat_id")
	var tgChatID int64
	if tgChatIDStr != "" {
		tgChatID, _ = strconv.ParseInt(tgChatIDStr, 10, 64)
	}
	if tgToken != "" {
		applyTGBot(tgToken, tgChatID)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] shutting down...")

	sched.StopAll()
	reconciler.Stop()
	if tgBot != nil {
		tgBot.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[main] server shutdown: %v", err)
	}

	log.Println("[main] exited")
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
