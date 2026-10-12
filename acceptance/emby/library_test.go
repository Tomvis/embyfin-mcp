//go:build acceptance

package emby_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
)

// embyLibraryOptions builds the options both fixtures share. Emby takes the
// whole library definition in the body, and a library created with no
// TypeOptions has no fetchers at all and never goes to the internet: the
// web client posts every type's default fetchers, so with providers on this
// asks the server for them (GetLibrariesAvailableOptions, for the library's
// content type: the spec omits its LibraryContentType and IsNewLibrary query
// parameters, which the emby-library-available-options-query workaround
// adds).
func embyLibraryOptions(ctx context.Context, t *testing.T, l sdkacc.LibraryFixture) *emby.LibraryOptions {
	t.Helper()

	opts := &emby.LibraryOptions{
		PathInfos: []emby.MediaPathInfo{{Path: l.Folder}},
	}
	if !l.Providers {
		// an entry per type with no fetchers named switches the internet
		// providers off for that type; the nfo readers stay on
		for _, typ := range sdkacc.TypesFor(l.CollectionType) {
			opts.TypeOptions = append(opts.TypeOptions, emby.TypeOptions{Type: typ})
		}

		return opts
	}

	available := sdkacc.Must(embyc.Library.GetLibrariesAvailableOptions(ctx, emby.GetLibrariesAvailableOptionsOptions{
		LibraryContentType: l.CollectionType, IsNewLibrary: new(true),
	})).Answer
	for _, typ := range sdkacc.TypesFor(l.CollectionType) {
		i := slices.IndexFunc(available.TypeOptions, func(o emby.LibraryTypeOptions) bool { return o.Type == typ })
		if i < 0 {
			t.Fatalf("GetLibrariesAvailableOptions lists no defaults for %s: %+v", typ, available.TypeOptions)
		}
		enabled := func(infos []emby.LibraryOptionInfo) []string {
			names := make([]string, 0, len(infos))
			for _, f := range infos {
				if f.DefaultEnabled {
					names = append(names, f.Name)
				}
			}
			return names
		}
		metadata, images := enabled(available.TypeOptions[i].MetadataFetchers), enabled(available.TypeOptions[i].ImageFetchers)
		if len(metadata) == 0 || len(images) == 0 {
			t.Fatalf("no default fetchers for %s: %+v", typ, available.TypeOptions[i])
		}
		opts.TypeOptions = append(opts.TypeOptions, emby.TypeOptions{
			Type: typ, MetadataFetchers: metadata, MetadataFetcherOrder: metadata, ImageFetchers: images, ImageFetcherOrder: images,
		})
	}

	return opts
}

// embyLibrary creates and scans a fixture library once and returns the id of
// its root folder, which is what ParentId queries take.
func embyLibrary(t *testing.T, l sdkacc.LibraryFixture) emby.ItemID {
	t.Helper()
	ctx := t.Context()

	if vf, ok := embyFindVirtualFolder(ctx, l.Name); ok {
		return vf.ItemId
	}

	since := embyScanEnded(ctx) //nolint:azproviderlint // read before the create queues the scan, not after
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFolders(ctx, emby.LibraryAddVirtualFolder{
		Name:           l.Name,
		CollectionType: l.CollectionType,
		Paths:          []string{l.Folder},
		RefreshLibrary: new(true),
		LibraryOptions: embyLibraryOptions(ctx, t, l),
	}); err != nil {
		t.Fatalf("creating %s: %v", l.Name, err)
	}

	vf, ok := embyFindVirtualFolder(ctx, l.Name)
	if !ok {
		t.Fatalf("%s was created but is not listed", l.Name)
	}

	embyWaitForItems(ctx, t, vf.ItemId, l)
	embyWaitForScan(ctx, t, since)

	return vf.ItemId
}

