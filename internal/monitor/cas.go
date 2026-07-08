package monitor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// targetExtensions 与脚本中的 TARGET_EXTENSIONS 一致。
var targetExtensions = []string{".mkv", ".mp4", ".ts"}

var (
	// casEpisodeExtractRe 提取 SxxExx 编号，用于主目录 CAS 匹配。
	// 对应脚本 extract_episode_pattern（保留原始集数位数）。
	casEpisodeExtractRe = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)
	// casStandardPureRe 匹配追更目录的纯剧集 .cas：S01E01.mp4.cas
	casStandardPureRe = regexp.MustCompile(`(?i)^(S\d+E\d+)\.(.+?)\.cas$`)
	// casNumberPureRe 匹配追更目录的纯数字 .cas：164 4K.mp4.cas
	casNumberPureRe = regexp.MustCompile(`^(\d+)\s+.+\.([^.]+)\.cas$`)
	// casSxxExxExtractRe 用于识别模板 .cas（含 SxxExx 但非纯剧集标识）。
	casSxxExxExtractRe = regexp.MustCompile(`(?i)S\d+E\d+`)
)

// extractEpisodePattern 对应脚本 extract_episode_pattern：
// 解析出 "SssEE" 形式的编号，并保留原集数位数（S 补零到 2 位，E 保持原位数）。
// 例："adb.S1E2.x" → "S01E2"；"S12E100" → "S12E100"。无匹配返回空。
func extractEpisodePattern(name string) string {
	m := casEpisodeExtractRe.FindStringSubmatch(name)
	if m == nil {
		return ""
	}
	season, _ := strconv.Atoi(m[1])
	episode := m[2]
	episodeDigits := len(episode)
	epNum, _ := strconv.Atoi(episode)
	return fmt.Sprintf("S%02dE%0*d", season, episodeDigits, epNum)
}

func lowerHasSuffixFold(s, suffix string) bool {
	return strings.HasSuffix(strings.ToLower(s), strings.ToLower(suffix))
}

// hasTargetVideoExt 返回 name 是否以目标视频扩展名结尾（大小写不敏感）。
func hasTargetVideoExt(name string) bool {
	for _, ext := range targetExtensions {
		if lowerHasSuffixFold(name, ext) {
			return true
		}
	}
	return false
}

// stripSuffix 大小写不敏感地去掉指定后缀，返回剩余部分。
func stripSuffixFold(name, suffix string) string {
	ln := strings.ToLower(name)
	ls := strings.ToLower(suffix)
	if strings.HasSuffix(ln, ls) {
		return name[:len(name)-len(suffix)]
	}
	return name
}

// findVideoExt 返回 name 命中的目标视频扩展名（小写形式，含点），未命中返回空。
func findVideoExt(name string) string {
	ln := strings.ToLower(name)
	for _, ext := range targetExtensions {
		if strings.HasSuffix(ln, ext) {
			return ext
		}
	}
	return ""
}

// syncMainDirCAS 对应脚本 sync_cas_filenames（主目录）。
// 按目录分组处理：在每个目录内，把 .cas 边车文件名对齐到同目录的视频文件名。
// since 非零时按子目录 modified 增量剪枝；为零时全量扫描。
func (s *Service) syncMainDirCAS(mainDir string, since time.Time) stepStats {
	skip := s.chasingDirNamesAt(mainDir)
	var (
		entries []remoteEntry
		err     error
	)
	if since.IsZero() {
		entries, err = s.walkDir(mainDir, skip)
	} else {
		entries, err = s.walkChanged(mainDir, since, skip)
	}
	if err != nil {
		s.logf("error", "扫描主目录失败 %s: %v", mainDir, err)
		return stepStats{failed: 1}
	}

	var stats stepStats
	byDir := groupByDir(entries)
	for dir, files := range byDir {
		stats = stats.add(s.syncMainDirCASInDir(dir, files))
	}
	return stats
}

