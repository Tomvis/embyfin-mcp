// Package sdkacc is what the suites that run a generated SDK against a real server share: the libraries they create and what a finished scan of each
// holds, the fixture facts their assertions lean on, the waits and clean-ups, and the read sweep. The suites are ../../acceptance/emby, jf and tmdb;
// each brings its own client.
package sdkacc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/katbyte/pandorest/client"
	"github.com/katbyte/pandorest/sweep"

	"github.com/katbyte/embyfin-mcp/lib/testenv"
)

// What is deliberately not exercised, and why:
//
//   - Live TV, DVR, tuners, guide data and recordings: the container has no
//     tuner and no listings provider; the static reads that work without one
//     (tuner host types, the service info) are covered.
//   - DLNA and UPnP: nothing on the docker network answers SSDP.
//   - Media streaming and transcoding (Videos/{id}/stream, HLS, universal
//     audio, trickplay tiles, subtitle delivery): needs a playback session
//     and ffmpeg time for a byte stream the tests would only discard;
//     PlaybackInfo, which reports the sources without starting anything, is
//     covered instead.
//   - SyncPlay groups: needs a second websocket-connected client to join.
//   - Emby Sync (offline downloads), Emby Connect, Games and the Kodi
//     companion endpoints: the reads that answer on a bare server are
//     covered; the writes need a target device or an Emby Connect account.
//   - Jellyfin QuickConnect authorization: the flow needs a second device
//     polling for a code; only the enabled flag is read.
//   - Backup restore: restarts the server mid-run.
//   - Plugin install, update and uninstall, and server restart/shutdown:
//     they would take the container out from under the rest of the suite.
//   - Image upload and delete for users and items: the item image endpoints
//     are covered through the remote-image download; user images have no
//     fixture to upload.
//   - Websocket messages: the SDKs are HTTP only.
//   - Cameras and photo libraries: no fixture images.

// repoRoot is the repository from a suite's folder, which is where the services' paths resolve from.
const repoRoot = "../.."

// AdminID is the administrator the test server was seeded with.
func AdminID() string { return os.Getenv("EMBYFIN_TEST_ADMIN_ID") }

// UserID is the ordinary account it was seeded with.
func UserID() string { return os.Getenv("EMBYFIN_TEST_USER_ID") }

// Password is the password of both.
func Password() string { return os.Getenv("EMBYFIN_TEST_PASSWORD") }

// Run starts the provider proxy the container's HTTPS_PROXY points at, runs the tests, calls cleanup, and returns the exit code. A replay miss, or an
// answer that changed shape, fails the run even when the assertions happened to survive it.
func Run(m *testing.M, backend string, cleanup func()) int {
	proxy, err := testenv.StartProxy(context.Background(), testenv.CassetteDir(backend))
	if err != nil {
		fmt.Fprintln(os.Stderr, "provider proxy:", err)

		return 1
	}

	code := m.Run()
	cleanup()
	if err := proxy.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	if report := proxy.Report(); report != "" {
		fmt.Fprint(os.Stderr, report)
		if code == 0 {
			code = 1
		}
	}

	return code
}

// Libraries is every library the suites create, for removing them after the last test.
func Libraries() []LibraryFixture {
	return []LibraryFixture{SDKMovies, SDKShows, SDKScratch, SDKMusic, SDKMessyMovies, SDKMessyShows, SDKBulk}
}

// The read sweep: every GET operation in a server's definitions, called
// against the running server with arguments resolved from the fixtures, and
// its answer decoded (or its file read). The bespoke tests prove the shapes the
// tools rely on field by field; the sweep proves the rest of the read surface
// answers in its documented status with something in it, and that what it
// sends lands in the model, and that it keeps doing so as the definitions
// change: a GET the importer adds is swept on the next run with nothing to
// write, and one that fails must be classified here.
//
// What the sweep holds an answer to: it decodes; it carries something (a
// file of at least one byte, JSON with at least one value that is not empty,
// zero or false), because an empty list decodes into any model and so proves
// nothing about its shape; and no object in it is lost whole, which is how a
// model whose fields do not match the server shows up (the JSON decoder drops
// a key it has no field for without a word). It does not check every field:
// a server sends many its document does not declare, and a model that holds
// one of an object's keys holds that object.
//
// Every operation either answers with something, or has a sweep.Case that
// says why not: the feature needs something the container lacks (a tuner, a
// DLNA client, a transcoding session), the endpoint is gone from the server,
// the server answers a shape its document does not describe, or what the
// operation reads is not there on a fresh server. A Status, Decode or Empty
// case whose operation starts answering fails the sweep, so a stale one is
// noticed and removed, the way a stale importer workaround is. A Skip case
// is never called, because calling it would hang on ffmpeg, scan a network
// or need a fixture the container cannot have, so it is only ever reviewed by
// hand. Nothing is allowed to answer one way on some runs and another on
// others: where a server does, the test sets up what makes it answer the
// same way every time. The one way out is a MayBeEmpty case, for an answer
// that follows the server image rather than anything a test can set up.
//
// The sweep itself is pandorest's (github.com/katbyte/pandorest/sweep),
// shared with the other servers' SDKs. What is here is how this suite hands
// it a service: its checked-in definitions, the SDK's client, and the base
// client's reading of a status.