// embyFindVirtualFolder looks a library up by name.
func embyFindVirtualFolder(ctx context.Context, name string) (*emby.VirtualFolderInfo, bool) {
	folders := sdkacc.Must(embyc.LibraryStructure.GetLibraryVirtualFoldersQuery(ctx, emby.GetLibraryVirtualFoldersQueryOptions{})).Answer.Items
	for i := range folders {
		if folders[i].Name == name {
			return &folders[i], true
		}
	}

	return nil, false
}

// embyVirtualFolder fetches one library by name, failing when it is missing.
func embyVirtualFolder(ctx context.Context, t *testing.T, name string) *emby.VirtualFolderInfo {
	t.Helper()

	vf, ok := embyFindVirtualFolder(ctx, name)
	if !ok {
		t.Fatalf("no virtual folder named %q", name)
	}

	return vf
}

// embyCount is the recursive count of one item type under a folder.
func embyCount(ctx context.Context, parentID emby.ItemID, kind string) int {
	res, err := embyc.Items.GetItems(ctx, emby.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: kind, Limit: new(1),
	})
	if err != nil {
		return -1
	}

	return res.Answer.TotalRecordCount
}

// embyWaitForItems polls until the scan has found everything the fixture
// holds.
func embyWaitForItems(ctx context.Context, t *testing.T, parentID emby.ItemID, l sdkacc.LibraryFixture) {
	t.Helper()

	var last string
	if l.CollectionType == "music" {
		ok := sdkacc.Poll(sdkacc.ScanPatience, func() bool {
			a, s := embyCount(ctx, parentID, "MusicAlbum"), embyCount(ctx, parentID, "Audio")
			last = fmt.Sprintf("%d albums, %d songs", a, s)

			return a == l.AlbumCount() && s == l.Songs
		})
		if !ok {
			t.Fatalf("%s never reached %d albums and %d songs; last saw %s", l.Name, l.AlbumCount(), l.Songs, last)
		}

		return
	}

	ok := sdkacc.Poll(sdkacc.ScanPatience, func() bool {
		m, s, e := embyCount(ctx, parentID, "Movie"), embyCount(ctx, parentID, "Series"), embyCount(ctx, parentID, "Episode")
		last = fmt.Sprintf("%d movies, %d series, %d episodes", m, s, e)

		return m == l.FilmCount() && s == l.Series && e == l.EpisodeCount()
	})
	if !ok {
		t.Fatalf("%s never reached %d movies, %d series, %d episodes; last saw %s", l.Name, l.FilmCount(), l.Series, l.EpisodeCount(), last)
	}
}

// embyScanEnded reports when the library scan last finished, the zero time when it has
// never run, so a caller can tell a scan it triggers from the one before.
func embyScanEnded(ctx context.Context) emby.Time {
	tasks, err := embyc.ScheduledTask.GetScheduledTasks(ctx, emby.GetScheduledTasksOptions{})
	if err != nil {
		return emby.Time{}
	}
	for _, task := range tasks.Answer {
		if sdkacc.IsScanTask(task.Key, task.Name) && task.LastExecutionResult != nil {
			return task.LastExecutionResult.EndTimeUtc
		}
	}

	return emby.Time{}
}

// embyWaitForScan waits for a library scan newer than since to finish and
// the task to go idle, so the provider lookups a scan triggers have
// finished before a test looks at their results. A triggered scan sits
// queued and Idle for a moment, which is why the previous end time is
// needed: idle alone would return before it started.
func embyWaitForScan(ctx context.Context, t *testing.T, since emby.Time) {
	t.Helper()

	ok := sdkacc.Poll(sdkacc.ScanPatience, func() bool {
		tasks, err := embyc.ScheduledTask.GetScheduledTasks(ctx, emby.GetScheduledTasksOptions{})
		if err != nil {
			return false
		}
		for _, task := range tasks.Answer {
			if !sdkacc.IsScanTask(task.Key, task.Name) {
				continue
			}
			if task.State != "Idle" || task.LastExecutionResult == nil || task.LastExecutionResult.EndTimeUtc.Equal(since.Time) {
				return false
			}
		}

		return true
	})
	if !ok {
		t.Fatal("the library scan never finished")
	}
}

