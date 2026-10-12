//go:build acceptance

package jf_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/pandorest/client"
)

// jfLibraryOptions builds the options both fixtures share. The generated
// LibraryOptions booleans are *bool with omitempty, so the ones left unset
// are not sent and the server's defaults apply to them. Enabled and the
// internet providers are still set explicitly: say what the fixture needs
// rather than lean on server defaults.
func jfLibraryOptions(l sdkacc.LibraryFixture) *jf.LibraryOptions {
	opts := &jf.LibraryOptions{
		Enabled:                 new(true),
		EnableInternetProviders: new(l.Providers),
		PathInfos:               []jf.MediaPathInfo{{Path: l.Folder}},
	}
	if !l.Providers {
		// an entry per type with no fetchers named switches the internet
		// providers off for that type; the nfo readers stay on
		for _, typ := range sdkacc.TypesFor(l.CollectionType) {
			opts.TypeOptions = append(opts.TypeOptions, jf.TypeOptions{Type: typ})
		}
	}

	return opts
}

// jfLibrary creates and scans a fixture library once and returns the id of
// its root folder, which is what ParentId queries take.
func jfLibrary(t *testing.T, l sdkacc.LibraryFixture) string {
	t.Helper()
	ctx := t.Context()

	if vf, ok := jfFindVirtualFolder(ctx, l.Name); ok {
		return vf.ItemId
	}

	since := jfScanEnded(ctx) //nolint:azproviderlint // read before the create queues the scan, not after
	if _, err := jfc.LibraryStructure.AddVirtualFolder(ctx, jf.AddVirtualFolderDto{LibraryOptions: jfLibraryOptions(l)}, jf.AddVirtualFolderOptions{
		Name:           l.Name,
		CollectionType: jf.CollectionTypeOptions(l.CollectionType),
		Paths:          []string{l.Folder},
		RefreshLibrary: new(true),
	}); err != nil {
		t.Fatalf("creating %s: %v", l.Name, err)
	}

	vf, ok := jfFindVirtualFolder(ctx, l.Name)
	if !ok {
		t.Fatalf("%s was created but is not listed", l.Name)
	}

	jfWaitForItems(ctx, t, vf.ItemId, l)
	jfWaitForScan(ctx, t, since)

	return vf.ItemId
}

// jfFindVirtualFolder looks a library up by name.
func jfFindVirtualFolder(ctx context.Context, name string) (*jf.VirtualFolderInfo, bool) {
	folders := sdkacc.Must(jfc.LibraryStructure.GetVirtualFolders(ctx)).Answer
	for i := range folders {
		if folders[i].Name == name {
			return &folders[i], true
		}
	}

	return nil, false
}

// jfVirtualFolder fetches one library by name, failing when it is missing.
func jfVirtualFolder(ctx context.Context, t *testing.T, name string) *jf.VirtualFolderInfo {
	t.Helper()

	vf, ok := jfFindVirtualFolder(ctx, name)
	if !ok {
		t.Fatalf("no virtual folder named %q", name)
	}

	return vf
}

// jfCount is the recursive count of one item type under a folder.
func jfCount(ctx context.Context, parentID string, kind jf.BaseItemKind) int {
	res, err := jfc.Library.GetItems(ctx, jf.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{kind}, Limit: new(1),
	})
	if err != nil {
		return -1
	}

	return res.Answer.TotalRecordCount
}

// jfScanNudge asks for the library scan again, once, when nothing has been
// found by a third of the patience. The scan a library create queues does not
// always run, and then every count stays at zero however long the wait: CI saw
// a music library sit empty for the whole six minutes.
func jfScanNudge(ctx context.Context, t *testing.T) func(found int) {
	t.Helper()
	due, asked := time.Now().Add(sdkacc.ScanPatience/3), false

	return func(found int) {
		if found > 0 || asked || time.Now().Before(due) {
			return
		}
		asked = true
		// a nudge the server refuses leaves the wait to fail on the count
		// alone, so it is said
		if _, err := jfc.Library.RefreshLibrary(ctx); err != nil {
			t.Errorf("asking for the library scan again: %v", err)
		}
	}
}

