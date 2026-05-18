package sync

import (
	"path"
	"regexp"
	"strings"

	"github.com/user/openlist-sync/internal/openlist"
)

var seasonEpisodeRe = regexp.MustCompile(`^[Ss]\d+[Ee]\d+\.[^.]+$`)

func CompareFilesRecursive(src, dest []openlist.FileEntry, matchMode string) []openlist.FileEntry {
	destSet := make(map[string]struct{}, len(dest))
	for _, f := range dest {
		rel := f.RelPath
		destSet[rel] = struct{}{}
		if strings.HasSuffix(strings.ToLower(rel), ".cas") {
			destSet[rel[:len(rel)-4]] = struct{}{}
		}
	}

	var missing []openlist.FileEntry
	for _, f := range src {
		if _, exists := destSet[f.RelPath]; exists {
			continue
		}
		if matchMode == "smart" {
			if found := smartMatch(f, destSet); found {
				continue
			}
		}
		missing = append(missing, f)
	}
	return missing
}

func smartMatch(src openlist.FileEntry, destSet map[string]struct{}) bool {
	fileName := path.Base(src.RelPath)
	if !seasonEpisodeRe.MatchString(fileName) {
		return false
	}
	ext := path.Ext(fileName)
	seCode := strings.ToLower(fileName[:len(fileName)-len(ext)])

	for key := range destSet {
		keyBase := path.Base(key)
		keyExt := path.Ext(keyBase)
		keyName := keyBase[:len(keyBase)-len(keyExt)]
		if strings.Contains(strings.ToLower(keyName), seCode) {
			return true
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
