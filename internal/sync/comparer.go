package sync

import (
	"path"
	"regexp"
	"strings"

	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
)

var (
	strictSERe  = regexp.MustCompile(`^[Ss](\d+)[Ee](\d+)\.[^.]+$`)
	extractSERe = regexp.MustCompile(`[Ss](\d+)[Ee](\d+)`)
)

func CompareFilesRecursive(src, dest []openlist.FileEntry, matchMode, srcRoot string, pendingFiles map[string]struct{}) []openlist.FileEntry {
	var missing []openlist.FileEntry

	for _, f := range src {
		fileName := path.Base(f.RelPath)

		srcDir, _, fileName := RelPathToCopyDirs(f.RelPath, srcRoot, "")
		if _, ok := pendingFiles[database.CopyJobKey(srcDir, fileName)]; ok {
			continue
		}

		if matchMode == "smart" {
			if smartMatch(fileName, dest) {
				continue
			}
		}

		// smart 模式下非 S##E## 文件也走精确匹配，.cas 文件正确跳过
		if exactMatch(fileName, dest) {
			continue
		}

		missing = append(missing, f)
	}
	return missing
}

func smartMatch(srcFileName string, dest []openlist.FileEntry) bool {
	srcM := strictSERe.FindStringSubmatch(strings.ToLower(srcFileName))
	if srcM == nil {
		return false
	}
	srcCode := "s" + srcM[1] + "e" + srcM[2]

	for _, d := range dest {
		destName := strings.ToLower(path.Base(d.RelPath))
		destM := extractSERe.FindStringSubmatch(destName)
		if destM != nil {
			destCode := "s" + destM[1] + "e" + destM[2]
			if srcCode == destCode {
				return true
			}
		}
	}
	return false
}

func exactMatch(srcFileName string, dest []openlist.FileEntry) bool {
	srcLower := strings.ToLower(srcFileName)
	for _, d := range dest {
		destName := path.Base(d.RelPath)
		destLower := strings.ToLower(destName)

		if destLower == srcLower {
			return true
		}
		if strings.HasSuffix(destLower, ".cas") {
			stripped := destLower[:len(destLower)-4]
			if stripped == srcLower {
				return true
			}
		}
	}
	return false
}

func RelPathToCopyDirs(relPath, srcRoot, dstRoot string) (srcDir, dstDir, fileName string) {
	dir := path.Dir(relPath)
	fileName = path.Base(relPath)
	srcDir = path.Join(srcRoot, dir)
	dstDir = path.Join(dstRoot, dir)
	return
}
