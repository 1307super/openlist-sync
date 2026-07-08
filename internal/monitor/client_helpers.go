package monitor

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"
)

// scanConcurrency 是并发扫描子目录的 worker 数。
const scanConcurrency = 8

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

// dirNode 表示扫描出的一个目录节点（含自身文件 + 子目录树）。
// 一棵 dirNode 树是一次 scanTree 的产物，供一轮内所有处理函数共用，
// 避免各处理函数重复遍历同一目录树。
type dirNode struct {
	absPath  string              // 该目录的绝对路径
	files    []remoteEntry       // 该目录直接包含的文件
	dirs     []remoteEntry       // 该目录直接包含的子目录（remoteEntry 视图）
	children map[string]*dirNode // 子目录名 -> 节点（并发填充）
	mu       sync.Mutex          // 保护 children 的并发写入
	scanErr  error               // 该目录扫描失败时的错误（非 nil 表示该节点数据不完整）
}

// ensureChild 并发安全地获取或创建一个子节点。
func (n *dirNode) ensureChild(name string) *dirNode {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.children == nil {
		n.children = make(map[string]*dirNode)
	}
	if c, ok := n.children[name]; ok {
		return c
	}
	c := &dirNode{absPath: joinPath(n.absPath, name)}
	n.children[name] = c
	return c
}

// allDirs 扁平化该节点及其所有后代目录节点（含自身），供处理函数遍历。
func (n *dirNode) allDirs() []*dirNode {
	var out []*dirNode
	var walk func(node *dirNode)
	walk = func(node *dirNode) {
		out = append(out, node)
		if node.children != nil {
			for _, c := range node.children {
				walk(c)
			}
		}
	}
	walk(n)
	return out
}

// changedFiles 返回该目录内 modified > since 的文件。
// since 为零值（全量模式）时返回全部文件。
func (n *dirNode) changedFiles(since time.Time) []remoteEntry {
	if since.IsZero() {
		return n.files
	}
	var out []remoteEntry
	for _, f := range n.files {
		if f.modified.After(since) {
			out = append(out, f)
		}
	}
	return out
}

// hasChanged 判断该目录是否有 modified > since 的文件（增量模式跳过判断用）。
func (n *dirNode) hasChanged(since time.Time) bool {
	if since.IsZero() {
		return true
	}
	for _, f := range n.files {
		if f.modified.After(since) {
			return true
		}
	}
	return false
}

// totalSize 递归累加该节点子树内所有文件的大小（字节）。
// 直接使用已扫描的 dirTree，不再额外请求 OpenList API。
func (n *dirNode) totalSize() int64 {
	var total int64
	for _, f := range n.files {
		total += f.size
	}
	if n.children != nil {
		for _, c := range n.children {
			total += c.totalSize()
		}
	}
	return total
}

// countDirs / countFiles 统计该子树的目录/文件数（用于日志）。
func (n *dirNode) countDirs() int {
	c := 1
	for _, child := range n.children {
		c += child.countDirs()
	}
	return c
}
func (n *dirNode) countFiles() int {
	c := len(n.files)
	for _, child := range n.children {
		c += child.countFiles()
	}
	return c
}

// failedCount 统计该子树中扫描失败的节点数。
func (n *dirNode) failedCount() int {
	c := 0
	if n.scanErr != nil {
		c = 1
	}
	for _, child := range n.children {
		c += child.failedCount()
	}
	return c
}

// scanTree 并发全量扫描一棵目录树，返回根 dirNode。
// 用固定 worker 池（信号量容量 scanConcurrency）并发拉取子目录，
// 不按目录 modified 剪枝（云盘目录 modified 不可靠），全部遍历到文件级。
// skipChasingRoots：主目录扫描时跳过与追更目录同名的根层子目录。
// 单个子目录扫描失败不中断整棵树（记 scanErr，其他子目录继续）。
func (s *Service) scanTree(rootPath string, skipChasingRoots map[string]bool) (*dirNode, error) {
	root := &dirNode{absPath: rootPath}
	sem := make(chan struct{}, scanConcurrency)
	var wg sync.WaitGroup

	var scan func(node *dirNode, isRoot bool)
	scan = func(node *dirNode, isRoot bool) {
		defer wg.Done()

		entries, err := s.listDirEntries(node.absPath)
		if err != nil {
			node.scanErr = err
			s.logf("error", "扫描目录失败 %s: %v", node.absPath, err)
			return
		}

		// 分离文件与子目录
		for _, e := range entries {
			if e.isDir {
				// 主目录根层跳过追更同名子目录
				if isRoot && skipChasingRoots != nil && skipChasingRoots[e.name] {
					continue
				}
				node.dirs = append(node.dirs, e)
			} else {
				node.files = append(node.files, e)
			}
		}

		// 并发扫描各子目录
		for _, d := range node.dirs {
			child := node.ensureChild(d.name)
			wg.Add(1)
			sem <- struct{}{}
			go func(c *dirNode) {
				defer func() { <-sem }()
				scan(c, false)
			}(child)
		}
	}

	wg.Add(1)
	scan(root, true)
	wg.Wait()

	return root, nil
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

// parseModified 解析 openlist 返回的 modified 字段（RFC3339）。
// 解析失败返回零值（调用方按零值=全量处理，安全）。
func parseModified(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	// openlist 通常返回 RFC3339（带时区），先按此解析
	t, err := time.Parse(time.RFC3339, v)
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
		if t, err := time.Parse(f, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// rename 调用 OpenList 重命名（filePath 为绝对路径，newName 为新文件名）。
func (s *Service) rename(filePath, newName string) error {
	return s.client.Rename(filePath, newName)
}

// pathBase / pathExt 仅是对 path 包的薄封装，便于在处理函数中统一调用。
func pathBase(p string) string { return path.Base(p) }
func pathExt(p string) string  { return path.Ext(p) }