// jfWaitForItems polls until the scan has found everything the fixture
// holds.
func jfWaitForItems(ctx context.Context, t *testing.T, parentID string, l sdkacc.LibraryFixture) {
	t.Helper()

	nudge := jfScanNudge(ctx, t)
	var last string
	if l.CollectionType == "music" {
		ok := sdkacc.Poll(sdkacc.ScanPatience, func() bool {
			a, s := jfCount(ctx, parentID, jf.BaseItemKindMusicAlbum), jfCount(ctx, parentID, jf.BaseItemKindAudio)
			last = fmt.Sprintf("%d albums, %d songs", a, s)
			nudge(a + s)

			return a == l.AlbumCount() && s == l.Songs
		})
		if !ok {
			t.Fatalf("%s never reached %d albums and %d songs; last saw %s", l.Name, l.AlbumCount(), l.Songs, last)
		}

		return
	}

	ok := sdkacc.Poll(sdkacc.ScanPatience, func() bool {
		m, s, e := jfCount(ctx, parentID, jf.BaseItemKindMovie), jfCount(ctx, parentID, jf.BaseItemKindSeries), jfCount(ctx, parentID, jf.BaseItemKindEpisode)
		last = fmt.Sprintf("%d movies, %d series, %d episodes", m, s, e)
		nudge(m + s + e)

		return m == l.FilmCount() && s == l.Series && e == l.EpisodeCount()
	})
	if !ok {
		t.Fatalf("%s never reached %d movies, %d series, %d episodes; last saw %s", l.Name, l.FilmCount(), l.Series, l.EpisodeCount(), last)
	}
}

// jfScanEnded reports when the library scan last finished, "" when it has
// never run, so a caller can tell a scan it triggers from the one before.
func jfScanEnded(ctx context.Context) string {
	tasks, err := jfc.ScheduledTask.GetTasks(ctx, jf.GetTasksOptions{})
	if err != nil {
		return ""
	}
	for _, task := range tasks.Answer {
		if sdkacc.IsScanTask(task.Key, task.Name) && task.LastExecutionResult != nil {
			return task.LastExecutionResult.EndTimeUtc
		}
	}

	return ""
}

// jfWaitForScan waits for a library scan newer than since to finish and the
// task to go idle, so the provider lookups a scan triggers have finished
// before a test looks at their results. A triggered scan sits queued and
// Idle for a moment, which is why the previous end time is needed: idle
// alone would return before it started.
func jfWaitForScan(ctx context.Context, t *testing.T, since string) {
	t.Helper()

	ok := sdkacc.Poll(sdkacc.ScanPatience, func() bool {
		tasks, err := jfc.ScheduledTask.GetTasks(ctx, jf.GetTasksOptions{})
		if err != nil {
			return false
		}
		for _, task := range tasks.Answer {
			if !sdkacc.IsScanTask(task.Key, task.Name) {
				continue
			}
			if task.State != jf.TaskStateIdle || task.LastExecutionResult == nil || task.LastExecutionResult.EndTimeUtc == since {
				return false
			}
		}

		return true
	})
	if !ok {
		t.Fatal("the library scan never finished")
	}
}

// jfScanTask finds the library scan in the task list.
func jfScanTask(ctx context.Context, t *testing.T) jf.TaskInfo {
	t.Helper()

	for _, task := range sdkacc.Must(jfc.ScheduledTask.GetTasks(ctx, jf.GetTasksOptions{})).Answer {
		if sdkacc.IsScanTask(task.Key, task.Name) {
			return task
		}
	}
	t.Fatal("no library scan task in the list")

	return jf.TaskInfo{}
}

// jfMovie finds one of the fixture movies by title.
func jfMovie(t *testing.T, title string) jf.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(jfc.Library.GetItems(t.Context(), jf.GetItemsOptions{
		ParentId: jfLibrary(t, sdkacc.SDKMovies), Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindMovie},
		SearchTerm: title, Fields: []jf.ItemFields{jf.ItemFieldsProviderIds, jf.ItemFieldsPath},
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no movie titled %q in %s", title, sdkacc.SDKMovies.Name)

	return jf.BaseItemDto{}
}

// jfFirst returns the first item of a kind in a library, for the routes that
// need any album, song or artist to read rather than a particular one.
func jfFirst(t *testing.T, parentID string, kind jf.BaseItemKind) jf.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(jfc.Library.GetItems(t.Context(), jf.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{kind},
		Limit: new(1), SortBy: []jf.ItemSortBy{jf.ItemSortBySortName},
	})).Answer
	if len(res.Items) == 0 {
		t.Fatalf("no %s in library %s", kind, parentID)
	}

	return res.Items[0]
}

