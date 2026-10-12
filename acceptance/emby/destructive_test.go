//go:build acceptance

package emby_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
	"github.com/katbyte/pandorest/client"
)

// TestEmbyDeleteItem removes a movie the test laid out itself, in a library
// of its own, so nothing the rest of the suite asserts on can disappear.
//

func TestEmbyDeleteItem(t *testing.T) {
	ctx := skipUnlessEmby(t)

	dir := sdkacc.ScratchDir(t)
	t.Cleanup(func() {
		_ = embyDeleteVirtualFolder(context.WithoutCancel(ctx), sdkacc.SDKScratch.Name)
		_ = os.RemoveAll(dir)
	})
	scratchID := embyLibrary(t, sdkacc.SDKScratch)

	res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: scratchID, Recursive: new(true), IncludeItemTypes: "Movie", Fields: "Path,ProductionYear"})).Answer
	if len(res.Items) != 1 || !strings.HasPrefix(res.Items[0].Name, sdkacc.ScratchTitle) || res.Items[0].ProductionYear != 1995 {
		t.Fatalf("the scratch library holds %+v, want %s", res.Items, sdkacc.ScratchTitle)
	}
	id, path := res.Items[0].Id, res.Items[0].Path

	if _, err := embyc.Library.DeleteItemsById(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, id); !client.IsNotFound(err) {
		t.Errorf("fetching after the delete = %v, want a 404", err)
	}
	// the server deleted the file, not only the record
	if _, err := os.Stat(dir + "/" + sdkacc.ScratchTitle + " (1995)/" + sdkacc.ScratchTitle + " (1995).mp4"); !os.IsNotExist(err) {
		t.Errorf("after the delete %s is still on disk (%v)", path, err)
	}
	if n := embyCount(ctx, scratchID, "Movie"); n != 0 {
		t.Errorf("the scratch library still counts %d movies", n)
	}
}

// TestEmbyDeleteItems removes several items in one call (DELETE /Items with
// Ids, the route item_orphans_delete takes), from a library of its own: two
// films at once, then a film beside an id the server does not know, either
// way round.
//

func TestEmbyDeleteItems(t *testing.T) {
	ctx := skipUnlessEmby(t)

	dir := sdkacc.LayOut(t, "sdk-bulk", sdkacc.BulkTitles...)
	t.Cleanup(func() {
		_ = embyDeleteVirtualFolder(context.WithoutCancel(ctx), sdkacc.SDKBulk.Name)
		_ = os.RemoveAll(dir)
	})
	bulkID := embyLibrary(t, sdkacc.SDKBulk)

	// the films by the folder they were laid out in
	films := map[string]emby.BaseItemDto{}
	for _, it := range sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: bulkID, Recursive: new(true), IncludeItemTypes: "Movie", Fields: "Path"})).Answer.Items {
		films[filepath.Base(filepath.Dir(it.Path))] = it
	}
	if len(films) != len(sdkacc.BulkTitles) {
		t.Fatalf("the bulk library holds %d films, want %d: %+v", len(films), len(sdkacc.BulkTitles), films)
	}
	// gone is whether a film's record and its file are both gone
	gone := func(title string) bool {
		_, err := embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, films[title].Id)
		_, statErr := os.Stat(filepath.Join(dir, title, title+".mp4"))
		switch {
		case client.IsNotFound(err) && os.IsNotExist(statErr):
			return true
		case err == nil && statErr == nil:
			return false
		}
		t.Fatalf("%s is half deleted: the read answers %v, the file %v", title, err, statErr)
		return false
	}
	// unknown is an id no item has
	const unknown = "999999999"

	// two at once, and nothing else
	a, b, c, d := sdkacc.BulkTitles[0], sdkacc.BulkTitles[1], sdkacc.BulkTitles[2], sdkacc.BulkTitles[3]
	if _, err := embyc.Library.DeleteItems(ctx, []emby.ItemID{films[a].Id, films[b].Id}); err != nil {
		t.Fatal(err)
	}
	if !gone(a) || !gone(b) || gone(c) || gone(d) {
		t.Fatalf("after deleting %s and %s together: %s gone %t, %s gone %t, %s gone %t, %s gone %t", a, b, a, gone(a), b, gone(b), c, gone(c), d, gone(d))
	}

	// an unknown id beside a known one is passed over, before it or after
	// it: answered 204, and the known one deleted
	if _, err := embyc.Library.DeleteItems(ctx, []emby.ItemID{films[c].Id, unknown}); err != nil {
		t.Errorf("deleting %s and an unknown id = %v", c, err)
	}
	if !gone(c) || gone(d) {
		t.Errorf("after deleting %s beside an unknown id: %s gone %t, %s gone %t", c, c, gone(c), d, gone(d))
	}
	if _, err := embyc.Library.DeleteItems(ctx, []emby.ItemID{unknown, films[d].Id}); err != nil {
		t.Errorf("deleting an unknown id and %s = %v", d, err)
	}
	if !gone(d) {
		t.Errorf("after deleting an unknown id before %s it is still there", d)
	}
	if n := embyCount(ctx, bulkID, "Movie"); n != 0 {
		t.Errorf("the bulk library still counts %d films", n)
	}

	// an unknown id alone is answered the same, and one that is not a
	// number at all is a 500
	if _, err := embyc.Library.DeleteItems(ctx, []emby.ItemID{unknown}); err != nil {
		t.Errorf("deleting only an unknown id = %v", err)
	}
	if _, err := embyc.Library.DeleteItems(ctx, []emby.ItemID{"not-an-id"}); client.StatusCode(err) != 500 {
		t.Errorf("deleting a malformed id = %v, want a 500", err)
	}
}
