//go:build acceptance

package jf_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/pandorest/client"
)

func TestJFPlaylists(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	a, b := jfMovie(t, sdkacc.Alien), jfMovie(t, sdkacc.Aliens)

	created := sdkacc.Must(jfc.Playlist.CreatePlaylist(ctx, jf.CreatePlaylistDto{
		Name: "SDK Playlist", Ids: []jf.ItemID{a.Id}, UserId: adminID, MediaType: jf.MediaTypeVideo, IsPublic: new(true),
	})).Answer
	if created.Id == "" {
		t.Fatal("CreatePlaylist returned no id")
	}
	id := created.Id
	sdkacc.RemoveLater(t, "deleting "+id, func() error { _, err := jfc.Library.DeleteItem(context.WithoutCancel(ctx), jf.ItemID(id)); return err })

	// the playlist's own record and its share list want a user behind the
	// request: with an API key the server answers 400 ("Guid can't be
	// empty") rather than serving them, userId query or not
	if _, err := jfc.Playlist.GetPlaylist(ctx, jf.ItemID(id)); client.StatusCode(err) != 400 {
		t.Errorf("GetPlaylist with an API key = %v, want a 400", err)
	}
	if _, err := jfc.Playlist.GetPlaylistUsers(ctx, jf.ItemID(id)); client.StatusCode(err) != 400 {
		t.Errorf("GetPlaylistUsers with an API key = %v, want a 400", err)
	}
	// and so do the playlist's own update and move, which sdk/embyfin works
	// around with the item update and a remove and re-add
	if _, err := jfc.Playlist.UpdatePlaylist(ctx, jf.ItemID(id), jf.UpdatePlaylistDto{Name: "SDK Renamed"}); client.StatusCode(err) != 400 {
		t.Errorf("UpdatePlaylist with an API key = %v, want a 400", err)
	}
	if _, err := jfc.Playlist.MoveItem(ctx, jf.ItemID(id), a.Id, 0); client.StatusCode(err) != 400 {
		t.Errorf("MoveItem with an API key = %v, want a 400", err)
	}

	items := sdkacc.Must(jfc.Playlist.GetPlaylistItems(ctx, jf.ItemID(id), jf.GetPlaylistItemsOptions{UserId: adminID})).Answer
	if items.TotalRecordCount != 1 || len(items.Items) != 1 || items.Items[0].Id != a.Id || items.Items[0].PlaylistItemId == "" {
		t.Fatalf("GetPlaylistItems = %+v", items)
	}

	if _, err := jfc.Playlist.AddItemToPlaylist(ctx, jf.ItemID(id), jf.AddItemToPlaylistOptions{Ids: []jf.ItemID{b.Id}, UserId: adminID}); err != nil {
		t.Fatal(err)
	}
	items = sdkacc.Must(jfc.Playlist.GetPlaylistItems(ctx, jf.ItemID(id), jf.GetPlaylistItemsOptions{UserId: adminID})).Answer
	if len(items.Items) != 2 || items.Items[1].Id != b.Id {
		t.Errorf("after AddItemToPlaylist = %+v", items.Items)
	}

	// an entry's id is its item's id (Emby numbers them), so a refresh
	// cannot change it
	if string(items.Items[0].PlaylistItemId) != string(a.Id) || string(items.Items[1].PlaylistItemId) != string(b.Id) {
		t.Errorf("entry ids = %s %s, want the item ids %s %s", items.Items[0].PlaylistItemId, items.Items[1].PlaylistItemId, a.Id, b.Id)
	}
	// a removal of an entry the playlist does not hold is answered 204 and
	// changes nothing
	if _, err := jfc.Playlist.RemoveItemFromPlaylist(ctx, jf.ItemID(id), []jf.PlaylistItemID{"00000000000000000000000000000001"}...); err != nil {
		t.Errorf("removing an unknown entry = %v", err)
	}
	if n := len(sdkacc.Must(jfc.Playlist.GetPlaylistItems(ctx, jf.ItemID(id), jf.GetPlaylistItemsOptions{UserId: adminID})).Answer.Items); n != 2 {
		t.Errorf("after removing an unknown entry the playlist holds %d", n)
	}

	// entries are removed by their playlist entry id
	if _, err := jfc.Playlist.RemoveItemFromPlaylist(ctx, jf.ItemID(id), []jf.PlaylistItemID{items.Items[0].PlaylistItemId}...); err != nil {
		t.Fatal(err)
	}
	items = sdkacc.Must(jfc.Playlist.GetPlaylistItems(ctx, jf.ItemID(id), jf.GetPlaylistItemsOptions{UserId: adminID})).Answer
	if len(items.Items) != 1 || items.Items[0].Id != b.Id {
		t.Errorf("after RemoveItemFromPlaylist = %+v", items.Items)
	}

	// the playlist is an item too
	if it := sdkacc.Must(jfc.Library.GetItem(ctx, jf.ItemID(id), adminID)).Answer; it.Type != jf.BaseItemKindPlaylist || it.Name != "SDK Playlist" {
		t.Errorf("the playlist as an item = %+v", it)
	}
}

// TestJFCollections covers a collection's create, add and removal. A library
// scan's refresh of a collection writes its saved members over any added
// meanwhile (sdk/embyfin re-sends for this; the behaviour is in
// api-defs/README.md), so the create waits out a scan first.
//

