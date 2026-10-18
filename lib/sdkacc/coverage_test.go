package sdkacc

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/katbyte/pandorest/client"
	"github.com/katbyte/pandorest/definitions"
)

// The scan finds what embyfin's code calls through each SDK's client.
func TestUsedOperations(t *testing.T) {
	t.Parallel()

	for service, want := range map[string]string{"emby": "GetItems", "jellyfin": "GetItems", "tmdb": "MovieDetails"} {
		used, err := UsedOperations(service)
		if err != nil {
			t.Fatal(err)
		}
		if used[want] == nil || len(used) < 10 {
			t.Errorf("%s: %d operations used, want %s among more than ten", service, len(used), want)
		}
	}
	if _, err := UsedOperations("sonarr"); err == nil {
		t.Error("a service embyfin has no client for was scanned")
	}
}

// Only writes need a live suite to reach them; a call counts for the operation it names, and an excuse must still be needed.
func TestUnreached(t *testing.T) {
	t.Parallel()

	used := map[string]*definitions.Operation{
		"GetItem":     {Name: "GetItem", Method: "GET", Path: "/Items/{itemId}"},
		"DeleteItem":  {Name: "DeleteItem", Method: "DELETE", Path: "/Items/{itemId}"},
		"PostItem":    {Name: "PostItem", Method: "POST", Path: "/Items/{itemId}"},
		"PostRefresh": {Name: "PostRefresh", Method: "POST", Path: "/Items/{itemId}/Refresh"},
		"PostScan":    {Name: "PostScan", Method: "POST", Path: "/Library/Refresh"},
	}
	writes := Writes(used)
	if writes["GetItem"] != nil || len(writes) != 4 {
		t.Errorf("Writes = %v, want the four that are not GETs", slices.Sorted(maps.Keys(writes)))
	}

	calls := []client.Call{{Method: "DELETE", Path: "/Items/1", Operation: "DeleteItem"}, {Method: "POST", Path: "/Library/Refresh", Operation: "PostScan"}}
	excused := map[string]string{"PostItem": "another suite sends it", "PostScan": "stale", "GetItem": "a GET", "PostNothing": "names nothing"}

	got := Unreached(writes, calls, excused)
	want := []string{
		"PostRefresh (POST /Items/{itemId}/Refresh) is sent by embyfin's code, and no live test reached it",
		"PostScan (POST /Library/Refresh) was reached, so drop its excuse: stale",
		"the excuse for GetItem names nothing embyfin's code sends",
		"the excuse for PostNothing names nothing embyfin's code sends",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("Unreached =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
