//go:build acceptance

package tools_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/katbyte/go-kt/mcp/acctest"
	"github.com/katbyte/go-kt/test/env"

	"github.com/katbyte/embyfin-mcp/lib/testenv"
)

// Audits against a shape staged for them and taken away again: a file cut
// short, which the server cannot read, and a show's folder copied under its
// name with the year added. And the one audit held to the clean fixtures.

// fixtureVideo reads one of the files .scripts/testenv.sh made.
func fixtureVideo(t *testing.T, parts ...string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(append([]string{testenv.DataDir()}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

// The fixtures were probed at their scan and never rewritten, and are all
// 720p, so the audit proves its sweep: every one of Shows' episodes
// counted, nothing reported.
func TestAuditQualityTrustsTheFixtures(t *testing.T) {
	out := suite.Call(t, "audit_quality", map[string]any{"library": "Shows"})
	if n := acctest.Num(t, out["items_scanned"], "items_scanned"); n != showEpisodes() {
		t.Errorf("items_scanned = %d, want Shows' %d episodes", n, showEpisodes())
	}
	if n := acctest.Num(t, out["total_findings"], "total_findings"); n != 0 {
		t.Errorf("%d files reported: %v", n, out["findings"])
	}
	if n := acctest.Num(t, out["total_unprobed"], "total_unprobed"); n != 0 {
		t.Errorf("%d files reported unprobed: %v", n, out["unprobed"])
	}
	if n := acctest.Num(t, out["total_replaced"], "total_replaced"); n != 0 {
		t.Errorf("%d files reported replaced: %v", n, out["replaced"])
	}
}

// A file cut short in the copying - the first few kilobytes of an episode -
// is an episode the server holds and cannot read: audit_quality lists it
// among the files it could not judge, and not among its findings, since
// nothing about its picture is known. Staged in the messy Severance as its
// sixth episode. Jellyfin gives the cut file its size and no streams, and
// Emby neither, so a size is not taken for a probe.
func TestAuditQualityCannotReadATruncatedFile(t *testing.T) {
	sev := findItem(t, "Messy Shows", "Series", "Severance")
	file := filepath.Join(testenv.DataDir(), "messy-shows", "Severance", "Season 01", "Severance S01E06.mp4")
	t.Cleanup(func() {
		_ = os.Remove(file)
		scanUntilTrue(t, "Messy Shows", func() bool { there, _ := held(t, sev, 1, 6); return !there })
	})
	audit := func() (unprobed map[string]any, total int, found bool) {
		out := suite.Call(t, "audit_quality", map[string]any{"library": "Messy Shows"})
		_, id := held(t, sev, 1, 6)
		for _, r := range acctest.Rows(t, out["unprobed"], "unprobed") {
			if id != "" && acctest.Str(r["id"]) == id {
				unprobed = r
			}
		}
		for _, f := range acctest.Rows(t, out["findings"], "findings") {
			found = found || id != "" && acctest.Str(f["id"]) == id
		}
		return unprobed, acctest.Num(t, out["total_unprobed"], "total_unprobed"), found
	}
	_, before, _ := audit()

	whole := fixtureVideo(t, "messy-shows", "Severance", "Season 01", "Severance S01E01.mp4")
	env.WriteFile(t, file, whole[:4096])
	scanUntilTrue(t, "Messy Shows", func() bool { there, _ := held(t, sev, 1, 6); return there })

	row, total, found := audit()
	if row == nil || total != before+1 {
		t.Fatalf("audit_quality lists %v unprobed (%d, %d before), want the cut file among them", row, total, before)
	}
	if !strings.HasSuffix(acctest.Str(row["path"]), "/Severance S01E06.mp4") || !strings.Contains(acctest.Str(row["detail"]), "never probed") {
		t.Errorf("the cut file's row = %v", row)
	}
	if found {
		t.Error("audit_quality judges a file it could not read")
	}
}

// A show's folder copied under its name with the year added, the rename a
// tidy-up leaves half done: two folders the name-folding audit keeps apart,
// since a year is more than spelling, and the ids audit puts together, since
// both carry Severance's ids. And both servers list both folders' episodes
// under either entry.
func TestAFolderRenamedWithAYear(t *testing.T) {
	if testenv.DataDir() == "" {
		t.Skip("EMBYFIN_TEST_DATA is not set")
	}
	have := typeCount(t, "Messy Shows", "Series")
	src := filepath.Join(testenv.DataDir(), "messy-shows", "Severance")
	dst := filepath.Join(testenv.DataDir(), "messy-shows", "Severance (2022)")
	t.Cleanup(func() {
		_ = os.RemoveAll(dst)
		if err := scanUntil("Messy Shows", have); err != nil {
			t.Error(err)
		}
	})
	env.CopyTree(t, testenv.DataDir(), src, dst)
	if err := scanUntil("Messy Shows", have+1); err != nil {
		t.Fatal(err)
	}

	for _, g := range acctest.Rows(t, suite.Call(t, "audit_duplicates", map[string]any{"library": "Messy Shows"})["folder_groups"], "folder_groups") {
		for _, s := range acctest.Rows(t, g["series"], "series") {
			if strings.HasPrefix(acctest.Str(s["folder"]), "Severance") {
				t.Errorf("audit_duplicates' folder rule groups the renamed folder: %v", g)
			}
		}
	}

	var paths, ids []string
	groups := acctest.RowsOfAny(suite.Call(t, "audit_duplicates", map[string]any{"library": "Messy Shows"})["groups"])
	for _, g := range groups {
		// The Wire, split the same way, is the other series group
		group := acctest.RowsOf(g)
		if len(group) == 0 || acctest.Str(group[0]["type"]) != "Series" || acctest.Str(group[0]["name"]) != "Severance" {
			continue
		}
		for _, s := range group {
			paths = append(paths, acctest.Str(s["path"]))
			ids = append(ids, acctest.Str(s["id"]))
		}
	}
	if want := []string{"/media/messy-shows/Severance", "/media/messy-shows/Severance (2022)"}; !slices.Equal(acctest.Sorted(paths), want) {
		t.Fatalf("audit_duplicates groups the series at %v, want %v", paths, want)
	}

	// what each entry holds: both servers key a show's episodes by the
	// show's ids, so either entry lists both folders' six - a lookup by one
	// entry can answer with the other's file, and show_episodes_exist names
	// the other entry for it. Emby lists each folder's season featurette
	// among them, which it takes for an episode
	want := 6
	if !isJellyfin() {
		want = 8
	}
	for i, id := range ids {
		if n := len(acctest.Rows(t, suite.Call(t, "library_episodes", map[string]any{"series_id": id})["episodes"], "episodes")); n != want {
			t.Errorf("library_episodes lists %d episodes for the series at %s, want both folders' %d", n, paths[i], want)
		}
		out := suite.Call(t, "show_episodes_exist", map[string]any{"series_id": id, "episodes": []map[string]any{{"season": 1, "episode": 1}}})
		if other := ids[1-i]; !slices.Contains(acctest.Strs(t, out["duplicate_entries"], "duplicate_entries"), other) {
			t.Errorf("show_episodes_exist for the series at %s names %v as its other entries, want %s among them", paths[i], out["duplicate_entries"], other)
		}
	}
}
