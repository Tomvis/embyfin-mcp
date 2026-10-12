//go:build acceptance

package tools_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	acc "github.com/katbyte/go-kt/mcp/acctest"
)

// What user_delete says of what goes with an account and what stays: its playlist is still there afterwards, belonging to nobody, and an account made
// again by the same name is a new one, with a new id and none of the old one's marks.
//
// Jellyfin lists the new one with the old one's marks all the same (12.2), which user_create clears and says it did; Emby hands a new account
// nothing. The day Jellyfin stops, its note is no longer there and this fails, which is when the clearing can go.
func TestADeletedAccountsPlaylistStaysAndItsMarksGo(t *testing.T) {
	password := os.Getenv("EMBYFIN_TEST_PASSWORD")
	if password == "" {
		t.Skip("EMBYFIN_TEST_PASSWORD is not set")
	}
	const account, list = "zzyzxgone", "Zzyzx Left Behind"
	arrival := findItem(t, "Movies", "Movie", "Arrival")
	t.Cleanup(func() {
		for _, u := range acc.Rows(t, suite.Call(t, "user_list", nil)["users"], "users") {
			if acc.Str(u["name"]) == account {
				suite.Call(t, "user_delete", map[string]any{"user": account, "confirm": true})
			}
		}
		for _, p := range acc.RowsOf(suite.Call(t, "playlist_list", nil)["playlists"]) {
			if acc.Str(p["name"]) == list {
				suite.Call(t, "playlist_delete", map[string]any{"playlist": acc.Str(p["id"])})
			}
		}
	})

	first := acc.Str(suite.Call(t, "user_create", map[string]any{"name": account, "password": password, "libraries": []any{"Movies"}})["id"])
	playlist := acc.Str(suite.Call(t, "playlist_create", map[string]any{"name": list, "user": account, "item_ids": []any{arrival}})["id"])
	suite.Call(t, "item_set_state", map[string]any{"id": arrival, "user": account, "watched": true, "favourite": true})
	if stats := suite.Call(t, "user_stats", map[string]any{"user": account}); acc.Num(t, stats["movies_watched"], "movies_watched") != 1 || acc.Num(t, stats["favourites"], "favourites") != 1 {
		t.Fatalf("user_stats before the delete = %v, want one film watched and favourited", stats)
	}

	gone := suite.Call(t, "user_delete", map[string]any{"user": account, "confirm": true})
	if acc.Str(gone["id"]) != first || !strings.Contains(acc.Str(gone["note"]), "playlists are not removed") {
		t.Errorf("user_delete = %v", gone)
	}
	if msg := suite.CallErr(t, "user_get", map[string]any{"user": account}); !strings.Contains(msg, "no user named") {
		t.Errorf("user_get after the delete = %q", msg)
	}
	if canLogIn(t, account, password) {
		t.Error("the deleted account still logs in")
	}

	// its playlist is still listed, and still holds the film
	rows := acc.RowsOf(suite.Call(t, "playlist_list", nil)["playlists"])
	listed := make([]string, 0, len(rows))
	for _, p := range rows {
		listed = append(listed, acc.Str(p["id"]))
	}
	if !slices.Contains(listed, playlist) {
		t.Errorf("the deleted account's playlist %s is not among %v: user_delete says it stays", playlist, listed)
	} else if got := playlistNames(t, playlist, ""); !slices.Equal(got, []string{"Arrival"}) {
		t.Errorf("the playlist left behind holds %v, want Arrival", got)
	}

	// the name again is another account, with nothing of the first one's
	second := suite.Call(t, "user_create", map[string]any{"name": account, "password": password})
	if acc.Str(second["id"]) == first || acc.Str(second["id"]) == "" || !acc.BoolOf(second["all_libraries"]) {
		t.Errorf("the account made again = %v, want a new id (the first was %s) and every library", second, first)
	}
	if note, cleared := acc.Str(second["note"]), "listed with such state on 1 of the items it sees, each cleared again"; strings.Contains(note, cleared) != isJellyfin() || strings.Contains(note, "could not be cleared") {
		t.Errorf("user_create's note for the account made again = %q: on Jellyfin it says the one film it was listed with was cleared (%q), and on Emby nothing of the kind", note, cleared)
	}
	if stats := suite.Call(t, "user_stats", map[string]any{"user": account}); acc.Num(t, stats["movies_watched"], "movies_watched") != 0 || acc.Num(t, stats["favourites"], "favourites") != 0 {
		t.Errorf("user_stats of the account made again = %v, want nothing watched or favourited", stats)
	}
}
