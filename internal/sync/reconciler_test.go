package sync

import (
	"testing"

	"github.com/user/openlist-sync/internal/openlist"
)

func TestPendingCopyReadyRequiresCopiedFileNameForPureNumberSource(t *testing.T) {
	dest := []openlist.FileEntry{{RelPath: "Show.S01E195.2160p.mkv"}}
	if pendingCopyReady("195 4K.mkv", dest) {
		t.Fatalf("pending copy should not be considered ready by an existing smart-match target only")
	}
}

func TestPendingCopyReadyAcceptsOriginalCasOrAlreadyRenamedFile(t *testing.T) {
	cases := []struct {
		name        string
		destName    string
		wantCurrent string
		wantRenamed bool
	}{
		{name: "original", destName: "195 4K.mkv", wantCurrent: "195 4K.mkv"},
		{name: "original cas", destName: "195 4K.mkv.cas", wantCurrent: "195 4K.mkv.cas"},
		{name: "renamed", destName: "S01E195.mkv", wantCurrent: "S01E195.mkv", wantRenamed: true},
		{name: "renamed cas", destName: "S01E195.mkv.cas", wantCurrent: "S01E195.mkv.cas", wantRenamed: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dest := []openlist.FileEntry{{RelPath: tc.destName}}
			if !pendingCopyReady("195 4K.mkv", dest) {
				t.Fatalf("pending copy should be ready when %q exists", tc.destName)
			}
			gotCurrent, gotRenamed := pendingCopyCurrentName("195 4K.mkv", dest)
			if gotCurrent != tc.wantCurrent || gotRenamed != tc.wantRenamed {
				t.Fatalf("pendingCopyCurrentName() = (%q, %v), want (%q, %v)", gotCurrent, gotRenamed, tc.wantCurrent, tc.wantRenamed)
			}
		})
	}
}

func TestRenameTargetPureNumberQualityName(t *testing.T) {
	got := RenameTarget("195 4K.mkv")
	want := "S01E195.mkv"
	if got != want {
		t.Fatalf("RenameTarget() = %q, want %q", got, want)
	}
}
