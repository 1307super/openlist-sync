package monitor

import (
	"regexp"
	"strings"
	"time"
)

var (
	// targetRenamePattern 匹配纯剧集文件名（对应脚本 TARGET_RENAME_PATTERN）：
	// 整个文件名就是 SxxExx（季至少2位），可选跟随质量标签等。例："S01E01"、"S01E01.4K"。
	targetRenamePattern = regexp.MustCompile(`(?i)^(S\d{2,}E\d{2,})(\..+)?$`)
	// sxxExxExtractor 提取文件名中出现的 SxxExx 编号（对应脚本 SXXEXX_EXTRACTOR）。
	sxxExxExtractor = regexp.MustCompile(`(?i)S\d{1,}E\d{1,}`)
)

// renamePureSxxExx 对应脚本 auto_rename_pure_sxxexx_files：在追更目录中，
// 把纯剧集文件名（如 "S01E01.mkv"）用同目录模板文件（含 SxxExx 的非纯文件，
// 如 "Show.S01E01.1080P.mkv"）构造出完整命名（→ "Show.S01E01.1080P.mkv"）。
// since 非零时按子目录 modified 增量剪枝；为零时全量扫描。
func (s *Service) renamePureSxxExx(chasingDir string, since time.Time) stepStats {
	var (
		entries []remoteEntry
		err     error
	)
	if since.IsZero() {
		entries, err = s.walkDir(chasingDir, nil)
	} else {
		entries, err = s.walkChanged(chasingDir, since, nil)
	}
	if err != nil {
		s.logf("error", "扫描追更目录失败 %s: %v", chasingDir, err)
		return stepStats{failed: 1}
	}

	var stats stepStats
	byDir := groupByDir(entries)
	for dir, files := range byDir {
		// 先在当前目录内查找一个可用模板（含 SxxExx 的非纯文件，取第一个）
		var template string
		for _, f := range files {
			if f.isDir {
				continue
			}
			if targetRenamePattern.MatchString(f.name) {
				continue
			}
			if sxxExxExtractor.FindString(f.name) != "" {
				// 去掉扩展名后，把第一个 SxxExx 替换为占位符作为模板
				ext := pathExt(f.name)
				tBase := strings.TrimSuffix(f.name, ext)
				tpl := sxxExxExtractor.ReplaceAllStringFunc(tBase, func(string) string { return "###SXXEXX###" })
				if idx := strings.Index(tpl, "###SXXEXX###"); idx >= 0 {
					template = tpl
				}
				break
			}
		}
		if template == "" {
			continue
		}

		for _, f := range files {
			if f.isDir {
				continue
			}
			m := targetRenamePattern.FindStringSubmatch(f.name)
			if m == nil {
				continue
			}
			sxxexx := m[1]
			ext := pathExt(f.name)
			newName := strings.Replace(template, "###SXXEXX###", sxxexx, 1) + ext
			if f.name == newName {
				continue
			}

			oldPath := joinPath(dir, f.name)
			stats.scanned++
			if err := s.rename(oldPath, newName); err != nil {
				stats.failed++
				s.logf("error", "重命名纯SxxExx文件失败 %s: %v", f.name, err)
			} else {
				s.logf("info", "重命名纯SxxExx文件: %s -> %s", f.name, newName)
			}
		}
	}
	return stats
}

// containsExcludedKeyword 对应脚本 contains_excluded_keyword（EXCLUDED_KEYWORDS）。
// 在监控处理中默认排除的关键字。当前为空集合，保留接口以便后续扩展。
func containsExcludedKeyword(text string) bool {
	_ = text
	return false
}