// embyScanTask finds the library scan in the task list.
func embyScanTask(ctx context.Context, t *testing.T) emby.TaskInfo {
	t.Helper()

	for _, task := range sdkacc.Must(embyc.ScheduledTask.GetScheduledTasks(ctx, emby.GetScheduledTasksOptions{})).Answer {
		if sdkacc.IsScanTask(task.Key, task.Name) {
			return task
		}
	}
	t.Fatal("no library scan task in the list")

	return emby.TaskInfo{}
}

// embyMovie finds one of the fixture movies by title. (By search term: Emby
// matches NameStartsWith against the sort name, which drops a leading "The".)
func embyMovie(t *testing.T, title string) emby.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(embyc.Items.GetItems(t.Context(), emby.GetItemsOptions{
		ParentId: embyLibrary(t, sdkacc.SDKMovies), Recursive: new(true), IncludeItemTypes: "Movie",
		SearchTerm: title, Fields: "ProviderIds,Path,ProductionYear",
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no movie titled %q in %s", title, sdkacc.SDKMovies.Name)

	return emby.BaseItemDto{}
}

// embyFirst returns the first item of a kind in a library, for the routes that
// need any album, song or artist to read rather than a particular one.
func embyFirst(t *testing.T, parentID emby.ItemID, kind string) emby.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(embyc.Items.GetItems(t.Context(), emby.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: kind, Limit: new(1), SortBy: "SortName",
	})).Answer
	if len(res.Items) == 0 {
		t.Fatalf("no %s in library %s", kind, parentID)
	}

	return res.Items[0]
}

