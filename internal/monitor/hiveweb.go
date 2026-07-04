package monitor

import (
	"strings"
	"time"
)

// addHiveWebTag 对应脚本 add_hiveweb_tag：在路径（目录）中包含 "hiveweb" 的目录下，
// 为视频文件（.mkv/.mp4/.ts）在扩展名前追加 "@HiveWeb" 标签。
// 跳过：文件名已含 hiveweb、纯 SxxExx 格式、含排除关键字。
// since 非零时按子目录 modified 增量剪枝；为零时全量扫描。
func (s *Service) addHiveWebTag(scanDir string, since time.Time) {
	var (
		entries []remoteEntry
		err     error
	)
	if since.IsZero() {
		entries, err = s.walkDir(scanDir, nil)
	} else {
		entries, err = s.walkChanged(scanDir, since, nil)
	}
	if err != nil {
		s.logf("error", "扫描目录失败 %s: %v", scanDir, err)
		return
	}

	for _, f := range entries {
		if f.isDir {
			continue
		}
		// 脚本：if 'hiveweb' not in root.lower(): continue（root 为文件所在目录）
		if !strings.Contains(strings.ToLower(f.absDir), "hiveweb") {
			continue
		}

		if !hasTargetVideoExt(f.name) {
			continue
		}
		if strings.Contains(strings.ToLower(f.name), "hiveweb") {
			continue
		}
		if targetRenamePattern.MatchString(f.name) {
			continue
		}
		if containsExcludedKeyword(f.name) {
			continue
		}

		ext := pathExt(f.name)
		base := strings.TrimSuffix(f.name, ext)
		newName := base + "@HiveWeb" + ext

		oldPath := joinPath(f.absDir, f.name)
		if err := s.rename(oldPath, newName); err != nil {
			s.logf("error", "添加HiveWeb标签失败 %s: %v", f.name, err)
		} else {
			s.logf("info", "添加HiveWeb标签: %s", f.name)
		}
	}
}
