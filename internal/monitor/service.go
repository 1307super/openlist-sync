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

// sizeInterval 与脚本 SIZE_MONITOR_INTERVAL_MINUTES 一致：目录大小重命名
// 每 3 分钟执行一次（用真实时间戳判断，与扫描间隔解耦）。
const sizeInterval = 3 * time.Minute

// Service 是监控处理服务：周期性对主目录/追更目录执行 CAS 同步、
// 目录大小重命名、纯 SxxExx 模板重命名、HiveWeb 标签添加。
type Service struct {
	db     *sql.DB
	client *openlist.Client

	mainDirs    []string // 运行时从数据库加载
	chasingDirs []string // 运行时从数据库加载

	// 增量扫描基准：上次成功扫描的时间。零值=下次全量。
	// 仅存内存，进程重启后自动全量一次（安全）。
	lastScanAt time.Time
	// 上次执行目录大小重命名的时间，独立于扫描间隔。
	lastSizeAt time.Time

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

	// 重启服务时强制全量一次
	s.lastScanAt = time.Time{}

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
	time.Sleep(150 * time.Millisecond)
	s.Start()
}

func (s *Service) loop(intervalSec int64, stopCh chan struct{}) {
	// 启动后立即执行一次（Start 已把 lastScanAt 清零 → 全量）
	for {
		s.runSafe(false) // 定时任务走增量
		select {
		case <-time.After(time.Duration(intervalSec) * time.Second):
		case <-stopCh:
			return
		}
	}
}

// runSafe 执行一次处理，带并发保护与 panic 恢复。
// forceFull=true 时忽略增量基准，全量扫描（手动触发用）。
func (s *Service) runSafe(forceFull bool) {
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

	s.runOnce(forceFull)
}

// runOnce 是统一的处理编排。forceFull=true 时全量扫描。
// 增量模式（forceFull=false）：以 lastScanAt 为基准，只扫描有变动的目录子树；
// 全量模式（forceFull=true）：扫描整棵目录树（手动触发、首次启动）。
func (s *Service) runOnce(forceFull bool) {
	if err := s.loadDirs(); err != nil {
		log.Printf("[monitor] load dirs: %v", err)
		s.logf("error", "加载监控目录失败: %v", err)
		_ = database.UpdateMonitorRunResult(s.db, "error")
		return
	}

	// 计算本次扫描基准
	var since time.Time
	mode := "增量"
	if forceFull || s.lastScanAt.IsZero() {
		since = time.Time{}
		if forceFull {
			mode = "全量(手动)"
		} else {
			mode = "全量(首次)"
		}
	} else {
		since = s.lastScanAt
	}

	start := time.Now()
	s.logf("info", "监控处理周期开始 [%s]：主目录 %d，追更目录 %d", mode, len(s.mainDirs), len(s.chasingDirs))

	// 1. 主目录 CAS 同步
	for _, d := range s.mainDirs {
		s.syncMainDirCAS(d, since)
	}
	// 2. 追更目录 CAS 同步
	for _, d := range s.chasingDirs {
		s.syncChasingDirCAS(d, since)
	}
	// 3. 目录大小重命名（独立 3 分钟节流，用真实时间戳判断）
	if s.lastSizeAt.IsZero() || time.Since(s.lastSizeAt) >= sizeInterval {
		s.logf("info", "执行目录大小重命名（距上次 %s）", sinceOrDash(s.lastSizeAt))
		for _, d := range s.mainDirs {
			s.renameDirsWithSize(d)
		}
		s.lastSizeAt = time.Now()
	} else {
		s.logf("info", "跳过目录大小重命名（距上次 %s，不足 %s）",
			time.Since(s.lastSizeAt).Round(time.Second), sizeInterval)
	}
	// 4. 追更目录纯 SxxExx 模板重命名
	for _, d := range s.chasingDirs {
		s.renamePureSxxExx(d, since)
	}
	// 5. HiveWeb 标签（主目录 + 追更目录，与脚本 CHASING_DIRS_HIVEWEB_FULL 一致）
	for _, d := range s.mainDirs {
		s.addHiveWebTag(d, since)
	}
	for _, d := range s.chasingDirs {
		s.addHiveWebTag(d, since)
	}

	// 更新增量基准：本次成功完成后记录时间
	s.lastScanAt = time.Now()
	dur := time.Since(start).Round(time.Millisecond)
	s.logf("info", "监控处理周期完成，耗时 %s", dur)
	_ = database.UpdateMonitorRunResult(s.db, "idle")
}

func sinceOrDash(t time.Time) string {
	if t.IsZero() {
		return "首次"
	}
	return time.Since(t).Round(time.Second).String()
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

// TriggerOnce 异步触发一次完整的监控处理（强制全量）。返回是否成功提交（已在运行则返回 false）。
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

		s.runOnce(true)
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