func TestJFCollections(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	a, b, c := jfMovie(t, sdkacc.Alien), jfMovie(t, sdkacc.Aliens), jfMovie(t, sdkacc.BladeRunner)

	jfScanIdle(ctx, t)
	const name = "SDK Collection"
	created := sdkacc.Must(jfc.Collection.CreateCollection(ctx, jf.CreateCollectionOptions{Name: name, Ids: []jf.ItemID{a.Id, b.Id}})).Answer
	if created.Id == "" {
		t.Fatal("CreateCollection returned no id")
	}
	id := created.Id
	t.Cleanup(func() { jfDeleteCollection(context.WithoutCancel(ctx), t, id) })

	// a collection's children are linked, not parented, and the server only
	// resolves the links for a user: without UserId the list is empty
	members := func() []string {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: jf.ItemID(id), UserId: adminID, SortBy: []jf.ItemSortBy{jf.ItemSortByName}})).Answer
		ids := make([]string, 0, len(res.Items))
		for _, it := range res.Items {
			ids = append(ids, string(it.Id))
		}
		return ids
	}
	// each step waits for what it asked for: the server answers a collection
	// change before it has applied it, and a read straight after can still
	// see the membership from before
	holds := func(within time.Duration, want ...string) ([]string, bool) {
		var got []string
		ok := sdkacc.Poll(within, func() bool {
			got = members()

			return slices.Equal(got, want)
		})
		return got, ok
	}
	// the create's own members land with nothing sent again
	if got, ok := holds(sdkacc.EditPatience, string(a.Id), string(b.Id)); !ok {
		t.Fatalf("after CreateCollection members = %v, want %s and %s", got, sdkacc.Alien, sdkacc.Aliens)
	}
	// Jellyfin loses an add or a removal made while it is still refreshing
	// the collection after the change before (sdk/embyfin re-sends for this),
	// so one that has not landed in a third of the patience is sent once more:
	// the same call, so one that never works still fails
	change := func(step string, send func() error, want ...string) {
		t.Helper()
		if err := send(); err != nil {
			t.Fatal(err)
		}
		if _, ok := holds(sdkacc.EditPatience/3, want...); ok {
			return
		}
		t.Logf("%s did not land; sending it again", step)
		if err := send(); err != nil {
			t.Fatal(err)
		}
		if got, ok := holds(sdkacc.EditPatience, want...); !ok {
			t.Errorf("after %s members = %v, want %v", step, got, want)
		}
	}
	change("AddToCollection", func() error {
		_, err := jfc.Collection.AddToCollection(ctx, jf.ItemID(id), []jf.ItemID{c.Id})
		return err
	}, string(a.Id), string(b.Id), string(c.Id))
	change("RemoveFromCollection", func() error {
		_, err := jfc.Collection.RemoveFromCollection(ctx, jf.ItemID(id), []jf.ItemID{a.Id})
		return err
	}, string(b.Id), string(c.Id))
	// removing an item the collection does not hold is answered 204 and
	// changes nothing, which is why sdk/embyfin checks membership first
	if _, err := jfc.Collection.RemoveFromCollection(ctx, jf.ItemID(id), []jf.ItemID{a.Id}); err != nil {
		t.Errorf("removing a non-member = %v", err)
	}
	if got, ok := holds(sdkacc.EditPatience, string(b.Id), string(c.Id)); !ok {
		t.Errorf("after removing a non-member members = %v", got)
	}
	var item *jf.BaseItemDto
	if !sdkacc.Poll(sdkacc.EditPatience, func() bool {
		item = sdkacc.Must(jfc.Library.GetItem(ctx, jf.ItemID(id), adminID)).Answer

		return item != nil && item.Type == jf.BaseItemKindBoxSet && item.Name == name && item.ChildCount == 2
	}) {
		t.Errorf("the collection as an item = %+v", item)
	}
}

// jfScanIdle waits for the library scan to be idle, so a change about to be
// made is not written over by a scan's refresh.
func jfScanIdle(ctx context.Context, t *testing.T) {
	t.Helper()

	if !sdkacc.Poll(sdkacc.ScanPatience, func() bool { return jfScanTask(ctx, t).State == jf.TaskStateIdle }) {
		t.Fatal("the library scan never went idle")
	}
}

// jfDeleteCollection deletes a collection and makes sure it stays deleted.
// A refresh still queued on the collection when the delete lands saves it
// again (its collection.xml), and the next library scan brings it back, its
// members hidden from /Items listings behind it; so a scan is run after the
// delete, and a collection it brings back is deleted again.
func jfDeleteCollection(ctx context.Context, t *testing.T, id string) {
	t.Helper()

	for range 3 {
		if _, err := jfc.Library.DeleteItem(ctx, jf.ItemID(id)); err != nil && !client.IsNotFound(err) {
			t.Errorf("deleting collection %s: %v", id, err)
			return
		}
		since := jfScanEnded(ctx) //nolint:azproviderlint // read before the refresh queues the scan, not after
		if _, err := jfc.Library.RefreshLibrary(ctx); err != nil {
			t.Errorf("scanning after deleting collection %s: %v", id, err)
			return
		}
		jfWaitForScan(ctx, t, since)
		if back := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{Ids: []jf.ItemID{jf.ItemID(id)}})).Answer; len(back.Items) == 0 {
			return
		}
		t.Logf("collection %s came back after a scan; deleting it again", id)
	}
	t.Errorf("collection %s came back after each of three deletes", id)
}
