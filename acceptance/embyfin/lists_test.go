//go:build acceptance

package embyfin_test

import (
	"context"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// members is what a collection or a playlist holds once it holds want, in any order, or what it held when patience ran out.
func members(t *testing.T, read func() ([]string, error), want ...string) []string {
	t.Helper()

	slices.Sort(want)
	var got []string
	sdkacc.Poll(sdkacc.EditPatience, func() bool {
		var err error
		if got, err = read(); err != nil {
			t.Fatal(err)
		}
		slices.Sort(got)

		return slices.Equal(got, want)
	})

	return got
}

// collectionName is a name the tool suite's collections already use: both servers look a new collection's name up at TMDB whatever its library's
// fetchers, and that lookup is recorded for this one.
const collectionName = "Zzyzx Just Made"

// A collection is made of one film, given another, loses the first and is deleted, holding after each step what it was told to. A film in a
// collection is still one of its library's films: one server leaves it out of the library's list unless asked not to.
func TestACollectionMadeChangedAndDeleted(t *testing.T) {
	ctx := server(t)
	alien, aliens := film(t, sdkacc.Alien), film(t, sdkacc.Aliens)

	id, err := client.CreateCollection(ctx, collectionName, []string{alien.ID})
	if err != nil {
		t.Fatal(err)
	}
	deleted := false
	later(t, "deleting the collection", func(ctx context.Context) error {
		if deleted {
			return nil
		}
		_, err := client.DeleteCollection(ctx, id)

		return err
	})
	held := func() ([]string, error) { return client.CollectionMembers(ctx, id) }

	if got := members(t, held, alien.ID); !slices.Equal(got, []string{alien.ID}) {
		t.Fatalf("the new collection holds %v, want %s", got, alien.ID)
	}
	if got := count(ctx, filmLibrary.Name, "Movie"); got != filmLibrary.FilmCount() {
		t.Errorf("the library lists %d films with one of them in a collection, want %d", got, filmLibrary.FilmCount())
	}

	if err := client.AddToCollection(ctx, id, []string{aliens.ID}); err != nil {
		t.Fatal(err)
	}
	if got := members(t, held, alien.ID, aliens.ID); len(got) != 2 {
		t.Fatalf("the collection after a second film was added holds %v", got)
	}

	if err := client.RemoveFromCollection(ctx, id, []string{alien.ID}); err != nil {
		t.Fatal(err)
	}
	if got := members(t, held, aliens.ID); !slices.Equal(got, []string{aliens.ID}) {
		t.Fatalf("the collection after the first film was taken out holds %v, want %s", got, aliens.ID)
	}

	if _, err := client.DeleteCollection(ctx, id); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if got, err := client.ItemByID(ctx, id); !noItem(err) {
		t.Errorf("the deleted collection is still read by its id: %v, %v", got, err)
	}
	if _, err := client.ItemByID(ctx, aliens.ID); err != nil {
		t.Errorf("the film the deleted collection held: %v", err)
	}
}

// A playlist is made of two films in order, given a third, renamed, and loses the middle one, and reads back in the order it was put in. The
// servers take a new playlist differently and one will not let a key read or rename one; the caller sees neither.
func TestAPlaylistMadeChangedAndDeleted(t *testing.T) {
	ctx := server(t)
	alien, aliens, blade := film(t, sdkacc.Alien), film(t, sdkacc.Aliens), film(t, sdkacc.BladeRunner)

	id, err := client.CreatePlaylist(ctx, "Neutral Playlist", []string{alien.ID, aliens.ID}, "Video", adminID)
	if err != nil {
		t.Fatal(err)
	}
	later(t, "deleting the playlist", func(ctx context.Context) error { return client.DeleteItem(ctx, id) })

	// in order: what it holds once it holds want, or what it held when patience ran out
	holds := func(want ...string) []string {
		t.Helper()

		var got []string
		sdkacc.Poll(sdkacc.EditPatience, func() bool {
			items, _, rerr := client.PlaylistItems(ctx, id, adminID)
			if rerr != nil {
				t.Fatal(rerr)
			}
			got = ids(items)

			return slices.Equal(got, want)
		})

		return got
	}

	if got, want := holds(alien.ID, aliens.ID), []string{alien.ID, aliens.ID}; !slices.Equal(got, want) {
		t.Fatalf("the new playlist holds %v, want %v", got, want)
	}

	if err := client.AddToPlaylist(ctx, id, []string{blade.ID}, adminID); err != nil {
		t.Fatal(err)
	}
	if got, want := holds(alien.ID, aliens.ID, blade.ID), []string{alien.ID, aliens.ID, blade.ID}; !slices.Equal(got, want) {
		t.Fatalf("the playlist after a third film was added holds %v, want %v", got, want)
	}

	if err := client.RenamePlaylist(ctx, id, adminID, "Neutral Playlist Renamed"); err != nil {
		t.Fatal(err)
	}
	var name string
	if !sdkacc.Poll(sdkacc.EditPatience, func() bool {
		it, rerr := client.ItemByID(ctx, id)
		if rerr != nil {
			t.Fatal(rerr)
		}
		name = it.Name

		return name == "Neutral Playlist Renamed"
	}) {
		t.Errorf("the playlist after it was renamed is called %q", name)
	}

	// an entry is named by its own id and the item it holds: the servers number entries differently, and one renumbers them
	entries, _, err := client.PlaylistItems(ctx, id, adminID)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(entries, func(e embyfin.Item) bool { return e.ID == aliens.ID })
	if i < 0 {
		t.Fatalf("the playlist lists no entry holding %s: %v", sdkacc.Aliens, names(entries))
	}
	if _, err := client.RemoveFromPlaylist(ctx, id, embyfin.EntriesToRemove{EntryIDs: []string{entries[i].PlaylistItemID}, ItemIDs: []string{aliens.ID}}); err != nil {
		t.Fatal(err)
	}
	if got, want := holds(alien.ID, blade.ID), []string{alien.ID, blade.ID}; !slices.Equal(got, want) {
		t.Errorf("the playlist after the middle film was taken out holds %v, want %v", got, want)
	}
}
