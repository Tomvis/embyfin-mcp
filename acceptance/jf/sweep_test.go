//go:build acceptance

package jf_test

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/pandorest/sweep"
)

// TestJFReadSweep calls every Jellyfin GET against the fixtures (see
// sweep_test.go).
//

func TestJFReadSweep(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	moviesID := jfLibrary(t, sdkacc.SDKMovies)
	jfLibrary(t, sdkacc.SDKShows)

	musicID := jfLibrary(t, sdkacc.SDKMusic)

	movie := jfMovie(t, sdkacc.Alien)
	item := sdkacc.Must(jfc.Library.GetItem(ctx, movie.Id, adminID)).Answer
	album := jfFirst(t, musicID, jf.BaseItemKindMusicAlbum)
	song := jfFirst(t, musicID, jf.BaseItemKindAudio)
	musicArtists := sdkacc.Must(jfc.Artist.GetArtists(ctx, jf.GetArtistsOptions{ParentId: musicID, UserId: adminID})).Answer.Items //nolint:staticcheck // deprecated in the document and still an operation the SDK serves, which is what this holds it to
	if len(musicArtists) == 0 {
		t.Fatal("the music fixtures hold no artist")
	}
	// Jellyfin 12.1 documents no music genre listing, and /Genres of a music
	// library lists the genres its tags name. Not the first music genre the
	// server holds: those include what MusicBrainz says of an artist only the
	// tags name (The Pink Floyd's acid rock), which no album in the library
	// carries to make the genre's image from
	musicGenres := sdkacc.Must(jfc.Genre.GetGenres(ctx, jf.GetGenresOptions{ParentId: musicID, UserId: adminID})).Answer.Items
	if len(musicGenres) == 0 {
		t.Fatal("the music fixtures carry no genre")
	}
	musicGenre := musicGenres[0]
	series := jfSeries(t, sdkacc.Severance)
	people := sdkacc.Must(jfc.Person.GetPersons(ctx, jf.GetPersonsOptions{SearchTerm: sdkacc.RidleyScott})).Answer.Items
	studios := sdkacc.Must(jfc.Studio.GetStudios(ctx, jf.GetStudiosOptions{ParentId: moviesID})).Answer.Items
	logs := sdkacc.Must(jfc.System.GetServerLogs(ctx)).Answer
	plugins := sdkacc.Must(jfc.Plugin.GetPlugins(ctx)).Answer
	devices := sdkacc.Must(jfc.Device.GetDevices(ctx)).Answer.Items
	if len(people) == 0 || len(studios) == 0 || len(logs) == 0 || len(plugins) == 0 || len(devices) == 0 || len(item.MediaSources) == 0 {
		t.Fatalf("the fixtures are incomplete: %d people, %d studios, %d logs, %d plugins, %d devices, %d media sources",
			len(people), len(studios), len(logs), len(plugins), len(devices), len(item.MediaSources))
	}

	// the film with a subtitle, for the routes that hand one out
	subtitled := sdkacc.Must(jfc.Library.GetItem(ctx, jfMovie(t, sdkacc.ThirteenthFloor).Id, adminID)).Answer
	if len(subtitled.MediaSources) == 0 {
		t.Fatalf("%s has no media source", sdkacc.ThirteenthFloor)
	}
	srt := slices.IndexFunc(subtitled.MediaSources[0].MediaStreams, func(s jf.MediaStream) bool { return s.Type == jf.MediaStreamTypeSubtitle })
	if srt < 0 {
		t.Fatalf("%s lists no subtitle stream: %+v", sdkacc.ThirteenthFloor, subtitled.MediaSources[0].MediaStreams)
	}
	srtIndex := strconv.Itoa(subtitled.MediaSources[0].MediaStreams[srt].Index)
	// the catalogue the package routes read
	packages := sdkacc.Must(jfc.Plugin.GetPackages(ctx)).Answer
	if len(packages) == 0 {
		t.Fatal("GetPackages listed nothing")
	}
	tmdbPlugin := slices.IndexFunc(plugins, func(p jf.PluginInfo) bool { return p.Name == "TMDb" })
	if tmdbPlugin < 0 {
		t.Fatalf("no TMDb plugin among %+v", plugins)
	}

	// a playlist to read, and one of music for the mix made from a playlist
	created := sdkacc.Must(jfc.Playlist.CreatePlaylist(ctx, jf.CreatePlaylistDto{Name: "SDK Sweep", Ids: []jf.ItemID{movie.Id}, MediaType: jf.MediaTypeVideo, UserId: adminID})).Answer
	sdkacc.RemoveLater(t, "deleting the playlist", func() error {
		_, err := jfc.Library.DeleteItem(context.WithoutCancel(ctx), jf.ItemID(created.Id))
		return err
	})
	music := sdkacc.Must(jfc.Playlist.CreatePlaylist(ctx, jf.CreatePlaylistDto{Name: "SDK Sweep Music", Ids: []jf.ItemID{song.Id}, MediaType: jf.MediaTypeAudio, UserId: adminID})).Answer
	sdkacc.RemoveLater(t, "deleting the music playlist", func() error {
		_, err := jfc.Library.DeleteItem(context.WithoutCancel(ctx), jf.ItemID(music.Id))
		return err
	})
	// and a collection holding the film, once its member has landed (a scan's
	// refresh can write over it; see TestJFCollections)
	jfScanIdle(ctx, t)
	collection := sdkacc.Must(jfc.Collection.CreateCollection(ctx, jf.CreateCollectionOptions{Name: "SDK Sweep", Ids: []jf.ItemID{movie.Id}})).Answer.Id
	t.Cleanup(func() { jfDeleteCollection(context.WithoutCancel(ctx), t, collection) })
	if !sdkacc.Poll(sdkacc.EditPatience, func() bool {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: jf.ItemID(collection), UserId: adminID})).Answer
		return len(res.Items) == 1 && res.Items[0].Id == movie.Id
	}) {
		t.Fatalf("the sweep's collection never held %s", sdkacc.Alien)
	}

	fixtures := sweep.Fixtures{
		Path: map[string]string{
			"itemId":               string(movie.Id),
			"userId":               string(adminID),
			"seriesId":             string(series.Id),
			"playlistId":           created.Id,
			"Playlists/itemId":     created.Id,
			"taskId":               string(jfScanTask(ctx, t).Id),
			"pluginId":             plugins[0].Id,
			"version":              plugins[0].Version,
			"displayPreferencesId": "usersettings",
			"key":                  "encoding",
			"Persons/name":         sdkacc.RidleyScott,
			"Genres/genreName":     "Science Fiction",
			"Genres/name":          "Science Fiction",
			"Studios/name":         studios[0].Name,
			"Albums/itemId":        string(album.Id),
			"Artists/itemId":       string(musicArtists[0].Id),
			"Artists/name":         musicArtists[0].Name,
			"Songs/itemId":         string(song.Id),
			// the track with the .lrc beside it, so the lyric read has
			// something to answer with
			"Audio/itemId":          string(jfSong(t, musicID, sdkacc.LyricTrack).Id),
			"MusicGenres/genreName": musicGenre.Name,
			"MusicGenres/name":      musicGenre.Name,
			"year":                  "1982",
			"imageType":             "Primary",
			"imageIndex":            "0",
			"tag":                   item.ImageTags["Primary"],
			"format":                "jpg",
			"maxWidth":              "100",
			"maxHeight":             "100",
			"percentPlayed":         "0",
			"unplayedCount":         "0",
			"container":             "mp4",
			"mediaSourceId":         string(item.MediaSources[0].Id),
			"language":              "eng",
		},
		Options: map[string]string{
			"name":          logs[0].Name,
			"client":        "emby",
			"path":          "/media",
			"searchTerm":    sdkacc.Alien,
			"id":            devices[0].Id,
			"mediaSourceId": string(item.MediaSources[0].Id),
		},
		// an API key acts as no user, and Jellyfin answers 400 to a
		// user-scoped read that does not name one
		Always: map[string]string{"userId": string(adminID)},
	}

	cases := maps.Clone(jfSweepCases)
	cases["GetVideoStream"] = sweep.Case{Options: map[string]any{"Container": "mp4", "Static": true}}
	cases["GetVideoStreamByContainer"] = sweep.Case{Options: map[string]any{"Static": true}}
	cases["GetAudioStream"] = sweep.Case{Options: map[string]any{"Container": "mp3", "Static": true}}
	cases["GetAudioStreamByContainer"] = sweep.Case{Path: map[string]string{"container": "mp3"}, Options: map[string]any{"Static": true}}
	cases["GetUniversalAudioStream"] = sweep.Case{Options: map[string]any{"Container": "mp3", "UserId": adminID}}
	// the genre mix is the one route that takes the genre as a required id
	cases["GetInstantMixFromMusicGenreById"] = sweep.Case{Options: map[string]any{"Id": musicGenre.Id}}
	// a mix is made from music, and an album is like another by its
	// artist's other album
	cases["GetInstantMixFromItem"] = sweep.Case{Path: map[string]string{"itemId": string(album.Id)}}
	cases["GetInstantMixFromPlaylist"] = sweep.Case{Path: map[string]string{"itemId": music.Id}}
	cases["GetSimilarAlbums"] = sweep.Case{Path: map[string]string{"itemId": string(jfAlbum(t, musicID, sdkacc.DarkSide).Id)}}
	// the subtitle routes read the film with one
	subtitle := map[string]string{"routeItemId": string(subtitled.Id), "routeMediaSourceId": string(subtitled.MediaSources[0].Id), "routeIndex": srtIndex, "routeFormat": "srt", "routeStartPositionTicks": "0"}
	cases["GetSubtitle"] = sweep.Case{Path: subtitle}
	cases["GetSubtitleWithTicks"] = sweep.Case{Path: subtitle}
	cases["GetSubtitlePlaylist"] = sweep.Case{
		Path:    map[string]string{"itemId": string(subtitled.Id), "mediaSourceId": string(subtitled.MediaSources[0].Id), "index": srtIndex},
		Options: map[string]any{"SegmentLength": 30},
	}
	// a package from the catalogue, and the TMDb plugin's settings (the
	// first plugin's are one false flag)
	cases["GetPackageInfo"] = sweep.Case{Path: map[string]string{"name": packages[0].Name}}
	cases["GetPluginConfiguration"] = sweep.Case{Path: map[string]string{"pluginId": plugins[tmdbPlugin].Id}}
	// the folder listing lists files only unless asked for folders (/media
	// holds none), and the legacy filters answer for a library
	cases["GetDirectoryContents"] = sweep.Case{Options: map[string]any{"IncludeDirectories": true}}
	cases["GetQueryFiltersLegacy"] = sweep.Case{Options: map[string]any{"ParentId": moviesID}}

	sdkacc.RunSweep(t, "jellyfin", jfc, fixtures, cases, jfMissingIDs)
}

