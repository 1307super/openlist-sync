package scheduler

import (
	"database/sql"
	"log"
	stdsync "sync"
	"time"

	"github.com/user/openlist-sync/internal/database"
	syncengine "github.com/user/openlist-sync/internal/sync"
)

type Scheduler struct {
	db      *sql.DB
	engine  *syncengine.Engine
	tickers map[int64]*time.Ticker
	running map[int64]bool
	stopCh  map[int64]chan struct{}
	mu      stdsync.Mutex
}

func NewScheduler(db *sql.DB, engine *syncengine.Engine) *Scheduler {
	return &Scheduler{
		db:      db,
		engine:  engine,
		tickers: make(map[int64]*time.Ticker),
		running: make(map[int64]bool),
		stopCh:  make(map[int64]chan struct{}),
	}
}

func (s *Scheduler) Start() {
	tasks, err := database.GetEnabledTasks(s.db)
	if err != nil {
		log.Printf("[scheduler] load tasks: %v", err)
		return
	}
	for _, t := range tasks {
		s.StartTask(t.ID, t.ScanIntervalSec)
	}
	log.Printf("[scheduler] started %d tasks", len(tasks))
}

func (s *Scheduler) StartTask(taskID int64, intervalSec int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tickers[taskID]; exists {
		return
	}

	if intervalSec < 10 {
		intervalSec = 10
	}

	stopCh := make(chan struct{})
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	s.tickers[taskID] = ticker
	s.stopCh[taskID] = stopCh

	go func() {
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				isRunning := s.running[taskID]
				s.mu.Unlock()

				if isRunning {
					log.Printf("[scheduler] task %d already running, skipping", taskID)
					continue
				}

				s.mu.Lock()
				s.running[taskID] = true
				s.mu.Unlock()

				log.Printf("[scheduler] running task %d", taskID)
				s.engine.RunSync(taskID)

				s.mu.Lock()
				s.running[taskID] = false
				s.mu.Unlock()

			case <-stopCh:
				return
			}
		}
	}()

	log.Printf("[scheduler] task %d scheduled every %ds", taskID, intervalSec)
}

func (s *Scheduler) StopTask(taskID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ticker, exists := s.tickers[taskID]; exists {
		ticker.Stop()
		delete(s.tickers, taskID)
	}
	if stopCh, exists := s.stopCh[taskID]; exists {
		close(stopCh)
		delete(s.stopCh, taskID)
	}
	delete(s.running, taskID)
}

func (s *Scheduler) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, ticker := range s.tickers {
		ticker.Stop()
		delete(s.tickers, id)
	}
	for id, stopCh := range s.stopCh {
		close(stopCh)
		delete(s.stopCh, id)
	}
	s.running = make(map[int64]bool)
}

func (s *Scheduler) RestartTask(taskID int64, intervalSec int64) {
	s.StopTask(taskID)
	s.StartTask(taskID, intervalSec)
}