// syncMainDirCASInDir 处理单个目录内的 CAS 文件重命名。
func (s *Service) syncMainDirCASInDir(dir string, files []remoteEntry) stepStats {
	var stats stepStats
	// 按视频扩展名分组视频与 .cas 文件
	videosByExt := make(map[string]map[string]string) // ext(lower) -> baseName -> fileName
	for _, ext := range targetExtensions {
		videosByExt[ext] = make(map[string]string)
	}
	type casItem struct {
		fileName string
		baseName string
		casExt   string // 对应的视频扩展名（小写含点）
	}
	var casItems []casItem
	for _, f := range files {
		if f.isDir {
			continue
		}
		if videoExt := findVideoExt(f.name); videoExt != "" {
			baseName := stripSuffixFold(f.name, videoExt)
			videosByExt[videoExt][baseName] = f.name
			continue
		}
		// .cas 文件：判断它是哪个视频扩展名的 cas（xxx.mp4.cas）
		for _, ext := range targetExtensions {
			casSuffix := ext + ".cas"
			if lowerHasSuffixFold(f.name, casSuffix) {
				baseName := stripSuffixFold(f.name, casSuffix)
				casItems = append(casItems, casItem{fileName: f.name, baseName: baseName, casExt: ext})
				break
			}
		}
	}

	for _, ci := range casItems {
		matchingVideos := videosByExt[ci.casExt]
		if len(matchingVideos) == 0 {
			continue
		}

		targetVideoBase := ""
		casEpisode := extractEpisodePattern(ci.baseName)

		switch {
		case len(matchingVideos) == 1:
			for k := range matchingVideos {
				targetVideoBase = k
			}
		case casEpisode != "":
			for videoBase := range matchingVideos {
				videoEpisode := extractEpisodePattern(videoBase)
				if videoEpisode != "" && videoEpisode == casEpisode {
					targetVideoBase = videoBase
					break
				}
			}
		default:
			for videoBase := range matchingVideos {
				if videoBase == ci.baseName {
					targetVideoBase = videoBase
					break
				}
			}
			if targetVideoBase == "" {
				for k := range matchingVideos {
					targetVideoBase = k
					break
				}
			}
		}

		if targetVideoBase == "" {
			continue
		}

		targetCASName := targetVideoBase + ci.casExt + ".cas"
		if ci.fileName == targetCASName {
			continue
		}

		oldPath := joinPath(dir, ci.fileName)
		stats.scanned++
		if err := s.rename(oldPath, targetCASName); err != nil {
			stats.failed++
			s.logf("error", "CAS重命名失败（主目录）%s: %v", ci.fileName, err)
		} else {
			s.logf("info", "CAS重命名（主目录）: %s -> %s", ci.fileName, targetCASName)
		}
	}
	return stats
}

// syncChasingDirCAS 对应脚本 process_chasing_dir_cas_files（追更目录）。
// 在每个目录内：识别纯剧集 .cas（S01E01.mp4.cas 或 164 4K.mp4.cas）和模板 .cas，
// 用模板的 prefix/suffix + 新剧集号构造完整名（保留原集数位数）。
// since 非零时按子目录 modified 增量剪枝；为零时全量扫描。
func (s *Service) syncChasingDirCAS(chasingDir string, since time.Time) stepStats {
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
		stats = stats.add(s.syncChasingDirCASInDir(dir, files))
	}
	return stats
}