// jfSong finds one of the fixture tracks by title.
func jfSong(t *testing.T, parentID, title string) jf.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(jfc.Library.GetItems(t.Context(), jf.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindAudio},
		SearchTerm: title, Fields: []jf.ItemFields{jf.ItemFieldsPath},
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no track titled %q in %s", title, sdkacc.SDKMusic.Name)

	return jf.BaseItemDto{}
}

// jfAlbum finds one of the fixture albums by title.
func jfAlbum(t *testing.T, parentID, title string) jf.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(jfc.Library.GetItems(t.Context(), jf.GetItemsOptions{
		ParentId: parentID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindMusicAlbum}, SearchTerm: title,
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no album titled %q in %s", title, sdkacc.SDKMusic.Name)

	return jf.BaseItemDto{}
}

// jfSeries finds one of the fixture series by title.
func jfSeries(t *testing.T, title string) jf.BaseItemDto {
	t.Helper()

	res := sdkacc.Must(jfc.Library.GetItems(t.Context(), jf.GetItemsOptions{
		ParentId: jfLibrary(t, sdkacc.SDKShows), Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindSeries},
		SearchTerm: title, Fields: []jf.ItemFields{jf.ItemFieldsProviderIds},
	})).Answer
	for i := range res.Items {
		if res.Items[i].Name == title {
			return res.Items[i]
		}
	}
	t.Fatalf("no series titled %q in %s", title, sdkacc.SDKShows.Name)

	return jf.BaseItemDto{}
}

// --- the library lifecycle ---------------------------------------------------

// TestJFVirtualFolders covers list, add, rename, path add and remove, and
// options update, on a library of its own, so nothing the shared fixtures
// hold is disturbed.
//

