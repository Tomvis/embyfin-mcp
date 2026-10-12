//go:build acceptance

package tools_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/testenv"
	acc "github.com/katbyte/go-kt/mcp/acctest"

	mediaacc "github.com/katbyte/embyfin-mcp/lib/acceptance"
	"github.com/katbyte/go-kt/test/env"
)

// The one journey that does everything, with the tools alone: an account and a library made from nothing, the naive way; the audits run over what was
// made, and each thing they find put right with the tool they point at, until they find nothing; the films then organised and watched as the new
// account; and all of it taken down again, leaving the server holding what it held before.
//
// What is the same afterwards is what a person would look for: the same accounts, libraries, collections and playlists, and the same films with the
// same names, years and ids. Not the server byte for byte: both number what they make upward and write down what happened.

const (
	journeyLibrary    = "Zzyzx Journey"
	journeyFolder     = "journey-films"
	journeyAccount    = "zzyzx"
	journeyPlaylist   = "Zzyzx Journey List"
	journeyCollection = "Zzyzx Journey Collection"
	// the 2021 film's TMDB id, which the folder names and the nfo beside it, written for the 1984 film, does not
	duneTMDB = "438631"
)

// canLogIn reports whether the server lets a name and a password in.
func canLogIn(t *testing.T, user, password string) bool {
	t.Helper()

	auth := `MediaBrowser Client="embyfin-mcp acceptance", Device="journey", DeviceId="journey", Version="0"`
	body, _ := json.Marshal(map[string]string{"Username": user, "Pw": password})
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, os.Getenv("EMBYFIN_SERVER")+"/Users/AuthenticateByName", bytes.NewReader(body)) //nolint:gosec // the test server's own address
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	req.Header.Set("X-Emby-Authorization", auth)
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // the test server's own address
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// holding is what the journey must leave as it found it, each a sorted list a person could read: the accounts, the libraries that hold media, the
// collections and playlists, and the seeded films by name, year, ids and path.
func holding(t *testing.T) map[string][]string {
	t.Helper()

	out := map[string][]string{}
	for _, u := range acc.Rows(t, suite.Call(t, "user_list", nil)["users"], "users") {
		out["accounts"] = append(out["accounts"], acc.Str(u["name"])+" "+acc.Str(u["id"]))
	}
	for _, l := range acc.Rows(t, suite.Call(t, "library_list", nil)["libraries"], "libraries") {
		if kind := acc.Str(l["collection_type"]); kind == "movies" || kind == "tvshows" || kind == "music" {
			out["libraries"] = append(out["libraries"], fmt.Sprintf("%s %s %v", acc.Str(l["name"]), acc.Str(l["id"]), l["locations"]))
		}
	}
	for _, c := range acc.RowsOf(suite.Call(t, "collection_list", nil)["collections"]) {
		out["collections"] = append(out["collections"], acc.Str(c["name"])+" "+acc.Str(c["id"]))
	}
	for _, p := range acc.RowsOf(suite.Call(t, "playlist_list", nil)["playlists"]) {
		out["playlists"] = append(out["playlists"], acc.Str(p["name"])+" "+acc.Str(p["id"]))
	}
	for _, library := range []string{"Movies", "Messy Movies"} {
		for _, f := range acc.Rows(t, suite.Call(t, "library_items", map[string]any{"library": library, "types": "Movie", "limit": 200})["items"], "items") {
			out["films"] = append(out["films"], fmt.Sprintf("%s (%v) %v %s %s", acc.Str(f["name"]), f["year"], f["metadata_provider_ids"], acc.Str(f["id"]), acc.Str(f["path"])))
		}
	}
	for _, list := range out {
		slices.Sort(list)
	}

	return out
}

// journeyFilms is the journey's library as a person reads it: each film's name, year and TMDB id, by the folder it is in.
func journeyFilms(t *testing.T) (films, ids map[string]string) {
	t.Helper()

	films, ids = map[string]string{}, map[string]string{}
	for _, f := range acc.Rows(t, suite.Call(t, "library_items", map[string]any{"library": journeyLibrary, "types": "Movie", "limit": 50})["items"], "items") {
		folder := filepath.Base(filepath.Dir(acc.Str(f["path"])))
		provider := mediaacc.ObjectOf(f["metadata_provider_ids"])
		films[folder] = fmt.Sprintf("%s %v tmdb %s", acc.Str(f["name"]), f["year"], acc.Str(provider["tmdb"]))
		ids[folder] = acc.Str(f["id"])
	}

	return films, ids
}