// ScanPatience is how long a library scan is given to settle. Three minutes
// is plenty on a quiet machine and not always enough on a CI runner sharing
// itself with three other suites, where a music scan has come in just over.
const ScanPatience = 6 * time.Minute

// EditPatience is how long a change to an item is given to show up in
// what the server answers. Jellyfin applies a collection change a moment
// after it answers the request, so a read straight after it can miss it.
const EditPatience = 30 * time.Second

// LibraryFixture is one of the libraries the suite creates. The folders are
// the container-side paths .scripts/testenv.sh lays out.
type LibraryFixture struct {
	Name, CollectionType, Folder string
	// Providers leaves the server's internet fetchers on, so the scan fills
	// the library from TMDB (and, on Emby, TheTVDB and OMDb) through the
	// provider proxy. Off, the nfo sidecars are all the server knows.
	Providers bool
	// Movies, Series and Episodes are the counts a finished scan settles on,
	// Albums and Songs the same for a music library.
	Movies, Series, Episodes int
	Albums, Songs            int
	// Versions is how many of the films are a second file beside another in
	// its folder, which Jellyfin folds into that film and Emby lists as a
	// film of its own
	Versions int
	// TagAlbums is how many more albums Emby builds from the tags than there
	// are album folders (a track tagged with its album misspelt), and
	// ExtraEpisodes how many extras it takes for episodes (a featurette in a
	// season's Extras folder); Jellyfin builds albums from folders and keeps
	// a season's extras as extras
	TagAlbums, ExtraEpisodes int
}

// FilmCount is how many films a finished scan of the library lists on this
// backend.
func (l LibraryFixture) FilmCount() int {
	if testenv.Backend() == "jellyfin" {
		return l.Movies - l.Versions
	}

	return l.Movies
}

// AlbumCount is how many albums a finished scan of the library lists on this
// backend.
func (l LibraryFixture) AlbumCount() int {
	if testenv.Backend() == "jellyfin" {
		return l.Albums
	}

	return l.Albums + l.TagAlbums
}

// EpisodeCount is how many episodes a finished scan of the library lists on
// this backend.
func (l LibraryFixture) EpisodeCount() int {
	if testenv.Backend() == "jellyfin" {
		return l.Episodes
	}

	return l.Episodes + l.ExtraEpisodes
}

// The libraries. Only the movie library has providers on: it is what the
// remote-image, remote-search and refresh tests need, and each library with
// providers on costs a cassette full of provider traffic.
var (
	SDKMovies = LibraryFixture{Name: "SDK Movies", CollectionType: "movies", Folder: "/media/movies", Providers: true, Movies: 10}
	SDKShows  = LibraryFixture{Name: "SDK Shows", CollectionType: "tvshows", Folder: "/media/shows", Series: 4, Episodes: 11}
	// SDKScratch sits over a folder the destructive test lays out itself,
	// so the delete has something to remove that nothing else relies on
	SDKScratch = LibraryFixture{Name: "SDK Scratch", CollectionType: "movies", Folder: "/media/sdk-scratch", Movies: 1}
	// SDKMusic is what the artist, album, song and music genre routes have to
	// read: without it a fifth of each server's item surface cannot be called
	SDKMusic = LibraryFixture{Name: "SDK Music", CollectionType: "music", Folder: "/media/music", Albums: 5, TagAlbums: 1, Songs: 20}
	// SDKMessyMovies and SDKMessyShows sit over the messy folders with the
	// fetchers off, for what the servers read off the files themselves: a
	// legacy codec, a language, a film held as two files, a disc kept whole,
	// a file holding two episodes
	SDKMessyMovies = LibraryFixture{Name: "SDK Messy Movies", CollectionType: "movies", Folder: "/media/messy-movies", Movies: 14, Versions: 1}
	SDKMessyShows  = LibraryFixture{Name: "SDK Messy Shows", CollectionType: "tvshows", Folder: "/media/messy-shows", Series: 12, Episodes: 30, ExtraEpisodes: 1}
	// SDKBulk sits over a folder the bulk delete test lays out itself
	SDKBulk = LibraryFixture{Name: "SDK Bulk Delete", CollectionType: "movies", Folder: "/media/sdk-bulk", Movies: len(BulkTitles)}
)