func (s *Service) syncChasingDirCASInDir(dir string, files []remoteEntry) stepStats {
	var stats stepStats
	type pureItem struct {
		fileName    string
		newEpisode  string
		videoExtRaw string // 捕获到的扩展名（不含点）
	}
	var pureFiles []pureItem
	var templateFile string

	for _, f := range files {
		if f.isDir {
			continue
		}
		name := f.name
		if !strings.HasSuffix(strings.ToLower(name), ".cas") {
			continue
		}

		// 格式1：标准剧集格式 S01E01.mp4.cas
		if m := casStandardPureRe.FindStringSubmatch(name); m != nil {
			episodeStr := m[1]
			if sm := casEpisodeExtractRe.FindStringSubmatch(episodeStr); sm != nil {
				season, _ := strconv.Atoi(sm[1])
				episodeDigits := len(sm[2])
				epNum, _ := strconv.Atoi(sm[2])
				newEpisode := fmt.Sprintf("S%02dE%0*d", season, episodeDigits, epNum)
				pureFiles = append(pureFiles, pureItem{fileName: name, newEpisode: newEpisode, videoExtRaw: m[2]})
			} else {
				pureFiles = append(pureFiles, pureItem{fileName: name, newEpisode: episodeStr, videoExtRaw: m[2]})
			}
			continue
		}

		// 格式2：纯数字格式 164 4K.mp4.cas
		if m := casNumberPureRe.FindStringSubmatch(name); m != nil {
			numberDigits := len(m[1])
			num, _ := strconv.Atoi(m[1])
			newEpisode := fmt.Sprintf("S01E%0*d", numberDigits, num)
			pureFiles = append(pureFiles, pureItem{fileName: name, newEpisode: newEpisode, videoExtRaw: m[2]})
			continue
		}

		// 模板文件：含 SxxExx 但不是纯剧集标识
		if templateFile == "" && casSxxExxExtractRe.FindString(name) != "" {
			templateFile = name
		}
	}

	if len(pureFiles) == 0 || templateFile == "" {
		return stats
	}

	templateLower := strings.ToLower(templateFile)
	m := casSxxExxExtractRe.FindString(templateLower)
	if m == "" {
		return stats
	}
	// 大小写不敏感地在模板中定位 SxxExx，再按其拆分前后缀。
	oldEpisodeLower := m
	idx := strings.Index(templateLower, oldEpisodeLower)
	prefix := templateFile[:idx]
	suffix := templateFile[idx+len(oldEpisodeLower):]

	// 去掉模板后缀中的 .cas
	if strings.HasSuffix(strings.ToLower(suffix), ".cas") {
		suffix = suffix[:len(suffix)-len(".cas")]
	}

	for _, pi := range pureFiles {
		videoExt := pi.videoExtRaw
		if !strings.HasPrefix(videoExt, ".") {
			videoExt = "." + videoExt
		}

		// 在模板后缀中替换视频扩展名；模板中没有则追加
		tempSuffix := suffix
		replaced := false
		for _, ext := range targetExtensions {
			if strings.Contains(strings.ToLower(tempSuffix), ext) {
				tempSuffix = replaceFirstCI(tempSuffix, ext, videoExt)
				replaced = true
				break
			}
		}
		if !replaced {
			tempSuffix = tempSuffix + videoExt
		}

		newName := prefix + pi.newEpisode + tempSuffix + ".cas"
		if pi.fileName == newName {
			continue
		}

		oldPath := joinPath(dir, pi.fileName)
		stats.scanned++
		if err := s.rename(oldPath, newName); err != nil {
			stats.failed++
			s.logf("error", "CAS重命名失败（追更目录）%s: %v", pi.fileName, err)
		} else {
			s.logf("info", "CAS重命名（追更目录）: %s -> %s", pi.fileName, newName)
		}
	}
	return stats
}

// replaceFirstCI 大小写不敏感地替换第一个匹配子串（保留替换值的大小写）。
func replaceFirstCI(s, oldLower, newStr string) string {
	ls := strings.ToLower(s)
	idx := strings.Index(ls, oldLower)
	if idx < 0 {
		return s
	}
	return s[:idx] + newStr + s[idx+len(oldLower):]
}

// chasingDirNamesAt 返回在 mainDir 下应被跳过的追更目录名集合。
// 脚本逻辑：若某追更目录路径包含在主目录扫描路径中，则跳过。
// 这里简化为：把所有追更目录的 basename 收集起来，主目录根层出现这些名字则跳过。
func (s *Service) chasingDirNamesAt(mainDir string) map[string]bool {
	names := make(map[string]bool)
	for _, d := range s.chasingDirs {
		base := pathBase(d)
		if base != "" {
			names[base] = true
		}
	}
	return names
}