// journeyFindings is what audit_all counts in the journey's library, by row, leaving out the rows that count nothing and the one that is no defect
// (what nobody has watched).
func journeyFindings(t *testing.T) (rows map[string]int, total int) {
	t.Helper()

	out := suite.Call(t, "audit_all", map[string]any{"library": journeyLibrary})
	rows = map[string]int{}
	for _, row := range acc.Rows(t, out["audits"], "audits") {
		if n := acc.Num(t, row["findings"], "findings"); n > 0 && acc.Str(row["audit"]) != "audit_unwatched" {
			rows[rowKey(row)] = n
		}
	}

	return rows, acc.Num(t, out["total_findings"], "total_findings")
}

func TestFromNothingToAuditToFix(t *testing.T) {
	password := os.Getenv("EMBYFIN_TEST_PASSWORD")
	data := testenv.DataDir()
	if data == "" || password == "" {
		t.Skip("EMBYFIN_TEST_DATA and EMBYFIN_TEST_PASSWORD are not set")
	}
	before := holding(t)

	// what is taken down if the journey stops part way, so that the tests after it start from the server they expect: each is gone already when it
	// ran to its end
	t.Cleanup(func() {
		for _, c := range acc.RowsOf(suite.Call(t, "collection_list", nil)["collections"]) {
			if acc.Str(c["name"]) == journeyCollection {
				suite.Call(t, "collection_delete", map[string]any{"collection": acc.Str(c["id"])})
			}
		}
		for _, p := range acc.RowsOf(suite.Call(t, "playlist_list", nil)["playlists"]) {
			if acc.Str(p["name"]) == journeyPlaylist {
				suite.Call(t, "playlist_delete", map[string]any{"playlist": acc.Str(p["id"])})
			}
		}
		if removeLibraryIfThere(t, journeyLibrary) {
			settleScan(t, func() error { return suite.WaitForExpectedScan(isJellyfin()) })
		}
		for _, u := range acc.Rows(t, suite.Call(t, "user_list", nil)["users"], "users") {
			if acc.Str(u["name"]) == journeyAccount {
				suite.Call(t, "user_delete", map[string]any{"user": journeyAccount, "confirm": true})
			}
		}
		if err := os.RemoveAll(filepath.Join(data, journeyFolder)); err != nil {
			t.Error(err)
		}
	})

	// --- from nothing: an account

	made := suite.Call(t, "user_create", map[string]any{"name": journeyAccount, "password": password})
	if acc.Str(made["name"]) != journeyAccount || acc.Str(made["id"]) == "" || acc.BoolOf(made["admin"]) || !acc.BoolOf(made["all_libraries"]) {
		t.Fatalf("user_create = %v, want an ordinary account that sees every library", made)
	}
	if raw, _ := json.Marshal(made); strings.Contains(string(raw), password) {
		t.Errorf("user_create answered with the password: %s", raw)
	}
	if !canLogIn(t, journeyAccount, password) || canLogIn(t, journeyAccount, "") || canLogIn(t, journeyAccount, password+"x") {
		t.Errorf("the new account logs in with its password: %v, with none: %v, with another: %v; want only the first", canLogIn(t, journeyAccount, password), canLogIn(t, journeyAccount, ""), canLogIn(t, journeyAccount, password+"x"))
	}
	if msg := suite.CallErr(t, "user_create", map[string]any{"name": strings.ToUpper(journeyAccount), "password": password}); !strings.Contains(msg, "exists") || !strings.Contains(msg, "Nothing was made") {
		t.Errorf("a second account by the name in capitals = %q", msg)
	}

	// --- a library, the naive way: films copied in as they came. One is as it should be; one is there twice, under two folder names; one has no
	// overview and no poster; one has another film's nfo beside it

	root := filepath.Join(data, journeyFolder)
	clean, messy := filepath.Join(data, "movies"), filepath.Join(data, "messy-movies")
	// every fixture poster is the one picture, and this copy of it is the one no test replaces: Emby takes the poster from beside a film that an
	// earlier test matched again, so a film's own folder cannot be counted on to still have one
	poster := filepath.Join(messy, "Blade Runner (1982)", "poster.jpg")
	for to, from := range map[string]string{
		"Blade Runner (1982)/Blade Runner (1982).mp4":               filepath.Join(clean, "Blade Runner (1982)", "Blade Runner (1982).mp4"),
		"Blade Runner (1982)/movie.nfo":                             filepath.Join(clean, "Blade Runner (1982)", "movie.nfo"),
		"Blade Runner (1982)/poster.jpg":                            poster,
		"Alien (1979)/Alien (1979).mp4":                             filepath.Join(clean, "Alien (1979)", "Alien (1979).mp4"),
		"Alien (1979)/movie.nfo":                                    filepath.Join(clean, "Alien (1979)", "movie.nfo"),
		"Alien (1979)/poster.jpg":                                   poster,
		"Alien (1979) Directors Cut/Alien (1979) Directors Cut.mp4": filepath.Join(clean, "Alien (1979)", "Alien (1979).mp4"),
		"Alien (1979) Directors Cut/movie.nfo":                      filepath.Join(messy, "Alien (1979) Directors Cut", "movie.nfo"),
		"Alien (1979) Directors Cut/poster.jpg":                     poster,
		"Arrival (2016)/Arrival (2016).mp4":                         filepath.Join(clean, "Arrival (2016)", "Arrival (2016).mp4"),
		"Arrival (2016)/movie.nfo":                                  filepath.Join(messy, "Arrival (2016)", "movie.nfo"),
		"Dune (2021)/Dune (2021).mp4":                               filepath.Join(clean, "Dune (2021)", "Dune (2021).mp4"),
		"Dune (2021)/movie.nfo":                                     filepath.Join(messy, "Dune (2021)", "movie.nfo"),
		"Dune (2021)/poster.jpg":                                    poster,
	} {
		raw, err := os.ReadFile(from) //nolint:gosec // a fixture under the test data dir
		if err != nil {
			t.Fatal(err)
		}
		env.Mkdir(t, data, filepath.Dir(filepath.Join(root, to)))
		env.WriteFile(t, filepath.Join(root, to), raw)
	}

	// no nfos saved, so what an edit changes is the server's alone and the files stay as they were staged
	library := suite.Call(t, "library_create", map[string]any{"name": journeyLibrary, "type": "movies", "paths": []any{"/media/" + journeyFolder}, "scan": true, "save_nfo": false})
	if acc.Str(library["name"]) != journeyLibrary || acc.Str(library["collection_type"]) != "movies" {
		t.Fatalf("library_create = %v", library)
	}
	if err := waitForItems(journeyLibrary, 5); err != nil {
		t.Fatal(err)
	}
	settleScan(t, func() error { return suite.WaitForExpectedScan(isJellyfin()) })

	// the account is given the new library and no other
	narrowed := suite.Call(t, "user_edit", map[string]any{"user": journeyAccount, "libraries": []any{journeyLibrary}})
	if acc.BoolOf(narrowed["all_libraries"]) || strings.Join(acc.Texts(narrowed["libraries"]), ",") != journeyLibrary {
		t.Errorf("user_edit = %v, want the account seeing %s alone", narrowed, journeyLibrary)
	}
	if got := suite.Call(t, "user_get", map[string]any{"user": journeyAccount}); acc.BoolOf(got["all_libraries"]) || strings.Join(acc.Texts(got["libraries"]), ",") != journeyLibrary {
		t.Errorf("user_get = %v, want the account seeing %s alone", got, journeyLibrary)
	}

	// what the server made of the folders: the films with what the nfo beside each says, right or wrong
	films, ids := journeyFilms(t)
	if want := map[string]string{
		"Blade Runner (1982)":        "Blade Runner 1982 tmdb 78",
		"Alien (1979)":               "Alien 1979 tmdb 348",
		"Alien (1979) Directors Cut": "Alien 1979 tmdb 348",
		"Arrival (2016)":             "Arrival 2016 tmdb 329865",
		"Dune (2021)":                "Dune 1984 tmdb 841",
	}; fmt.Sprint(films) != fmt.Sprint(want) {
		t.Fatalf("the new library holds %v, want %v", films, want)
	}

	// --- audit: the four things wrong with it, and nothing else. The film held twice is one title with two versions on Emby, which joins them, and
	// two entries sharing an id on Jellyfin, which does not

	twice := "audit_multiple_versions"
	if isJellyfin() {
		twice = "audit_duplicates"
	}
	rows, total := journeyFindings(t)
	if want := map[string]int{rowOverview: 1, rowPoster: 1, "audit_file_path": 1, twice: 1}; fmt.Sprint(rows) != fmt.Sprint(want) || total != 4 {
		t.Fatalf("audit_all on the new library = %v (total %d), want %v (total 4); it finds no poster for %v and no overview for %v", rows, total, want, findings(t, missing(t, "poster", map[string]any{"library": journeyLibrary})), findings(t, missing(t, "overview", map[string]any{"library": journeyLibrary})))
	}

	// --- fix, each with the tool its audit points at

	// no overview: written in, here the one the same film has in the library that was matched
	arrival := ids["Arrival (2016)"]
	if got := findings(t, missing(t, "overview", map[string]any{"library": journeyLibrary})); !slices.Equal(got, []string{"Arrival"}) {
		t.Fatalf("audit_missing_metadata overview = %v, want Arrival", got)
	}
	matched := findItem(t, "Movies", "Movie", "Arrival")
	overview := acc.Str(suite.Call(t, "item_get", map[string]any{"id": matched})["overview"])
	if overview == "" {
		t.Fatal("the matched Arrival has no overview to copy")
	}
	suite.Call(t, "item_edit", map[string]any{"ids": []any{arrival}, "overview": overview})

	// no poster: one of the matched film's candidates, set by its address
	candidates := acc.Rows(t, suite.Call(t, "item_artwork", map[string]any{"id": matched, "type": "Primary", "limit": 1})["candidates"], "candidates")
	if len(candidates) == 0 {
		t.Fatal("no poster candidates for Arrival")
	}
	suite.Call(t, "item_artwork_set", map[string]any{"id": arrival, "url": acc.Str(candidates[0]["url"])})
	if !acc.Eventually(func() bool {
		return len(findings(t, missing(t, "poster", map[string]any{"library": journeyLibrary}))) == 0
	}) {
		t.Error("after item_artwork_set Arrival still has no poster")
	}

	// the wrong film's nfo: the path says 2021 and the metadata 1984, and the fix is the identity, then the year the match does not set with the
	// fetchers off
	dune := ids["Dune (2021)"]
	wrong := acc.Rows(t, suite.Call(t, "audit_file_path", map[string]any{"library": journeyLibrary})["findings"], "findings")
	if len(wrong) != 1 || acc.Str(wrong[0]["id"]) != dune || !strings.Contains(strings.Join(acc.Texts(wrong[0]["problems"]), ";"), "path says 2021, metadata says 1984") {
		t.Fatalf("audit_file_path = %v, want Dune's year", wrong)
	}
	candidate := -1
	for i, c := range acc.Rows(t, suite.Call(t, "item_identify", map[string]any{"id": dune, "kind": "movie", "year": 2021})["candidates"], "candidates") {
		if provider := mediaacc.ObjectOf(c["metadata_provider_ids"]); candidate < 0 && acc.Str(provider["tmdb"]) == duneTMDB {
			candidate = i
		}
	}
	if candidate < 0 {
		t.Fatal("the 2021 Dune is not among item_identify's candidates")
	}
	suite.Call(t, "item_identify_apply", withCandidateIDs(t, map[string]any{"id": dune, "kind": "movie", "candidate": candidate, "year": 2021}))
	suite.Call(t, "item_edit", map[string]any{"ids": []any{dune}, "year": 2021})

	// held twice: the copy that should not be there goes, files and all, and the other stays as it was
	extra, kept := ids["Alien (1979) Directors Cut"], env.FilesUnder(t, filepath.Join(root, "Alien (1979)"))
	if msg := suite.CallErr(t, "item_delete", map[string]any{"id": extra}); !strings.Contains(msg, "Directors Cut") {
		t.Errorf("item_delete without confirm = %q, want it to name what it would remove", msg)
	}
	suite.Call(t, "item_delete", map[string]any{"id": extra, "confirm": true})
	env.StillOnDisk(t, data, kept, "the delete of the other copy")
	if _, err := os.Stat(filepath.Join(root, "Alien (1979) Directors Cut", "Alien (1979) Directors Cut.mp4")); !os.IsNotExist(err) {
		t.Errorf("the deleted copy's file is still on disk: %v", err)
	}

	// --- audit again: nothing, now and after a scan has read the folders again

	nothing := func(when string) {
		t.Helper()

		var rows map[string]int
		var total int
		if !acc.Eventually(func() bool {
			rows, total = journeyFindings(t)
			return total == 0 && len(rows) == 0
		}) {
			t.Errorf("%s audit_all on the library = %v (total %d), want nothing", when, rows, total)
		}
	}
	nothing("after the fixes")
	if err := scanUntil(journeyLibrary, 4); err != nil {
		t.Fatal(err)
	}
	settleScan(t, func() error { return suite.WaitForExpectedScan(isJellyfin()) })
	nothing("after a scan")

	// the same films, with the right information now
	films, ids = journeyFilms(t)
	if want := map[string]string{
		"Blade Runner (1982)": "Blade Runner 1982 tmdb 78",
		"Alien (1979)":        "Alien 1979 tmdb 348",
		"Arrival (2016)":      "Arrival 2016 tmdb 329865",
		"Dune (2021)":         "Dune 2021 tmdb " + duneTMDB,
	}; fmt.Sprint(films) != fmt.Sprint(want) {
		t.Errorf("after the fixes the library holds %v, want %v", films, want)
	}
	if got := acc.Str(suite.Call(t, "item_get", map[string]any{"id": ids["Arrival (2016)"]})["overview"]); got != overview {
		t.Errorf("Arrival's overview = %q, want the one written in", got)
	}

	// --- used: a collection, a playlist of the new account's, and a film it has watched and favourited

	collection := suite.Call(t, "collection_create", map[string]any{"name": journeyCollection, "item_ids": []any{ids["Alien (1979)"], ids["Dune (2021)"]}})
	if acc.Num(t, collection["items"], "items") != 2 {
		t.Errorf("collection_create = %v, want it holding 2", collection)
	}
	if got := names(t, suite.Call(t, "collection_get", map[string]any{"collection": journeyCollection})["items"], "items"); !slices.Equal(got, []string{"Alien", "Dune"}) {
		t.Errorf("collection_get = %v, want Alien and Dune", got)
	}
	playlist := suite.Call(t, "playlist_create", map[string]any{"name": journeyPlaylist, "user": journeyAccount, "item_ids": []any{ids["Blade Runner (1982)"], ids["Arrival (2016)"]}})
	if got := playlistNames(t, acc.Str(playlist["id"]), journeyAccount); !slices.Equal(got, []string{"Blade Runner", "Arrival"}) {
		t.Errorf("the account's playlist holds %v, want Blade Runner then Arrival", got)
	}
	suite.Call(t, "item_set_state", map[string]any{"id": ids["Blade Runner (1982)"], "user": journeyAccount, "watched": true, "favourite": true})
	if stats := suite.Call(t, "user_stats", map[string]any{"user": journeyAccount}); acc.Num(t, stats["movies_watched"], "movies_watched") != 1 || acc.Num(t, stats["favourites"], "favourites") != 1 {
		t.Errorf("user_stats = %v, want one film watched and one favourite", stats)
	}

	// a new password, and the old one no longer lets the account in
	changed := suite.Call(t, "user_edit", map[string]any{"user": journeyAccount, "password": password + "-2"})
	if strings.Join(acc.Texts(changed["changed"]), ",") != "password" || !canLogIn(t, journeyAccount, password+"-2") || canLogIn(t, journeyAccount, password) {
		t.Errorf("user_edit = %v; the account logs in with the new password: %v, with the old: %v", changed, canLogIn(t, journeyAccount, password+"-2"), canLogIn(t, journeyAccount, password))
	}

	// --- and back: each thing in the order that lets it go

	suite.Call(t, "playlist_delete", map[string]any{"playlist": acc.Str(playlist["id"])})
	suite.Call(t, "collection_delete", map[string]any{"collection": journeyCollection})
	if msg := suite.CallErr(t, "library_delete", map[string]any{"library": journeyLibrary}); !strings.Contains(msg, "without confirm=true") {
		t.Errorf("library_delete without confirm = %q", msg)
	}
	removeLibrary(t, journeyLibrary)
	settleScan(t, func() error { return suite.WaitForExpectedScan(isJellyfin()) })
	// nothing of it is left under a folder no library covers
	if !acc.Eventually(func() bool {
		return acc.Num(t, suite.Call(t, "audit_orphans", nil)["total_findings"], "total_findings") == 0
	}) {
		t.Errorf("audit_orphans after the library went = %v", suite.Call(t, "audit_orphans", nil))
	}

	if msg := suite.CallErr(t, "user_delete", map[string]any{"user": journeyAccount}); !strings.Contains(msg, "without confirm=true") || !strings.Contains(msg, "no library") {
		t.Errorf("user_delete without confirm = %q, want it to say the account sees no library now", msg)
	}
	gone := suite.Call(t, "user_delete", map[string]any{"user": journeyAccount, "confirm": true})
	if acc.Str(gone["deleted"]) != journeyAccount || canLogIn(t, journeyAccount, password+"-2") {
		t.Errorf("user_delete = %v, and the account still logs in: %v", gone, canLogIn(t, journeyAccount, password+"-2"))
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	// the server holds what it held before
	after := holding(t)
	for _, what := range []string{"accounts", "libraries", "collections", "playlists", "films"} {
		if !slices.Equal(after[what], before[what]) {
			t.Errorf("the %s after the journey:\n  %s\nbefore it:\n  %s", what, strings.Join(after[what], "\n  "), strings.Join(before[what], "\n  "))
		}
	}
}