func TestJFVirtualFolders(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	moviesID := jfLibrary(t, sdkacc.SDKMovies)

	vf := jfVirtualFolder(ctx, t, sdkacc.SDKMovies.Name)
	if vf.ItemId != moviesID || vf.CollectionType != jf.CollectionTypeOptionsMovies || !slices.Equal(vf.Locations, []string{sdkacc.SDKMovies.Folder}) {
		t.Errorf("virtual folder = %+v", vf)
	}
	if vf.LibraryOptions == nil || len(vf.LibraryOptions.PathInfos) != 1 || vf.LibraryOptions.PathInfos[0].Path != sdkacc.SDKMovies.Folder {
		t.Errorf("library options did not decode: %+v", vf.LibraryOptions)
	}

	// a throwaway library for the writes
	const name, renamed = "SDK Lifecycle", "SDK Lifecycle Renamed"
	if _, err := jfc.LibraryStructure.AddVirtualFolder(ctx, jf.AddVirtualFolderDto{LibraryOptions: jfLibraryOptions(sdkacc.LibraryFixture{CollectionType: "movies", Folder: "/media/messy-movies"})}, jf.AddVirtualFolderOptions{
		Name: name, CollectionType: jf.CollectionTypeOptionsMovies, Paths: []string{"/media/messy-movies"}, RefreshLibrary: new(false),
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// the library goes by one of the two names, as far as the test got
		for _, n := range []string{name, renamed} {
			if _, err := jfc.LibraryStructure.RemoveVirtualFolder(context.WithoutCancel(ctx), jf.RemoveVirtualFolderOptions{Name: n, RefreshLibrary: new(false)}); err != nil && !client.IsNotFound(err) {
				t.Errorf("removing %s: %v", n, err)
			}
		}
	})

	if _, err := jfc.LibraryStructure.RenameVirtualFolder(ctx, jf.RenameVirtualFolderOptions{Name: name, NewName: renamed, RefreshLibrary: new(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := jfc.LibraryStructure.AddMediaPath(ctx, jf.MediaPathDto{Name: renamed, Path: "/media/messy-shows"}, false); err != nil {
		t.Fatal(err)
	}
	// a library that has never been scanned has no ItemId and no
	// LibraryOptions in the listing yet; its locations are there
	got := jfVirtualFolder(ctx, t, renamed)
	if !slices.Equal(got.Locations, []string{"/media/messy-movies", "/media/messy-shows"}) {
		t.Errorf("after AddMediaPath locations = %v", got.Locations)
	}

	if _, err := jfc.LibraryStructure.RemoveMediaPath(ctx, jf.RemoveMediaPathOptions{Name: renamed, Path: "/media/messy-shows", RefreshLibrary: new(false)}); err != nil {
		t.Fatal(err)
	}
	if got := jfVirtualFolder(ctx, t, renamed); !slices.Equal(got.Locations, []string{"/media/messy-movies"}) {
		t.Errorf("after RemoveMediaPath locations = %v", got.Locations)
	}

	// the options update needs a scanned library, so it runs on the show
	// fixture and puts the value back
	showsID := jfLibrary(t, sdkacc.SDKShows)
	shows := jfVirtualFolder(ctx, t, sdkacc.SDKShows.Name)
	opts, original := shows.LibraryOptions, shows.LibraryOptions.SeasonZeroDisplayName
	opts.SeasonZeroDisplayName = "SDK Specials"
	if _, err := jfc.LibraryStructure.UpdateLibraryOptions(ctx, jf.UpdateLibraryOptionsDto{Id: showsID, LibraryOptions: opts}); err != nil {
		t.Fatal(err)
	}
	if got := jfVirtualFolder(ctx, t, sdkacc.SDKShows.Name); got.LibraryOptions.SeasonZeroDisplayName != "SDK Specials" {
		t.Errorf("SeasonZeroDisplayName = %q after UpdateLibraryOptions", got.LibraryOptions.SeasonZeroDisplayName)
	}
	opts.SeasonZeroDisplayName = original
	if _, err := jfc.LibraryStructure.UpdateLibraryOptions(ctx, jf.UpdateLibraryOptionsDto{Id: showsID, LibraryOptions: opts}); err != nil {
		t.Fatal(err)
	}

	if _, err := jfc.LibraryStructure.RemoveVirtualFolder(ctx, jf.RemoveVirtualFolderOptions{Name: renamed, RefreshLibrary: new(false)}); err != nil {
		t.Fatal(err)
	}
	if _, ok := jfFindVirtualFolder(ctx, renamed); ok {
		t.Errorf("%s is still listed after RemoveVirtualFolder", renamed)
	}

	// the library-level reads
	if paths := sdkacc.Must(jfc.Library.GetPhysicalPaths(ctx)).Answer; !slices.Contains(paths, sdkacc.SDKMovies.Folder) {
		t.Errorf("GetPhysicalPaths = %v, want %s among them", paths, sdkacc.SDKMovies.Folder)
	}
	mf := sdkacc.Must(jfc.Library.GetMediaFolders(ctx)).Answer
	if !slices.ContainsFunc(mf.Items, func(it jf.BaseItemDto) bool { return it.Id == moviesID && it.Name == sdkacc.SDKMovies.Name }) {
		t.Errorf("GetMediaFolders does not list %s", sdkacc.SDKMovies.Name)
	}
	info := sdkacc.Must(jfc.Library.GetLibraryOptionsInfo(ctx, jf.GetLibraryOptionsInfoOptions{LibraryContentType: jf.CollectionTypeMovies, IsNewLibrary: new(true)})).Answer
	if len(info.MetadataSavers) == 0 || len(info.TypeOptions) == 0 {
		t.Errorf("GetLibraryOptionsInfo = %+v, want savers and type options", info)
	}
}

// TestJFScanTask runs the library scan through the scheduled tasks and
// through the library refresh, and waits for each to finish.
//

func TestJFScanTask(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	jfLibrary(t, sdkacc.SDKMovies)

	tasks := sdkacc.Must(jfc.ScheduledTask.GetTasks(ctx, jf.GetTasksOptions{IsHidden: new(false)})).Answer
	if len(tasks) == 0 {
		t.Fatal("no scheduled tasks")
	}
	for _, task := range tasks {
		if task.Id == "" || task.Name == "" || task.Key == "" || task.State == "" || task.Category == "" {
			t.Errorf("task did not decode: %+v", task)
		}
	}

	scan := jfScanTask(ctx, t)
	since := jfScanEnded(ctx)
	if _, err := jfc.ScheduledTask.StartTask(ctx, scan.Id); err != nil {
		t.Fatal(err)
	}
	jfWaitForScan(ctx, t, since)
	after := sdkacc.Must(jfc.ScheduledTask.GetTask(ctx, scan.Id)).Answer
	if after.LastExecutionResult == nil || after.LastExecutionResult.Status == "" || after.LastExecutionResult.StartTimeUtc == "" {
		t.Fatalf("after StartTask the scan's LastExecutionResult = %+v", after.LastExecutionResult)
	}
	if !strings.EqualFold(string(after.LastExecutionResult.Status), "Completed") {
		t.Errorf("the scan finished %s", after.LastExecutionResult.Status)
	}

	since = jfScanEnded(ctx)
	if _, err := jfc.Library.RefreshLibrary(ctx); err != nil {
		t.Fatal(err)
	}
	jfWaitForScan(ctx, t, since)
}