// embyAlbum finds one of the fixture albums by title.
func embyAlbum(t *testing.T, parentID emby.ItemID, title string) emby.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(embyc.Items.GetItems(t.Context(), emby.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: "MusicAlbum", SearchTerm: title,
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no album titled %q in %s", title, sdkacc.SDKMusic.Name)

	return emby.BaseItemDto{}
}

// embySeries finds one of the fixture series by title (by search term, as
// embyMovie does).
func embySeries(t *testing.T, title string) emby.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(embyc.Items.GetItems(t.Context(), emby.GetItemsOptions{
		ParentId: embyLibrary(t, sdkacc.SDKShows), Recursive: new(true), IncludeItemTypes: "Series",
		SearchTerm: title, Fields: "ProviderIds,ProductionYear",
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no series titled %q in %s", title, sdkacc.SDKShows.Name)

	return emby.BaseItemDto{}
}

// --- the library lifecycle ---------------------------------------------------

// TestEmbyVirtualFolders covers list, add, rename, path add, update and
// remove, and options update, on a library of its own.
//

func TestEmbyVirtualFolders(t *testing.T) {
	ctx := skipUnlessEmby(t)
	moviesID := embyLibrary(t, sdkacc.SDKMovies)

	vf := embyVirtualFolder(ctx, t, sdkacc.SDKMovies.Name)
	if vf.ItemId != moviesID || vf.CollectionType != "movies" || !slices.Equal(vf.Locations, []string{sdkacc.SDKMovies.Folder}) {
		t.Errorf("virtual folder = %+v", vf)
	}
	if vf.LibraryOptions == nil || len(vf.LibraryOptions.PathInfos) != 1 || vf.LibraryOptions.PathInfos[0].Path != sdkacc.SDKMovies.Folder {
		t.Errorf("library options did not decode: %+v", vf.LibraryOptions)
	}

	// a throwaway library for the writes
	const name, renamed = "SDK Lifecycle", "SDK Lifecycle Renamed"
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFolders(ctx, emby.LibraryAddVirtualFolder{
		Name: name, CollectionType: "movies", Paths: []string{"/media/messy-movies"},
		LibraryOptions: embyLibraryOptions(ctx, t, sdkacc.LibraryFixture{CollectionType: "movies", Folder: "/media/messy-movies"}),
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, n := range []string{name, renamed} {
			_ = embyDeleteVirtualFolder(context.WithoutCancel(ctx), n)
		}
	})

	// the rename, path add and path update name the library by its Id (a
	// Name is refused)
	lifecycle := embyVirtualFolder(ctx, t, name)
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersName(ctx, emby.LibraryRenameVirtualFolder{Id: string(lifecycle.ItemId), NewName: renamed}); err != nil {
		t.Fatal(err)
	}
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersPaths(ctx, emby.LibraryAddMediaPath{Id: string(lifecycle.ItemId), Path: "/media/messy-shows", RefreshLibrary: new(false)}); err != nil {
		t.Fatal(err)
	}
	got := embyVirtualFolder(ctx, t, renamed)
	if got.ItemId != lifecycle.ItemId || !slices.Equal(got.Locations, []string{"/media/messy-movies", "/media/messy-shows"}) {
		t.Errorf("after renaming and adding a path = %+v", got)
	}
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersPathsUpdate(ctx, emby.LibraryUpdateMediaPath{
		Id: string(lifecycle.ItemId), PathInfo: &emby.MediaPathInfo{Path: "/media/messy-shows", NetworkPath: "\\\\nas\\messy-shows"},
	}); err != nil {
		t.Fatal(err)
	}
	got = embyVirtualFolder(ctx, t, renamed)
	if got.LibraryOptions == nil || !slices.ContainsFunc(got.LibraryOptions.PathInfos, func(p emby.MediaPathInfo) bool {
		return p.Path == "/media/messy-shows" && p.NetworkPath == "\\\\nas\\messy-shows"
	}) {
		t.Errorf("after updating the path infos = %+v", got.LibraryOptions)
	}

	// the options update runs on the show fixture and puts the value back
	// (SeasonZeroDisplayName is not persisted by this endpoint on Emby; the
	// refresh interval is)
	showsID := embyLibrary(t, sdkacc.SDKShows)
	shows := embyVirtualFolder(ctx, t, sdkacc.SDKShows.Name)
	opts, original := shows.LibraryOptions, shows.LibraryOptions.AutomaticRefreshIntervalDays
	opts.AutomaticRefreshIntervalDays = 7
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersLibraryOptions(ctx, emby.LibraryUpdateLibraryOptions{Id: string(showsID), LibraryOptions: opts}); err != nil {
		t.Fatal(err)
	}
	if got := embyVirtualFolder(ctx, t, sdkacc.SDKShows.Name); got.LibraryOptions.AutomaticRefreshIntervalDays != 7 {
		t.Errorf("AutomaticRefreshIntervalDays = %d after updating the options", got.LibraryOptions.AutomaticRefreshIntervalDays)
	}
	opts.AutomaticRefreshIntervalDays = original
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersLibraryOptions(ctx, emby.LibraryUpdateLibraryOptions{Id: string(showsID), LibraryOptions: opts}); err != nil {
		t.Fatal(err)
	}

	if _, err := embyc.LibraryStructure.DeleteLibraryVirtualFoldersPaths(ctx, emby.DeleteLibraryVirtualFoldersPathsOptions{
		Id: string(lifecycle.ItemId), Path: "/media/messy-shows", RefreshLibrary: new(false),
	}); err != nil {
		t.Fatal(err)
	}
	if got := embyVirtualFolder(ctx, t, renamed); !slices.Equal(got.Locations, []string{"/media/messy-movies"}) {
		t.Errorf("after removing a path locations = %v", got.Locations)
	}
	// and the POST form of both, the path given as a path info
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersPaths(ctx, emby.LibraryAddMediaPath{Id: string(lifecycle.ItemId), PathInfo: &emby.MediaPathInfo{Path: "/media/shows"}, RefreshLibrary: new(false)}); err != nil {
		t.Fatal(err)
	}
	if got := embyVirtualFolder(ctx, t, renamed); !slices.Equal(got.Locations, []string{"/media/messy-movies", "/media/shows"}) {
		t.Errorf("after adding a path info locations = %v", got.Locations)
	}
	if _, err := embyc.LibraryStructure.PostLibraryVirtualFoldersPathsDelete(ctx, emby.LibraryRemoveMediaPath{Id: string(lifecycle.ItemId), Path: "/media/shows", RefreshLibrary: new(false)}); err != nil {
		t.Fatal(err)
	}
	if got := embyVirtualFolder(ctx, t, renamed); !slices.Equal(got.Locations, []string{"/media/messy-movies"}) {
		t.Errorf("after the POST removal locations = %v", got.Locations)
	}
	if err := embyDeleteVirtualFolder(ctx, renamed); err != nil {
		t.Fatal(err)
	}
	if _, ok := embyFindVirtualFolder(ctx, renamed); ok {
		t.Errorf("%s is still listed after removal", renamed)
	}

	// the library-level reads
	mf := sdkacc.Must(embyc.Library.GetLibraryMediaFolders(ctx)).Answer
	if !slices.ContainsFunc(mf.Items, func(it emby.BaseItemDto) bool { return it.Id == moviesID && it.Name == sdkacc.SDKMovies.Name }) {
		t.Errorf("GetLibraryMediaFolders does not list %s", sdkacc.SDKMovies.Name)
	}
	views := sdkacc.Must(embyc.UserViews.GetUsersByUserIdViews(ctx, adminID, false)).Answer
	if !slices.ContainsFunc(views.Items, func(v emby.BaseItemDto) bool { return v.Id == moviesID && v.CollectionType == "movies" }) {
		t.Errorf("GetUsersByUserIdViews = %+v, want %s", views.Items, sdkacc.SDKMovies.Name)
	}
}

// TestEmbyScanTask runs the library scan through the scheduled tasks and
// through the library refresh, and waits for each to finish.
//

func TestEmbyScanTask(t *testing.T) {
	ctx := skipUnlessEmby(t)
	embyLibrary(t, sdkacc.SDKMovies)

	tasks := sdkacc.Must(embyc.ScheduledTask.GetScheduledTasks(ctx, emby.GetScheduledTasksOptions{})).Answer
	if len(tasks) == 0 {
		t.Fatal("no scheduled tasks")
	}
	// (not every task has a Key: the log rotation has none)
	for _, task := range tasks {
		if task.Id == "" || task.Name == "" || task.State == "" || task.Category == "" {
			t.Errorf("task did not decode: %+v", task)
		}
	}

	scan := embyScanTask(ctx, t)
	since := embyScanEnded(ctx)
	if _, err := embyc.ScheduledTask.PostScheduledTasksRunningById(ctx, scan.Id); err != nil {
		t.Fatal(err)
	}
	embyWaitForScan(ctx, t, since)
	after := sdkacc.Must(embyc.ScheduledTask.GetScheduledTasksById(ctx, scan.Id)).Answer
	if after.LastExecutionResult == nil || after.LastExecutionResult.Status == "" || after.LastExecutionResult.StartTimeUtc.IsZero() {
		t.Fatalf("after running the scan its LastExecutionResult = %+v", after.LastExecutionResult)
	}
	if !strings.EqualFold(string(after.LastExecutionResult.Status), "Completed") {
		t.Errorf("the scan finished %s", after.LastExecutionResult.Status)
	}

	since = embyScanEnded(ctx)
	if _, err := embyc.Library.PostLibraryRefresh(ctx); err != nil {
		t.Fatal(err)
	}
	embyWaitForScan(ctx, t, since)
}
