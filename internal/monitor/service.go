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

// logCleanInterval 是自动清理日志的周期：每天清空一次 monitor_logs 与 sync_logs。
const logCleanInterval = 24 * time.Hour

// stepStats 是单个处理步骤的统计。
// scanFail 是扫描失败（网络异常等，可重试，不推进增量基准）；
// renameFail 是重命名失败（文件已存在/源不存在等业务冲突，不可重试，不阻止推进基准）。
type stepStats struct {
	scanned    int // 处理/扫描的条目数
	scanFail   int // 扫描失败数（可重试）
	renameFail int // 重命名失败数（不可重试）
}

func (a stepStats) add(b stepStats) stepStats {
	return stepStats{
		scanned:    a.scanned + b.scanned,
		scanFail:   a.scanFail + b.scanFail,
		renameFail: a.renameFail + b.renameFail,
	}
}

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

	stopCh    chan struct{} // 监控处理调度的停止信号
	logStopCh chan struct{} // 日志清理调度的停止信号（独立于监控开关）
}

// NewService 创建监控处理服务。不会自动启动监控调度，但会启动日志清理调度
// （日志清理独立于监控开关，始终运行）。
func NewService(db *sql.DB, client *openlist.Client) *Service {
	s := &Service{db: db, client: client}
	s.startLogCleaner()
	return s
}

// startLogCleaner 启动日志清理调度：每 logCleanInterval 清空一次全部日志。
// 独立于监控服务的开关，进程启动即运行。
func (s *Service) startLogCleaner() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.logStopCh != nil {
		return
	}
	s.logStopCh = make(chan struct{})
	go s.logCleanLoop(s.logStopCh)
	log.Printf("[monitor] log cleaner started, clears all logs every %s", logCleanInterval)
}

func (s *Service) logCleanLoop(stopCh chan struct{}) {
	ticker := time.NewTicker(logCleanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanAllLogs()
		case <-stopCh:
			return
		}
	}
}

// cleanAllLogs 清空 monitor_logs 与 sync_logs 两张表，并记录一条清理日志。
func (s *Service) cleanAllLogs() {
	if err := database.ClearMonitorLogs(s.db); err != nil {
		log.Printf("[monitor] clean monitor_logs: %v", err)
		return
	}
	if err := database.ClearAllLogs(s.db); err != nil {
		log.Printf("[monitor] clean sync_logs: %v", err)
		return
	}
	// 清空后写一条日志记录本次清理（用 monitor_logs，避免被自己的清空冲掉）
	if err := database.InsertMonitorLog(s.db, "info", "定时清理：已清空全部日志", nil); err != nil {
		log.Printf("[monitor] log clean result: %v", err)
	}
	log.Printf("[monitor] all logs cleared by scheduled cleaner")
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

	// 从数据库恢复增量扫描基准（持久化，重启不丢失）。
	// 为 nil（未设置）时为零值 → 下次自动全量。
	s.lastScanAt = time.Time{}
	if cfg.LastScanAt != nil {
		s.lastScanAt = time.Unix(*cfg.LastScanAt, 0)
	}

	go s.loop(interval, s.stopCh)
	log.Printf("[monitor] service started, interval=%ds, lastScanAt=%s", interval, formatScanAt(s.lastScanAt))
}

