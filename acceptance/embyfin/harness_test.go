//go:build acceptance

package embyfin_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/lib/testenv"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// The libraries the suite creates: the clean films and the clean shows, read from their files and nfo sidecars alone.
var (
	filmLibrary = sdkacc.LibraryFixture{Name: "Neutral Movies", CollectionType: "movies", Folder: "/media/movies", Movies: 10}
	showLibrary = sdkacc.LibraryFixture{Name: "Neutral Shows", CollectionType: "tvshows", Folder: "/media/shows", Series: 4, Episodes: 11}
)

var (
	// client is set when a server is up; without it every test skips
	client *embyfin.Client

	adminID, aliceID = sdkacc.AdminID(), sdkacc.UserID()

	librariesOnce sync.Once
	errLibraries  error
)

// runSuite connects, runs the tests, removes the libraries they created, and returns the exit code.
func runSuite(m *testing.M) int {
	if !testenv.Configured() {
		return m.Run() // every test skips
	}

	var err error
	if client, err = embyfin.New(embyfin.Backend(testenv.Backend()), os.Getenv("EMBYFIN_SERVER"), os.Getenv("EMBYFIN_TOKEN")); err != nil {
		fmt.Fprintln(os.Stderr, "embyfin client:", err)

		return 1
	}

	return sdkacc.Run(m, testenv.Backend(), removeLibraries)
}

// removeLibraries deletes the libraries the suite created, after the last test. EMBYFIN_TEST_KEEP=1 leaves them in place, to look at what a run
// left behind.
func removeLibraries() {
	if os.Getenv("EMBYFIN_TEST_KEEP") != "" {
		return
	}

	ctx := context.Background()
	for _, l := range []sdkacc.LibraryFixture{filmLibrary, showLibrary} {
		folder, err := client.FindLibrary(ctx, l.Name)
		if err != nil || folder == nil {
			continue
		}
		if err := client.DeleteLibrary(ctx, folder); err != nil {
			fmt.Fprintf(os.Stderr, "removing %s: %v\n", l.Name, err)
		}
	}
}

// server skips unless a server is up, makes the two libraries the first time it is called, waits for both to be scanned, and returns the test's
// context.
func server(t *testing.T) context.Context {
	t.Helper()

	if client == nil {
		t.Skip("no server; run: eval \"$(EMBYFIN_TEST_BACKEND=jellyfin .scripts/testenv.sh up)\"")
	}

	librariesOnce.Do(func() {
		ctx := context.Background()
		for _, l := range []sdkacc.LibraryFixture{filmLibrary, showLibrary} {
			if err := client.CreateLibrary(ctx, embyfin.LibrarySpec{Name: l.Name, CollectionType: l.CollectionType, Paths: []string{l.Folder}, Refresh: true}); err != nil {
				errLibraries = fmt.Errorf("creating %s: %w", l.Name, err)

				return
			}
		}

		// a library is not ready when it lists the right number of items: one server lists a film by its folder's name first and reads what its
		// nfo says of it after, so the wait is for the titles themselves
		var films, shows []string
		var episodes int
		if !settle(ctx, func() bool {
			films, shows, episodes = titles(ctx, filmLibrary.Name, "Movie"), titles(ctx, showLibrary.Name, "Series"), count(ctx, showLibrary.Name, "Episode")

			return slices.Equal(films, filmTitles()) && slices.Equal(shows, showTitles()) && episodes == showLibrary.EpisodeCount()
		}) {
			errLibraries = fmt.Errorf("the scans settled on the films %v, the series %v and %d episodes; want %v, %v and %d", films, shows, episodes, filmTitles(), showTitles(), showLibrary.EpisodeCount())
		}
	})
	if errLibraries != nil {
		t.Fatal(errLibraries)
	}

	return t.Context()
}

// settle waits for check to hold with no scan running, and asks for a scan of every library again whenever the server goes idle short of it: one
// ask is not always enough. Emby 4.10.1 was seen to cancel the scan of a library as the next one was made and never run the scan queued in its
// place, so that neither library was read; the tool suite's harness asks again for the same reason.
func settle(ctx context.Context, check func() bool) bool {
	deadline := time.Now().Add(sdkacc.ScanPatience)
	for {
		if sdkacc.Poll(45*time.Second, func() bool { return idle(ctx) && check() }) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		if idle(ctx) {
			if err := client.RefreshLibrary(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "asking for a scan again:", err)
			}
		}
	}
}

