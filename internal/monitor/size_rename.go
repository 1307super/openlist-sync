package monitor

import (
	"fmt"
	"regexp"
	"strings"
)

// oldSizeTagRe 匹配目录名末尾的旧大小标签（对应脚本 process_directory 的 OLD_SIZE_PATTERN）。
// 例："Show 12.5GB"、"Show 12.5gb"、"Show 200MB"、"Show 100B"。
var oldSizeTagRe = regexp.MustCompile(`(?i)\s\d+\.?\d*(?:KB|MB|GB|TB|B)$`)

// formatSize 对应脚本 format_size：把字节数转为带单位的可读字符串。
// 单位序列 ['B','KB','MB','GB','TB']，逢 1024 进位，保留 2 位小数并去掉 ".00"。
func formatSize(sizeBytes int64) string {
	if sizeBytes == 0 {
		return "0B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	size := float64(sizeBytes)
	for size >= 1024 && i < len(units)-1 {
		size /= 1024.0
		i++
	}
	s := fmt.Sprintf("%.2f%s", size, units[i])
	// 与脚本一致：去掉 ".00"
	return strings.Replace(s, ".00", "", 1)
}

// renameDirsWithSize 对应脚本 process_directory：遍历主目录的子目录，
// 计算每个子目录大小并在目录名末尾追加/更新大小标签（如 "Show" → "Show 12.5GB"）。
func (s *Service) renameDirsWithSize(mainDir string) {
	s.logf("info", "扫描主目录更新大小标签: %s", mainDir)

	subs, err := s.listDirsOnly(mainDir)
	if err != nil {
		s.logf("error", "列出主目录子目录失败 %s: %v", mainDir, err)
		return
	}

	for _, sub := range subs {
		itemName := sub.name
		if containsExcludedKeyword(itemName) {
			continue
		}

		sizeBytes, err := s.calcDirSize(sub.absPath())
		if err != nil {
			s.logf("error", "获取目录大小失败 %s: %v", sub.absPath(), err)
			continue
		}
		newSizeStr := formatSize(sizeBytes)

		var baseName string
		if m := oldSizeTagRe.FindString(itemName); m != "" {
			baseName = strings.TrimSpace(itemName[:len(itemName)-len(m)])
		} else {
			baseName = strings.TrimSpace(itemName)
		}

		newDirName := baseName + " " + newSizeStr
		if itemName == newDirName {
			continue
		}

		if err := s.rename(sub.absPath(), newDirName); err != nil {
			s.logf("error", "重命名目录失败 %s: %v", itemName, err)
		} else {
			s.logf("info", "目录大小更新: %s -> %s", itemName, newDirName)
		}
	}
}
