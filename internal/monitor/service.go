package monitor

import (
	"database/sql"
	"fmt"
	"log"
	stdsync "sync"
	"time"

	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
)

// sizeIntervalMinutes 与脚本 SIZE_MONITOR_INTERVAL_MINUTES 一致：目录大小
// 重命名每 3 分钟执行一次（按运行轮数节流）。
const sizeIntervalMinutes = 3

// Service 是监控处理服务：周期性对主目录/追更目录执行 CAS 同步、
// 目录大小重命名、纯 SxxExx 模板重命名、HiveWeb 标签添加。
type Service struct {
	db     *sql.DB
	client *openlist.Client

	mainDirs    []string // 运行时从数据库加载
	chasingDirs []string // 运行时从数据库加载

	mu      stdsync.Mutex
	running bool

	stopCh chan struct{}
}

// NewService 创建监控处理服务。不会自动启动。
func NewService(db *sql.DB, client *openlist.Client) *Service {
	return &Service{db: db, client: client}
}

// Start 启动周期调度。重复调用安全（已运行则忽略）。
func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		return
	}
	s.stopCh = make(chan struct{})

	cfg, err := database.GetMonitorConfig(s.db)
	if err != nil {
		log.Printf("[monitor] load config: %v", err)
		return
	}
	interval := cfg.ScanIntervalSec
	if interval < 10 {
		interval = 10
	}

	go s.loop(interval, s.stopCh)
	log.Printf("[monitor] service started, interval=%ds", interval)
}

// Stop 停止周期调度。
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
}

// Restart 重新以新的间隔启动调度。
func (s *Service) Restart() {
	s.Stop()
	// 等待旧 goroutine 退出（stopCh 关闭后 loop 会返回）
	time.Sleep(100 * time.Millisecond)
	s.Start()
}

func (s *Service) loop(intervalSec int64, stopCh chan struct{}) {
	// 脚本：大小监控每 SIZE_MONITOR_INTERVAL_MINUTES*60/RUN_INTERVAL 轮执行一次
	sizeEveryN := int(sizeIntervalMinutes*60) / int(intervalSec)
	if sizeEveryN < 1 {
		sizeEveryN = 1
	}
	sizeCounter := 0

	// 启动时立即执行一次
	for {
		s.runSafe(sizeCounter%sizeEveryN == 0)
		sizeCounter++
		select {
		case <-time.After(time.Duration(intervalSec) * time.Second):
		case <-stopCh:
			return
		}
	}
}

func (s *Service) runSafe(runSize bool) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[monitor] PANIC: %v", r)
			s.logf("error", "监控处理异常: %v", r)
			_ = database.UpdateMonitorRunResult(s.db, "error")
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	if err := s.loadDirs(); err != nil {
		log.Printf("[monitor] load dirs: %v", err)
		s.logf("error", "加载监控目录失败: %v", err)
		_ = database.UpdateMonitorRunResult(s.db, "error")
		return
	}

	s.logf("info", "监控处理周期开始（主目录 %d，追更目录 %d）", len(s.mainDirs), len(s.chasingDirs))

	// 1. 主目录 CAS 同步
	for _, d := range s.mainDirs {
		s.syncMainDirCAS(d)
	}
	// 2. 追更目录 CAS 同步
	for _, d := range s.chasingDirs {
		s.syncChasingDirCAS(d)
	}
	// 3. 目录大小重命名（按节流执行）
	if runSize {
		for _, d := range s.mainDirs {
			s.renameDirsWithSize(d)
		}
	}
	// 4. 追更目录纯 SxxExx 模板重命名
	for _, d := range s.chasingDirs {
		s.renamePureSxxExx(d)
	}
	// 5. HiveWeb 标签（主目录 + 追更目录，与脚本 CHASING_DIRS_HIVEWEB_FULL 一致）
	for _, d := range s.mainDirs {
		s.addHiveWebTag(d)
	}
	for _, d := range s.chasingDirs {
		s.addHiveWebTag(d)
	}

	s.logf("info", "监控处理周期完成")
	_ = database.UpdateMonitorRunResult(s.db, "idle")
}

// loadDirs 从数据库加载主目录/追更目录列表到运行时字段。
func (s *Service) loadDirs() error {
	mainDirs, err := database.ListMonitorDirsByKind(s.db, "main")
	if err != nil {
		return err
	}
	chasingDirs, err := database.ListMonitorDirsByKind(s.db, "chasing")
	if err != nil {
		return err
	}
	s.mainDirs = make([]string, 0, len(mainDirs))
	for _, d := range mainDirs {
		s.mainDirs = append(s.mainDirs, d.Path)
	}
	s.chasingDirs = make([]string, 0, len(chasingDirs))
	for _, d := range chasingDirs {
		s.chasingDirs = append(s.chasingDirs, d.Path)
	}
	return nil
}

// TriggerOnce 异步触发一次完整的监控处理。返回是否成功提交（已在运行则返回 false）。
func (s *Service) TriggerOnce() bool {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[monitor] trigger PANIC: %v", r)
			}
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()

		s.mu.Lock()
		if s.running {
			s.mu.Unlock()
			return
		}
		s.running = true
		s.mu.Unlock()

		if err := s.loadDirs(); err != nil {
			s.logf("error", "加载监控目录失败: %v", err)
			_ = database.UpdateMonitorRunResult(s.db, "error")
			return
		}
		s.logf("info", "手动触发监控处理（主目录 %d，追更目录 %d）", len(s.mainDirs), len(s.chasingDirs))
		for _, d := range s.mainDirs {
			s.syncMainDirCAS(d)
		}
		for _, d := range s.chasingDirs {
			s.syncChasingDirCAS(d)
		}
		for _, d := range s.mainDirs {
			s.renameDirsWithSize(d)
		}
		for _, d := range s.chasingDirs {
			s.renamePureSxxExx(d)
		}
		for _, d := range s.mainDirs {
			s.addHiveWebTag(d)
		}
		for _, d := range s.chasingDirs {
			s.addHiveWebTag(d)
		}
		s.logf("info", "手动触发监控处理完成")
		_ = database.UpdateMonitorRunResult(s.db, "idle")
	}()
	return true
}

// IsRunning 返回当前是否正在执行一次处理。
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// logf 写入一条监控日志（task_id = 0 表示监控服务，不归属于任何同步任务）。
func (s *Service) logf(level, format string, args ...interface{}) {
	_ = database.InsertLog(s.db, 0, level, fmt.Sprintf(format, args...), nil)
	log.Printf("[monitor] "+format, args...)
}
