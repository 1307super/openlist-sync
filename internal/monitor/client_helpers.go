package monitor

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// remoteEntry 表示远端目录中的一个条目（文件或子目录）。
type remoteEntry struct {
	absDir   string // 该条目所在的绝对目录路径
	name     string // 条目名称
	isDir    bool
	size     int64
	modified time.Time // openlist 返回的修改时间（解析失败为零值）
}

// absPath 返回该条目的绝对路径。
func (e remoteEntry) absPath() string {
	return joinPath(e.absDir, e.name)
}

// joinPath 安全地拼接目录与名称，保证有且仅有一个分隔符。
func joinPath(dir, name string) string {
	if dir == "" || dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

// parentDir 返回路径的父目录（OpenList 风格，根为 "/"）。
func parentDir(p string) string {
	if p == "/" || p == "" {
		return "/"
	}
	idx := strings.LastIndex(p, "/")
	if idx <= 0 {
		return "/"
	}
	return p[:idx]
}

// listDirEntries 列出目录下的所有条目（文件 + 子目录），分页拉取直到取完。
func (s *Service) listDirEntries(dir string) ([]remoteEntry, error) {
	page := 1
	perPage := 500
	var out []remoteEntry
	for {
		resp, err := s.client.ListDir(dir, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("list %s page %d: %w", dir, page, err)
		}
		for _, f := range resp.Content {
			if strings.HasPrefix(f.Name, ".") {
				continue
			}
			out = append(out, remoteEntry{
				absDir:   dir,
				name:     f.Name,
				isDir:    f.IsDir,
				size:     f.Size,
				modified: parseModified(f.Modified),
			})
		}
		if resp.Total <= perPage*page {
			break
		}
		page++
	}
	return out, nil
}

// listDirsOnly 仅列出目录下的子目录。
func (s *Service) listDirsOnly(dir string) ([]remoteEntry, error) {
	entries, err := s.listDirEntries(dir)
	if err != nil {
		return nil, err
	}
	var dirs []remoteEntry
	for _, e := range entries {
		if e.isDir {
			dirs = append(dirs, e)
		}
	}
	return dirs, nil
}

// walkDir 深度优先递归扫描目录，返回所有条目（含子目录自身），同时返回
// 按"目录路径"分组的映射，便于上层按目录批量处理。
//
// rootPath 是扫描根（不会被排除）。skipChasingRoots 用于主目录扫描时跳过
// 与追更目录重名的子目录（对应脚本里 "跳过追更目录" 的逻辑）。
func (s *Service) walkDir(rootPath string, skipChasingRoots map[string]bool) ([]remoteEntry, error) {
	var all []remoteEntry
	var walk func(current string) error
	walk = func(current string) error {
		entries, err := s.listDirEntries(current)
		if err != nil {
			return err
		}
		for _, e := range entries {
			// 主目录扫描时跳过作为子目录出现的追更目录
			if e.isDir && skipChasingRoots != nil && current == rootPath && skipChasingRoots[e.name] {
				continue
			}
			all = append(all, e)
			if e.isDir {
				if err := walk(e.absPath()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(rootPath); err != nil {
		return nil, err
	}
	return all, nil
}

// walkChanged 增量递归扫描：从顶层目录开始，逐层用子目录的 modified 时间剪枝。
//   - since 为零值时等价于全量扫描（首次运行或重启）。
//   - 对当前目录的每个【子目录】，仅当 modified > since 才递归进去；
//     子目录 modified 未变 → 连同它的整棵子树一起跳过（含其中的文件）。
//   - 当前目录内的【文件】始终纳入（目录树上的变动最终会通过某层目录的 modified 变化体现）。
//
// 返回的条目按目录分组后，调用方可只对实际变动的目录做处理。
func (s *Service) walkChanged(rootPath string, since time.Time, skipChasingRoots map[string]bool) ([]remoteEntry, error) {
	var all []remoteEntry
	skipped := 0
	var walk func(current string, isRoot bool) error
	walk = func(current string, isRoot bool) error {
		entries, err := s.listDirEntries(current)
		if err != nil {
			return err
		}
		for _, e := range entries {
			// 主目录扫描时跳过作为子目录出现的追更目录
			if e.isDir && skipChasingRoots != nil && isRoot && skipChasingRoots[e.name] {
				continue
			}
			if e.isDir {
				// 增量剪枝：子目录 modified 未超过基准时间则整棵跳过。
				// 根目录本身总是处理（isRoot 时无条件进入）。
				if !isRoot && !since.IsZero() && !e.modified.After(since) {
					skipped++
					continue
				}
				all = append(all, e)
				if err := walk(e.absPath(), false); err != nil {
					return err
				}
			} else {
				all = append(all, e)
			}
		}
		return nil
	}
	if err := walk(rootPath, true); err != nil {
		return nil, err
	}
	if skipped > 0 && !since.IsZero() {
		s.logf("info", "增量扫描 %s：跳过 %d 个未变动的子目录", rootPath, skipped)
	}
	return all, nil
}

// parseModified 解析 openlist 返回的 modified 字段（RFC3339）。
// 解析失败返回零值（调用方按零值=全量处理，安全）。
func parseModified(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// openlist 通常返回 RFC3339（带时区），先按此解析
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t
	}
	// 兜底：尝试其他常见格式
	formats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// groupByDir 把扁平条目按所在目录分组。
func groupByDir(entries []remoteEntry) map[string][]remoteEntry {
	m := make(map[string][]remoteEntry)
	for _, e := range entries {
		m[e.absDir] = append(m[e.absDir], e)
	}
	return m
}

// calcDirSize 递归累加目录下所有文件的大小（字节）。
func (s *Service) calcDirSize(dir string) (int64, error) {
	entries, err := s.walkDir(dir, nil)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, e := range entries {
		if !e.isDir {
			total += e.size
		}
	}
	return total, nil
}

// rename 调用 OpenList 重命名（filePath 为绝对路径，newName 为新文件名）。
func (s *Service) rename(filePath, newName string) error {
	return s.client.Rename(filePath, newName)
}

// pathBase / pathExt 仅是对 path 包的薄封装，便于在处理函数中统一调用。
func pathBase(p string) string { return path.Base(p) }
func pathExt(p string) string  { return path.Ext(p) }