// idle says that no scan is running, as far as the server says.
func idle(ctx context.Context) bool {
	running, err := client.ScansRunning(ctx)

	return err == nil && len(running) == 0
}

// count is how many items of a kind a library lists, -1 when it could not be asked.
func count(ctx context.Context, library, kind string) int {
	folder, err := client.FindLibrary(ctx, library)
	// a library just made is listed before it has an id, and a search with no id would count every library's
	if err != nil || folder == nil || folder.ItemID == "" {
		return -1
	}
	_, total, err := client.Search(ctx, embyfin.SearchOptions{ParentID: folder.ItemID, IncludeItemTypes: kind, Limit: 1})
	if err != nil {
		return -1
	}

	return total
}

// titles is the names of a library's items of a kind, sorted; nil when it could not be asked.
func titles(ctx context.Context, library, kind string) []string {
	folder, err := client.FindLibrary(ctx, library)
	if err != nil || folder == nil || folder.ItemID == "" {
		return nil
	}
	items, _, err := client.Search(ctx, embyfin.SearchOptions{ParentID: folder.ItemID, IncludeItemTypes: kind})
	if err != nil {
		return nil
	}

	return names(items)
}

// filmTitles is the fixture films' titles, sorted.
func filmTitles() []string {
	out := make([]string, 0, len(sdkacc.Movies))
	for _, m := range sdkacc.Movies {
		out = append(out, m.Title)
	}
	slices.Sort(out)

	return out
}

// showTitles is the fixture series' titles, sorted.
func showTitles() []string {
	out := make([]string, 0, len(sdkacc.Shows))
	for _, s := range sdkacc.Shows {
		out = append(out, s.Title)
	}
	slices.Sort(out)

	return out
}

// library is one of the suite's libraries as the server lists it.
func library(t *testing.T, l sdkacc.LibraryFixture) *embyfin.VirtualFolder {
	t.Helper()

	folder, err := client.FindLibrary(t.Context(), l.Name)
	if err != nil || folder == nil {
		t.Fatalf("finding the library %s: %v, %v", l.Name, folder, err)
	}

	return folder
}

// film is a film of the film library by its title.
func film(t *testing.T, title string) embyfin.Item {
	t.Helper()

	return one(t, library(t, filmLibrary), "Movie", title)
}

// series is a series of the show library by its title.
func series(t *testing.T, title string) embyfin.Item {
	t.Helper()

	return one(t, library(t, showLibrary), "Series", title)
}

func one(t *testing.T, folder *embyfin.VirtualFolder, kind, title string) embyfin.Item {
	t.Helper()

	items, _, err := client.Search(t.Context(), embyfin.SearchOptions{ParentID: folder.ItemID, IncludeItemTypes: kind})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(items, func(it embyfin.Item) bool { return it.Name == title })
	if i < 0 {
		t.Fatalf("%s lists no %s named %q: %v", folder.Name, kind, title, names(items))
	}

	return items[i]
}

// names is the items' names, sorted.
func names(items []embyfin.Item) []string {
	out := make([]string, 0, len(items))
	for i := range items {
		out = append(out, items[i].Name)
	}
	slices.Sort(out)

	return out
}

// ids is the items' ids, in the order given.
func ids(items []embyfin.Item) []string {
	out := make([]string, 0, len(items))
	for i := range items {
		out = append(out, items[i].ID)
	}

	return out
}

// providerID is an item's id at a provider, whatever case the server writes the provider's name in.
func providerID(it *embyfin.Item, provider string) string {
	for name, id := range it.ProviderIDs {
		if strings.EqualFold(name, provider) {
			return id
		}
	}

	return ""
}

// later runs a call that puts back what a test changed once the test ends, and reports one that fails.
func later(t *testing.T, what string, undo func(ctx context.Context) error) {
	t.Helper()
	t.Cleanup(func() {
		if err := undo(context.Background()); err != nil {
			t.Errorf("cleaning up after the test, %s: %v", what, err)
		}
	})
}

// noItem says whether an error is the one for an id no item has.
func noItem(err error) bool {
	var none *embyfin.NoItemError

	return errors.As(err, &none)
}
