//go:build acceptance

package embyfin_test

import (
	"testing"

	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// The server says who it is, and the client which API document it was built for: the one for the server it is talking to.
func TestTheServerAndTheClientSayWhatTheyAre(t *testing.T) {
	ctx := server(t)

	info, err := client.SystemInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.ServerName == "" || info.Version == "" {
		t.Errorf("the server's name and version = %q, %q", info.ServerName, info.Version)
	}

	want := embyfin.EmbyAPIVersion
	if client.Backend() == embyfin.Jellyfin {
		want = embyfin.JellyfinAPIVersion
	}
	if got := client.APIVersion(); got != want {
		t.Errorf("APIVersion = %q, want %q", got, want)
	}
}

// The server's own disk is read the same way on both: a folder it has is there with what it holds, and one it lacks is not found rather than an
// error.
func TestTheServersOwnFolders(t *testing.T) {
	ctx := server(t)

	if there, err := client.PathExists(ctx, filmLibrary.Folder); err != nil || !there {
		t.Errorf("PathExists(%s) = %v, %v", filmLibrary.Folder, there, err)
	}
	if there, err := client.PathExists(ctx, "/media/no-such-folder"); err != nil || there {
		t.Errorf("PathExists of a folder that is not there = %v, %v", there, err)
	}

	entries, found, err := client.ListFolder(ctx, filmLibrary.Folder)
	if err != nil || !found {
		t.Fatalf("ListFolder(%s) = found %v, %v", filmLibrary.Folder, found, err)
	}
	folders := 0
	for _, e := range entries {
		if e.IsDir {
			folders++
		}
	}
	if folders != filmLibrary.Movies {
		t.Errorf("%s holds %d folders, want one for each of the %d films: %v", filmLibrary.Folder, folders, filmLibrary.Movies, entries)
	}

	if _, found, err := client.ListFolder(ctx, "/media/no-such-folder"); err != nil || found {
		t.Errorf("ListFolder of a folder that is not there = found %v, %v", found, err)
	}
}

// The server's own counts are carried as it gives them. The films, series and episodes it counts are the ones a count of each kind finds; its total
// of everything is, on Jellyfin, what a count with no filter finds, and Emby leaves it at none while such a count finds plenty.
func TestTheServersOwnCounts(t *testing.T) {
	ctx := server(t)

	counts, err := client.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}

	everything, err := client.Count(ctx, embyfin.SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	for kind, got := range map[string]int{"Movie": counts.MovieCount, "Series": counts.SeriesCount, "Episode": counts.EpisodeCount} {
		n, err := client.Count(ctx, embyfin.SearchOptions{IncludeItemTypes: kind})
		if err != nil {
			t.Fatal(err)
		}
		if got != n || n == 0 {
			t.Errorf("the server counts %d of %s, and a count of them finds %d", got, kind, n)
		}
	}

	switch {
	case client.Backend() == embyfin.Emby && (counts.ItemCount != 0 || everything == 0):
		t.Errorf("Emby's own total = %d, and a count of everything finds %d; it was left at 0 on 4.10.1, so what ItemCounts says of it no longer holds", counts.ItemCount, everything)
	case client.Backend() == embyfin.Jellyfin && (counts.ItemCount != everything || everything == 0):
		t.Errorf("Jellyfin's own total = %d, and a count of everything finds %d", counts.ItemCount, everything)
	}
}