// BulkTitles are the films the bulk delete test lays out, copies of one
// fixture file under the names of messy films the clean library does not hold
var BulkTitles = []string{"Interstellar (2014)", "Memento (2000)", "Cube (1997)", "Coyote vs. Acme (2026)"}

// MovieFixture is a film in the clean Movies folder, as its nfo describes it.
type MovieFixture struct {
	Title    string
	Year     int
	TMDB     string
	IMDB     string
	Genre    string
	Director string
}

var Movies = []MovieFixture{
	{"Alien", 1979, "348", "tt0078748", "Horror", "Ridley Scott"},
	{"Aliens", 1986, "679", "tt0090605", "Action", "James Cameron"},
	{"Blade Runner", 1982, "78", "tt0083658", "Science Fiction", "Ridley Scott"},
	{"Dune", 2021, "438631", "tt1160419", "Science Fiction", "Denis Villeneuve"},
	{"Dune: Part Two", 2024, "693134", "tt15239678", "Science Fiction", "Denis Villeneuve"},
	{"Princess Mononoke", 1997, "128", "tt0119698", "Animation", "Hayao Miyazaki"},
	{"Arrival", 2016, "329865", "tt2543164", "Drama", "Denis Villeneuve"},
	{"The Thirteenth Floor", 1999, "1090", "tt0139809", "Science Fiction", "Josef Rusnak"},
	{"Brüno", 2009, "18480", "tt0889583", "Comedy", "Larry Charles"},
	{"Limitless", 2011, "51876", "tt1219289", "Thriller", "Neil Burger"},
}

// ShowFixture is a series in the clean Shows folder.
type ShowFixture struct {
	Title    string
	Year     int
	TMDB     string
	TVDB     string
	Episodes map[int][]int // season -> episode numbers on disk
}

var Shows = []ShowFixture{
	{"Severance", 2022, "95396", "371980", map[int][]int{1: {1, 2}, 2: {1, 2}}},
	{"Breaking Bad", 2008, "1396", "81189", map[int][]int{1: {1, 2, 3}}},
	{"The Expanse", 2015, "63639", "280619", map[int][]int{0: {1}, 1: {1, 2}}},
	{"Limitless", 2015, "62687", "295743", map[int][]int{1: {1}}},
}

// The fixture facts the assertions lean on.
const (
	Alien       = "Alien"
	Aliens      = "Aliens"
	BladeRunner = "Blade Runner"
	Severance   = "Severance"
	RidleyScott = "Ridley Scott"
	// ThirteenthFloor is the one clean film with a subtitle beside it, an
	// English .srt
	ThirteenthFloor = "The Thirteenth Floor"
	// LyricTrack is the one fixture track with an .lrc sidecar beside it
	LyricTrack = "The Future We Built"
	// DarkSide is an album whose artist has another in the fixtures, and
	// ProgressiveRock the genre the two share and no other artist has
	DarkSide        = "The Dark Side of the Moon"
	ProgressiveRock = "Progressive Rock"
	// ScratchTitle is the throwaway movie the destructive test lays out
	ScratchTitle = "SDK Disposable"
	// SDKApp is the AppName of the API key the suite creates and revokes
	SDKApp = "sdk-acceptance"
	// DeviceID is what both SDK clients identify themselves as, and so the
	// DeviceId of the suite's own session
	DeviceID = "embyfin-mcp"
)

// UndoLater runs a call that puts back what a test changed once the test
// ends, and reports one that fails rather than dropping it: the change would
// be left for the tests after.
func UndoLater(t *testing.T, what string, undo func() error) {
	t.Helper()
	t.Cleanup(func() {
		if err := undo(); err != nil {
			t.Errorf("cleaning up after the test, %s: %v", what, err)
		}
	})
}

