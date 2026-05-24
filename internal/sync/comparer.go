package sync

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/user/openlist-sync/internal/database"
	"github.com/user/openlist-sync/internal/openlist"
)

var (
	strictSERe  = regexp.MustCompile(`^[Ss](\d+)[Ee](\d+)\.[^.]+$`)
	extractSERe = regexp.MustCompile(`[Ss](\d+)[Ee](\d+)`)
	pureNumRe   = regexp.MustCompile(`^(\d+)\s+.*\.[^.]+$`)
)

type CompareResult struct {
	Matched []openlist.FileEntry // 目标已存在，跳过
	Missing []openlist.FileEntry // 目标不存在，需要复制
}

func CompareFilesRecursive(src, dest []openlist.FileEntry, matchMode, srcRoot string, pendingFiles map[string]struct{}) CompareResult {
	var result CompareResult

	for _, f := range src {
		fileName := path.Base(f.RelPath)

		srcDir, _, fileName := RelPathToCopyDirs(f.RelPath, srcRoot, "")
		if _, ok := pendingFiles[database.CopyJobKey(srcDir, fileName)]; ok {
			continue
		}

		if matchMode == "smart" {
			if smartMatch(fileName, dest) {
				result.Matched = append(result.Matched, f)
				continue
			}
		}

		if exactMatch(fileName, dest) {
			result.Matched = append(result.Matched, f)
			continue
		}

		result.Missing = append(result.Missing, f)
	}
	return result
}

func smartMatch(srcFileName string, dest []openlist.FileEntry) bool {
	lower := strings.ToLower(srcFileName)

	// S01E195.ext 格式
	srcM := strictSERe.FindStringSubmatch(lower)
	if srcM != nil {
		srcCode := "s" + srcM[1] + "e" + srcM[2]
		return matchDestCode(srcCode, dest)
	}

	// "195 4K.mp4" 格式 → s01e195
	numM := pureNumRe.FindStringSubmatch(lower)
	if numM != nil {
		epNum, _ := strconv.Atoi(numM[1])
		srcCode := fmt.Sprintf("s01e%02d", epNum)
		return matchDestCode(srcCode, dest)
	}

	return false
}

func matchDestCode(srcCode string, dest []openlist.FileEntry) bool {
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