// jfMissingIDs are ids of each kind that nothing on a test server has, all 32 hex digits, and what Jellyfin answers each. An item is read
// the way the neutral client reads one, and a gone one is a 404, which is how a deleted item is told from an error; a task is a 404 too. A
// user and a media source are only noted: nothing of embyfin's reads either by its id.
var jfMissingIDs = map[string]sweep.Missing{
	"ItemID":        {ID: "0123456789abcdef0123456789abcdef", Operation: "GetItem", Want: "404"},
	"TaskID":        {ID: "0123456789abcdef0123456789abcdef", Want: "404"},
	"UserID":        {ID: "0123456789abcdef0123456789abcdef"},
	"MediaSourceID": {ID: "0123456789abcdef0123456789abcdef"},
}

// Why a Jellyfin GET does not simply answer in the sweep.
const (
	jfNoTuner         = "Live TV needs a tuner and a guide provider; the container has neither"
	jfNoImage         = "the fixture has no image of that type, which the server answers with an error"
	jfNoLyricProvider = "no lyric provider is installed, so there is nothing to search; the .lrc beside a fixture track covers the local read"
	jfNoChannel       = "no channel plugin is installed, so there is no channel to ask about"
	jfNoSyncPlay      = "SyncPlay needs a group, which needs a second connected client"
	jfKeyNotUser      = "Jellyfin checks a playlist's access against the calling user, and an API key acts as none, whatever userId says"
	jfNoBranding      = "no branding is set on a fresh server"
	jfNoExtras        = "the fixture films have no intros, trailers, extras or additional parts"
)