// RemoveLater removes what a test made once the test ends, as UndoLater
// does; what is already gone (a 404: a server can drop an emptied
// collection itself) needs no removing.
func RemoveLater(t *testing.T, what string, remove func() error) {
	t.Helper()
	UndoLater(t, what, func() error {
		if err := remove(); err != nil && !client.IsNotFound(err) {
			return err
		}

		return nil
	})
}

// Must unwraps a call that Must not fail. Go only allows a multi-value call
// as a function's sole argument, so this cannot also take *testing.T - it
// panics instead, which the test framework reports as a failure. That is the
// right severity here: if the server will not answer, nothing downstream is
// meaningful.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

// Number is a season or episode Number the SDK holds by pointer, or -1 when
// the server sent none: no Number is -1, so a missing one fails every check
// of one rather than passing one of 0.
func Number(n *int) int {
	if n == nil {
		return -1
	}

	return *n
}

// Poll calls f every two seconds until it returns true or the deadline
// passes, and reports whether it did.
func Poll(timeout time.Duration, f func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if f() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(2 * time.Second)
	}
}

// ScratchDir lays out the throwaway movie the destructive test deletes: a
// copy of one fixture file under a folder of its own, no nfo, so the scan
// titles it from the folder and no provider is asked about it.
func ScratchDir(t *testing.T) string {
	t.Helper()

	return LayOut(t, "sdk-scratch", ScratchTitle+" (1995)")
}

// LayOut copies one fixture file into a folder of its own for each of the
// films named ("Title (Year)"), under a folder of the media tree the
// container sees as /media/<under>, and returns that folder on the host.
func LayOut(t *testing.T, under string, films ...string) string {
	t.Helper()

	data := testenv.DataDir()
	if data == filepath.Join("", "media") {
		t.Skip("EMBYFIN_TEST_DATA is not set")
	}
	source := filepath.Join(data, "movies", "Princess Mononoke (1997)", "Princess Mononoke (1997).mp4")
	video, err := os.ReadFile(source) //nolint:gosec // a fixture under the test data dir
	if err != nil {
		t.Fatalf("reading a fixture to copy: %v", err)
	}
	for _, film := range films {
		dir := filepath.Join(data, under, film)
		// the mode MkdirAll and WriteFile are asked for is filtered by the
		// process umask, which on Linux leaves a directory the media server's
		// own user (uid 2 in Emby's image) cannot delete from, so a delete
		// under test fails; chmod is not filtered. Docker Desktop maps every
		// file to the container's user, which is why this only bites in CI.
		if err := os.MkdirAll(dir, 0o777); err != nil { //nolint:gosec // the container reads it as another user
			t.Fatal(err)
		}
		for p := dir; strings.HasPrefix(p, data) && p != data; p = filepath.Dir(p) {
			if err := os.Chmod(p, 0o777); err != nil { //nolint:gosec // same
				t.Fatal(err)
			}
		}
		file := filepath.Join(dir, film+".mp4")
		if err := os.WriteFile(file, video, 0o666); err != nil { //nolint:gosec // same
			t.Fatal(err)
		}
		if err := os.Chmod(file, 0o666); err != nil { //nolint:gosec // same
			t.Fatal(err)
		}
	}

	return filepath.Join(data, under)
}

// IsScanTask picks the library scan out of the task list on either server.
func IsScanTask(key, name string) bool {
	return key == "RefreshLibrary" || strings.Contains(strings.ToLower(name), "scan media library")
}

// RunSweep calls every GET operation of the service on sdk. missing is, per kind of id, a value of its form that nothing on the server
// has: the sweep asks a GET with each, and fails when the server answers other than it is said to.
func RunSweep(t *testing.T, service string, sdk any, fixtures sweep.Fixtures, cases map[string]sweep.Case, missing map[string]sweep.Missing) {
	t.Helper()

	svc, err := loadDefinitions(service)
	if err != nil {
		t.Fatal(err)
	}

	sweep.Sweep{Definitions: svc, Client: sdk, StatusCode: client.StatusCode, Fixtures: fixtures, Cases: cases, MissingIDs: missing}.Run(t)
}

// TypesFor is the item types a library of a kind holds, which is what its settings name fetchers for.
func TypesFor(collectionType string) []string {
	switch collectionType {
	case "movies":
		return []string{"Movie"}
	case "tvshows":
		return []string{"Series", "Season", "Episode"}
	case "music":
		return []string{"MusicArtist", "MusicAlbum", "Audio"}
	default:
		return []string{"Movie", "Series", "Season", "Episode", "MusicArtist", "MusicAlbum", "Audio", "Video"}
	}
}
