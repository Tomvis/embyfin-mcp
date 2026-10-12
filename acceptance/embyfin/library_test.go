//go:build acceptance

package embyfin_test

import (
	"context"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// The libraries are listed with their kind and their folder, and one is found by its name or by its id.
func TestLibrariesAreListedAndFound(t *testing.T) {
	ctx := server(t)

	folders, err := client.VirtualFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []sdkacc.LibraryFixture{filmLibrary, showLibrary} {
		i := slices.IndexFunc(folders, func(f embyfin.VirtualFolder) bool { return f.Name == want.Name })
		if i < 0 {
			t.Errorf("no library named %q is listed: %+v", want.Name, folders)

			continue
		}
		if got := folders[i]; got.CollectionType != want.CollectionType || !slices.Equal(got.Locations, []string{want.Folder}) || got.ItemID == "" {
			t.Errorf("%s = kind %q over %v with id %q, want %s over %s and an id", want.Name, got.CollectionType, got.Locations, got.ItemID, want.CollectionType, want.Folder)
		}

		byName, err := client.FindLibrary(ctx, want.Name)
		if err != nil {
			t.Fatal(err)
		}
		if byID, err := client.FindLibrary(ctx, byName.ItemID); err != nil || byID.Name != want.Name {
			t.Errorf("the library with id %s = %+v, %v; want %s", byName.ItemID, byID, err, want.Name)
		}
	}

	if got, err := client.FindLibrary(ctx, "No Such Library"); err == nil {
		t.Errorf("a library nobody made was found: %+v", got)
	}
}

// A library is made over one folder, renamed, given a second folder, has it taken away, and is removed, and after each step it holds what its
// folders hold. The servers name a library differently to change it, see a new folder on different scans, and one keeps a removed library's items
// until its next scan; none of that reaches the caller.
func TestALibraryRenamedGrownAndRemoved(t *testing.T) {
	ctx := server(t)

	const made, renamed = "Neutral Scratch", "Neutral Renamed"
	sdkacc.LayOut(t, "neutral-one", sdkacc.BulkTitles[0])
	sdkacc.LayOut(t, "neutral-two", sdkacc.BulkTitles[1])

	holds := func(name string, want int) {
		t.Helper()

		var got int
		if !settle(ctx, func() bool {
			got = count(ctx, name, "Movie")

			return got == want
		}) {
			t.Fatalf("%s holds %d films, want %d", name, got, want)
		}
	}
	scan := func(name string) {
		t.Helper()

		folder, err := client.FindLibrary(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.ScanLibrary(ctx, folder); err != nil {
			t.Fatal(err)
		}
	}

	if err := client.CreateLibrary(ctx, embyfin.LibrarySpec{Name: made, CollectionType: "movies", Paths: []string{"/media/neutral-one"}, Refresh: true}); err != nil {
		t.Fatal(err)
	}
	removed := false
	later(t, "removing the scratch library", func(ctx context.Context) error {
		if removed {
			return nil
		}
		for _, name := range []string{made, renamed} {
			if folder, err := client.FindLibrary(ctx, name); err == nil && folder != nil {
				return client.DeleteLibrary(ctx, folder)
			}
		}

		return nil
	})
	holds(made, 1)

	if err := client.RenameLibrary(ctx, library(t, sdkacc.LibraryFixture{Name: made}), renamed); err != nil {
		t.Fatal(err)
	}
	if got, err := client.FindLibrary(ctx, made); err == nil {
		t.Errorf("the library is still found by its old name: %+v", got)
	}
	holds(renamed, 1)

	if err := client.AddLibraryPath(ctx, library(t, sdkacc.LibraryFixture{Name: renamed}), "/media/neutral-two"); err != nil {
		t.Fatal(err)
	}
	if got := library(t, sdkacc.LibraryFixture{Name: renamed}).Locations; !slices.Equal(got, []string{"/media/neutral-one", "/media/neutral-two"}) {
		t.Errorf("the library's folders after one was added = %v", got)
	}
	scan(renamed)
	holds(renamed, 2)

	if err := client.RemoveLibraryPath(ctx, library(t, sdkacc.LibraryFixture{Name: renamed}), "/media/neutral-two"); err != nil {
		t.Fatal(err)
	}
	scan(renamed)
	holds(renamed, 1)

	folder := library(t, sdkacc.LibraryFixture{Name: renamed})
	items, _, err := client.Search(ctx, embyfin.SearchOptions{ParentID: folder.ItemID, IncludeItemTypes: "Movie"})
	if err != nil || len(items) != 1 {
		t.Fatalf("the library's one film = %v, %v", names(items), err)
	}
	if err := client.DeleteLibrary(ctx, folder); err != nil {
		t.Fatal(err)
	}
	removed = true
	if got, err := client.FindLibrary(ctx, renamed); err == nil {
		t.Errorf("the library is still listed after it was removed: %+v", got)
	}

	// its film goes with it
	var last error
	if !sdkacc.Poll(sdkacc.ScanPatience, func() bool {
		_, last = client.ItemByID(ctx, items[0].ID)

		return noItem(last)
	}) {
		t.Errorf("the removed library's film is still read by its id: %v", last)
	}
}