// Stop 停止周期调度与日志清理调度。
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
	if s.logStopCh != nil {
		close(s.logStopCh)
		s.logStopCh = nil
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

// runOnce 是统一的处理编排。forceFull=true 时全量处理。
// 增量模式（forceFull=false）：扫描全量到文件级，但只处理含 file.modified>since 的目录。
// 全量模式（forceFull=true）：处理整棵树所有文件（手动触发、首次启动）。
//
// 关键优化：对每个监控目录只 scanTree 一次（并发 8），后续所有处理函数共用该结果，
// 不再重复遍历。只有当本轮无失败时才推进增量基准 lastScanAt。
func (s *Service) runOnce(forceFull bool) {
	if err := s.loadDirs(); err != nil {
		log.Printf("[monitor] load dirs: %v", err)
		s.logf("error", "加载监控目录失败: %v", err)
		_ = database.UpdateMonitorRunResult(s.db, "error")
		return
	}

	// 计算本次扫描基准（文件级增量判定）
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

	// === 一次性并发扫描所有监控目录（每个目录树只扫一次） ===
	scanStart := time.Now()
	// 主目录扫描时跳过追更同名子目录
	skipMap := s.chasingDirNamesAt()

	mainTrees := make([]*dirNode, len(s.mainDirs))
	for i, d := range s.mainDirs {
		tree, _ := s.scanTree(d, skipMap)
		mainTrees[i] = tree
	}
	chasingTrees := make([]*dirNode, len(s.chasingDirs))
	for i, d := range s.chasingDirs {
		tree, _ := s.scanTree(d, nil)
		chasingTrees[i] = tree
	}

	// 统计扫描结果
	var totalDirs, totalFiles, scanFailed int
	for _, t := range mainTrees {
		if t != nil {
			totalDirs += t.countDirs()
			totalFiles += t.countFiles()
			scanFailed += t.failedCount()
		}
	}
	for _, t := range chasingTrees {
		if t != nil {
			totalDirs += t.countDirs()
			totalFiles += t.countFiles()
			scanFailed += t.failedCount()
		}
	}
	scanDur := time.Since(scanStart).Round(time.Millisecond)
	s.logf("info", "扫描完成：目录 %d，文件 %d，扫描失败 %d，耗时 %s",
		totalDirs, totalFiles, scanFailed, scanDur)

	var total stepStats
	if scanFailed > 0 {
		// 扫描有失败：计入统计，但不中断处理（已扫到的部分仍处理）
		total.scanFail += scanFailed
	}

	// === 用同一份扫描结果依次跑各处理函数 ===
	// 1. 主目录 CAS 同步
	for _, t := range mainTrees {
		if t != nil {
			total = total.add(s.syncMainDirCAS(t, since))
		}
	}
	// 2. 追更目录 CAS 同步
	for _, t := range chasingTrees {
		if t != nil {
			total = total.add(s.syncChasingDirCAS(t, since))
		}
	}
	// 3. 目录大小重命名（独立 3 分钟节流，用真实时间戳判断）
	if s.lastSizeAt.IsZero() || time.Since(s.lastSizeAt) >= sizeInterval {
		s.logf("info", "执行目录大小重命名（距上次 %s）", sinceOrDash(s.lastSizeAt))
		for _, t := range mainTrees {
			if t != nil {
				total = total.add(s.renameDirsWithSize(t))
			}
		}
		s.lastSizeAt = time.Now()
	} else {
		s.logf("info", "跳过目录大小重命名（距上次 %s，不足 %s）",
			time.Since(s.lastSizeAt).Round(time.Second), sizeInterval)
	}
	// 4. 追更目录纯 SxxExx 模板重命名
	for _, t := range chasingTrees {
		if t != nil {
			total = total.add(s.renamePureSxxExx(t, since))
		}
	}
	// 5. HiveWeb 标签（主目录 + 追更目录，与脚本 CHASING_DIRS_HIVEWEB_FULL 一致）
	for _, t := range mainTrees {
		if t != nil {
			total = total.add(s.addHiveWebTag(t, since))
		}
	}
	for _, t := range chasingTrees {
		if t != nil {
			total = total.add(s.addHiveWebTag(t, since))
		}
	}

	dur := time.Since(start).Round(time.Millisecond)

	// 只有扫描失败（可重试）才不推进基准；重命名失败（不可重试）不阻止推进。
	if total.scanFail > 0 {
		s.logf("warn", "监控处理周期完成（耗时 %s）：处理 %d 项，扫描失败 %d 项，重命名失败 %d 项；增量基准保持 %s，下轮将重试",
			dur, total.scanned, total.scanFail, total.renameFail, formatScanAt(s.lastScanAt))
		_ = database.UpdateMonitorRunResult(s.db, "error")
		return
	}

	// 无扫描失败：推进增量基准并持久化（即使有重命名失败也推进，因为不可重试）
	s.lastScanAt = time.Now()
	_ = database.UpdateMonitorLastScanAt(s.db, s.lastScanAt.Unix())
	if total.renameFail > 0 {
		s.logf("warn", "监控处理周期完成（耗时 %s）：处理 %d 项，重命名失败 %d 项（不可重试，已忽略）；增量基准更新为 %s",
			dur, total.scanned, total.renameFail, formatScanAt(s.lastScanAt))
		_ = database.UpdateMonitorRunResult(s.db, "error")
	} else {
		s.logf("info", "监控处理周期完成（耗时 %s）：处理 %d 项，全部成功；增量基准更新为 %s",
			dur, total.scanned, formatScanAt(s.lastScanAt))
		_ = database.UpdateMonitorRunResult(s.db, "idle")
	}
}

func sinceOrDash(t time.Time) string {
	if t.IsZero() {
		return "首次"
	}
	return time.Since(t).Round(time.Second).String()
}

// formatScanAt 格式化扫描基准时间用于日志显示。
func formatScanAt(t time.Time) string {
	if t.IsZero() {
		return "未设置(全量)"
	}
	return t.Format("2006-01-02 15:04:05")
}

// SetLastScanAt 手动设置增量扫描基准（供 API 调用）。
// 传零值=清零（下次全量）；传非零值=以该时间为基准增量扫描。
func (s *Service) SetLastScanAt(t time.Time) {
	s.mu.Lock()
	s.lastScanAt = t
	s.mu.Unlock()
}

// LastScanAt 返回当前增量扫描基准。
func (s *Service) LastScanAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastScanAt
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

// logf 写入一条监控日志（独立 monitor_logs 表，不归属任何同步任务）。
func (s *Service) logf(level, format string, args ...interface{}) {
	if err := database.InsertMonitorLog(s.db, level, fmt.Sprintf(format, args...), nil); err != nil {
		log.Printf("[monitor] 写入日志失败: %v", err)
	}
	log.Printf("[monitor] "+format, args...)
}
