package monitor

import (
	"fmt"
	"path"
	"strings"
)

// remoteEntry 表示远端目录中的一个条目（文件或子目录）。
type remoteEntry struct {
	absDir string // 该条目所在的绝对目录路径
	name   string // 条目名称
	isDir  bool
	size   int64
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
				absDir: dir,
				name:   f.Name,
				isDir:  f.IsDir,
				size:   f.Size,
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
