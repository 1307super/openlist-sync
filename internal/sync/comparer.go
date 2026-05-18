package sync

import (
	"path"
	"strings"

	"github.com/user/openlist-sync/internal/openlist"
)

// CompareFilesRecursive returns source entries missing from destination.
// A dest file "a/b/c.mkv.cas" counts as present for "a/b/c.mkv".
func CompareFilesRecursive(src, dest []openlist.FileEntry) []openlist.FileEntry {
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
		if _, exists := destSet[f.RelPath]; !exists {
			missing = append(missing, f)
		}
	}
	return missing
}

// RelPathToCopyDirs splits a relative path like "电视剧/国产剧/北上/S01/1.mkv"
// into dir("电视剧/国产剧/北上/S01") and base("1.mkv"), then joins them
// with the given root to produce srcDir and dstDir.
func RelPathToCopyDirs(relPath, srcRoot, dstRoot string) (srcDir, dstDir, fileName string) {
	dir := path.Dir(relPath)
	fileName = path.Base(relPath)
	srcDir = path.Join(srcRoot, dir)
	dstDir = path.Join(dstRoot, dir)
	return
}