// jfSweepCases classifies the Jellyfin GETs that do not simply answer.
var jfSweepCases = map[string]sweep.Case{
	// Live TV
	"GetChannel":           {Skip: jfNoTuner},
	"GetLiveRecordingFile": {Skip: jfNoTuner},
	"GetLiveStreamFile":    {Skip: jfNoTuner},
	"GetProgram":           {Skip: jfNoTuner},
	"GetRecording":         {Skip: jfNoTuner},
	"GetSeriesTimer":       {Skip: jfNoTuner},
	"GetTimer":             {Skip: jfNoTuner},

	// music: the SDK Music library answers the rest (see TestJFReadSweep)
	// an artist has no image of its own, but a genre answers with one of its
	// items' images, so only the artist route needs classifying
	"GetArtistImage":     {Status: 404, Why: jfNoImage},
	"SearchRemoteLyrics": {Skip: jfNoLyricProvider},
	"GetRemoteLyrics":    {Skip: jfNoLyricProvider + " (and it takes an id from that search)"},

	"GetChannelFeatures": {Skip: jfNoChannel},
	"GetChannelItems":    {Skip: jfNoChannel},
	"SyncPlayGetGroup":   {Skip: jfNoSyncPlay},

	// Live TV reads that reach for a guide provider
	"GetChannelMappingOptions":      {Skip: jfNoTuner},
	"GetLineups":                    {Skip: jfNoTuner},
	"GetSchedulesDirectCountries":   {Skip: jfNoTuner + " (it asks Schedules Direct, through the provider proxy)"},
	"GetRecordingFolders":           {Skip: jfNoTuner},
	"GetQuickConnectState":          {Skip: "QuickConnect needs a secret from a second device's pairing request"},
	"GetFallbackFont":               {Skip: "no fallback font directory is configured, so there is no font to fetch"},
	"GetRemoteSubtitles":            {Skip: "needs a subtitle id from a provider search, and no subtitle provider is installed"},
	"GetTrickplayHlsPlaylist":       {Skip: "no trickplay tiles are generated for the fixtures"},
	"GetTrickplayTileImage":         {Skip: "no trickplay tiles are generated for the fixtures"},
	"GetAttachment":                 {Skip: "the fixture videos carry no attachments"},
	"GetBackup":                     {Status: 404, Why: "no backup has been made, so there is no manifest at the path"},
	"GetDashboardConfigurationPage": {Status: 404, Why: "no plugin configuration page is named; the route answers 404 without one"},
	"GetStudioImage":                {Status: 404, Why: "the fixture studio has no image"},
	"GetStudioImageByIndex":         {Status: 404, Why: "the fixture studio has no image"},
	"GetCurrentUser":                {Status: 400, Why: "an API key acts as no user, so there is no current user"},
	"SyncPlayGetGroups":             {Skip: jfNoSyncPlay},
	"GetDeviceOptions":              {Status: 404, Why: "no options are stored for the fixture device until a client sets them"},
	"GetUserImage":                  {Status: 404, Why: "the fixture user has no image"},
	"GetPlaylist":                   {Status: 400, Why: jfKeyNotUser},
	"GetPlaylistUsers":              {Status: 400, Why: jfKeyNotUser},
	"GetPlaylistUser":               {Status: 400, Why: jfKeyNotUser},
	"GetTrailers":                   {Status: 500, Why: "the trailers route reads the calling user's claims, which an API key does not have (a NullReferenceException in GetIsApiKey), even with a userId"},

	// the GETs that answer, with nothing in it, on a fresh server and the
	// fixtures: what they read is not there to read
	"DiscoverTuners":             {Empty: jfNoTuner},
	"DiscvoverTuners":            {Empty: jfNoTuner},
	"GetLiveTvChannels":          {Empty: jfNoTuner},
	"GetLiveTvPrograms":          {Empty: jfNoTuner},
	"GetRecommendedPrograms":     {Empty: jfNoTuner},
	"GetRecordings":              {Empty: jfNoTuner},
	"GetSeriesTimers":            {Empty: jfNoTuner},
	"GetTimers":                  {Empty: jfNoTuner},
	"GetAllChannelFeatures":      {Empty: jfNoChannel},
	"GetChannels":                {Empty: jfNoChannel},
	"GetLatestChannelItems":      {Empty: jfNoChannel},
	"ListBackups":                {Empty: "no backup has been made"},
	"GetBrandingCss":             {Empty: jfNoBranding},
	"GetBrandingCss2":            {Empty: jfNoBranding},
	"GetBrandingOptions":         {Empty: jfNoBranding},
	"GetDefaultDirectoryBrowser": {Empty: "the default browser path is empty on Linux"},
	"GetDefaultMetadataOptions":  {Empty: "the defaults a new item type starts from are blank"},
	"GetFallbackFontList":        {Empty: "no fallback font folder is set"},
	"GetPublicUsers":             {Empty: "every user is hidden from the login screen (the users test shows one and hides her again)"},
	"GetResumeItems":             {Empty: "nothing is in progress (the played and resume test clears the position it sets)"},
	"GetNextUp":                  {Empty: "no episode is played (the shows test unmarks the one it marks)"},
	"GetUpcomingEpisodes":        {Empty: "the show library has its fetchers off, so no episode is known to be coming"},
	"GetMovieRecommendations":    {Empty: "no film is watched or liked, which recommendations start from"},
	"SearchRemoteSubtitles":      {Empty: "no subtitle provider is installed"},
	"GetItemSegments":            {Empty: "no media segment provider is installed, so no film has an intro or credits marked"},
	"GetIntros":                  {Empty: jfNoExtras},
	"GetLocalTrailers":           {Empty: jfNoExtras},
	"GetSpecialFeatures":         {Empty: jfNoExtras},
	"GetAdditionalPart":          {Empty: jfNoExtras},
}
