//go:build acceptance

package embyfin_test

import (
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// A library's films come back with what their files and nfo say of them: the title, the year and the ids at TMDB and IMDb.
func TestTheFilmsAsTheirFilesDescribeThem(t *testing.T) {
	ctx := server(t)

	items, total, err := client.Search(ctx, embyfin.SearchOptions{ParentID: library(t, filmLibrary).ItemID, IncludeItemTypes: "Movie"})
	if err != nil {
		t.Fatal(err)
	}
	if total != len(sdkacc.Movies) || len(items) != len(sdkacc.Movies) {
		t.Fatalf("the library lists %d films and says it holds %d, want %d: %v", len(items), total, len(sdkacc.Movies), names(items))
	}
	for _, want := range sdkacc.Movies {
		i := slices.IndexFunc(items, func(it embyfin.Item) bool { return it.Name == want.Title })
		if i < 0 {
			t.Errorf("no film named %q: %v", want.Title, names(items))

			continue
		}
		if got := &items[i]; got.ProductionYear != want.Year || providerID(got, "tmdb") != want.TMDB || providerID(got, "imdb") != want.IMDB || got.Path == "" {
			t.Errorf("%s = year %d, ids %v, path %q; want %d, tmdb %s, imdb %s and a path", want.Title, got.ProductionYear, got.ProviderIDs, got.Path, want.Year, want.TMDB, want.IMDB)
		}
	}
}

// A film is found by its id at a provider on both servers, though only one of them can be asked that question; an id no film carries finds none.
func TestAFilmFoundByItsProviderID(t *testing.T) {
	ctx := server(t)

	for _, want := range sdkacc.Movies[:3] {
		items, _, err := client.ItemsByProviderID(ctx, "tmdb", want.TMDB, "Movie")
		if err != nil {
			t.Fatal(err)
		}
		if got := names(items); !slices.Equal(got, []string{want.Title}) {
			t.Errorf("the films with tmdb id %s = %v, want %s alone", want.TMDB, got, want.Title)
		}
	}

	items, _, err := client.ItemsByProviderID(ctx, "tmdb", "999999999", "Movie")
	if err != nil || len(items) != 0 {
		t.Errorf("the films with a tmdb id none carries = %v, %v", names(items), err)
	}
}

// One item is read by its id, and an id no item has is said to be that, whatever its shape: never answered with some other item, the whole
// library, or the server's own error for an id it cannot read.
func TestOneItemByItsID(t *testing.T) {
	ctx := server(t)
	alien := film(t, sdkacc.Alien)

	got, err := client.ItemByID(ctx, alien.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != alien.ID || got.Name != sdkacc.Alien || got.Type != "Movie" {
		t.Errorf("ItemByID(%s) = %s %q (%s)", alien.ID, got.ID, got.Name, got.Type)
	}

	for _, id := range []string{"no-such-id", "999999999", "0123456789abcdef0123456789abcdef"} {
		if got, err := client.ItemByID(ctx, id); !noItem(err) {
			t.Errorf("ItemByID(%q) = %v, %v; want the error for an id no item has", id, got, err)
		}
	}
}

// The films like another are asked for in a user's name on both servers; one of them refuses the question without.
func TestFilmsLikeAnother(t *testing.T) {
	ctx := server(t)

	if _, err := client.Similar(ctx, film(t, sdkacc.Alien).ID, adminID, 5); err != nil {
		t.Error(err)
	}
}
