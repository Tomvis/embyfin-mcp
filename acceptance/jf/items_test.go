//go:build acceptance

package jf_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/go-kt/pointer"
	"github.com/katbyte/pandorest/client"
)

// TestJFItemsQuery drives /Items through every parameter the neutral layer
// uses, against the facts in the fixture table.
//

func TestJFItemsQuery(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	moviesID := jfLibrary(t, sdkacc.SDKMovies)
	jfLibrary(t, sdkacc.SDKShows) // the counts below span both
	fields := []jf.ItemFields{jf.ItemFieldsPath, jf.ItemFieldsProviderIds, jf.ItemFieldsOverview, jf.ItemFieldsGenres, jf.ItemFieldsTags, jf.ItemFieldsPeople, jf.ItemFieldsStudios, jf.ItemFieldsDateCreated}

	all := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{
		ParentId: moviesID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindMovie},
		Fields: fields, SortBy: []jf.ItemSortBy{jf.ItemSortByName}, SortOrder: []jf.SortOrder{jf.SortOrderAscending},
	})).Answer
	if all.TotalRecordCount != len(sdkacc.Movies) || len(all.Items) != len(sdkacc.Movies) {
		t.Fatalf("movies = %d/%d, want %d", len(all.Items), all.TotalRecordCount, len(sdkacc.Movies))
	}
	byTitle := map[string]jf.BaseItemDto{}
	for _, it := range all.Items {
		byTitle[it.Name] = it
		if it.Id == "" || it.Type != jf.BaseItemKindMovie || it.Path == "" || it.Overview == "" || it.DateCreated == "" || it.ServerId == "" {
			t.Errorf("%s did not decode: %+v", it.Name, it)
		}
		if it.MediaType != jf.MediaTypeVideo || it.LocationType != jf.LocationTypeFileSystem {
			t.Errorf("%s: MediaType %q LocationType %q", it.Name, it.MediaType, it.LocationType)
		}
	}
	for _, m := range sdkacc.Movies {
		it, ok := byTitle[m.Title]
		if !ok {
			t.Errorf("%s is missing from the scan", m.Title)
			continue
		}
		if it.ProductionYear != m.Year || it.ProviderIds["Tmdb"] != m.TMDB || it.ProviderIds["Imdb"] != m.IMDB {
			t.Errorf("%s: year %d, provider ids %v; the nfo says %d/%s/%s", m.Title, it.ProductionYear, it.ProviderIds, m.Year, m.TMDB, m.IMDB)
		}
		if !slices.Contains(it.Genres, m.Genre) {
			t.Errorf("%s: genres %v lack %q", m.Title, it.Genres, m.Genre)
		}
		if !slices.ContainsFunc(it.People, func(p jf.BaseItemPerson) bool { return p.Name == m.Director && p.Type == jf.PersonKindDirector }) {
			t.Errorf("%s: people %+v lack director %s", m.Title, it.People, m.Director)
		}
	}
	// the sort order held
	if all.Items[0].Name != sdkacc.Alien || all.Items[len(all.Items)-1].Name != "The Thirteenth Floor" {
		t.Errorf("sorted by name: first %q, last %q", all.Items[0].Name, all.Items[len(all.Items)-1].Name)
	}

	// the providers filled what the nfo does not carry: a rating and the
	// studios (the cast is not merged, since the nfo already named a person)
	if a := byTitle[sdkacc.Alien]; a.CommunityRating == 0 || len(a.Studios) == 0 || a.Studios[0].Name == "" {
		t.Errorf("%s has no rating or studios after a scan with providers on: rating %v, studios %+v", sdkacc.Alien, a.CommunityRating, a.Studios)
	}

	t.Run("SearchTerm", func(t *testing.T) {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: moviesID, Recursive: new(true), SearchTerm: "Dune"})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("SearchTerm=Dune found %d, want Dune and Dune: Part Two", res.TotalRecordCount)
		}
	})
	t.Run("Ids", func(t *testing.T) {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{Ids: []string{byTitle[sdkacc.Alien].Id, byTitle[sdkacc.Aliens].Id}})).Answer
		if len(res.Items) != 2 {
			t.Errorf("Ids found %d, want 2", len(res.Items))
		}
	})
	t.Run("Paging", func(t *testing.T) {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{
			ParentId: moviesID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindMovie},
			SortBy: []jf.ItemSortBy{jf.ItemSortByProductionYear}, SortOrder: []jf.SortOrder{jf.SortOrderDescending}, StartIndex: new(2), Limit: new(3),
		})).Answer
		if len(res.Items) != 3 || res.TotalRecordCount != len(sdkacc.Movies) || res.StartIndex != 2 {
			t.Errorf("StartIndex=2 Limit=3: %d items of %d from %d", len(res.Items), res.TotalRecordCount, res.StartIndex)
		}
		// newest first: 2024, 2021, then the page starts at 2016
		if res.Items[0].ProductionYear != 2016 {
			t.Errorf("third newest is %s (%d), want Arrival (2016)", res.Items[0].Name, res.Items[0].ProductionYear)
		}
	})
	t.Run("Genres", func(t *testing.T) {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: moviesID, Recursive: new(true), Genres: []string{"Horror", "Animation"}})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("Genres=Horror,Animation found %d, want Alien and Princess Mononoke", res.TotalRecordCount)
		}
	})
	t.Run("Years", func(t *testing.T) {
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: moviesID, Recursive: new(true), Years: []int{1982, 1999}})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("Years=1982,1999 found %d, want Blade Runner and The Thirteenth Floor", res.TotalRecordCount)
		}
	})
	t.Run("PersonIds", func(t *testing.T) {
		people := sdkacc.Must(jfc.Person.GetPersons(ctx, jf.GetPersonsOptions{SearchTerm: sdkacc.RidleyScott})).Answer
		i := slices.IndexFunc(people.Items, func(p jf.BaseItemDto) bool { return p.Name == sdkacc.RidleyScott })
		if i < 0 {
			t.Fatalf("GetPersons(%s) = %+v", sdkacc.RidleyScott, people.Items)
		}
		if people.Items[i].Id == "" || people.Items[i].Type != jf.BaseItemKindPerson {
			t.Errorf("person did not decode: %+v", people.Items[i])
		}
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: moviesID, Recursive: new(true), PersonIds: []string{people.Items[i].Id}})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("PersonIds=%s found %d, want Alien and Blade Runner", sdkacc.RidleyScott, res.TotalRecordCount)
		}
	})
	t.Run("IsFavorite", func(t *testing.T) {
		id := byTitle[sdkacc.BladeRunner].Id
		if ud := sdkacc.Must(jfc.UserData.MarkFavoriteItem(ctx, id, adminID)).Answer; !pointer.From(ud.IsFavorite) {
			t.Error("MarkFavoriteItem returned IsFavorite=false")
		}
		res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{
			UserId: adminID, ParentId: moviesID, Recursive: new(true), Filters: []jf.ItemFilter{jf.ItemFilterIsFavorite},
		})).Answer
		if len(res.Items) != 1 || res.Items[0].Id != id {
			t.Errorf("Filters=IsFavorite found %+v, want %s", res.Items, sdkacc.BladeRunner)
		}
		if res.Items[0].UserData == nil || !pointer.From(res.Items[0].UserData.IsFavorite) {
			t.Errorf("UserData = %+v, want IsFavorite", res.Items[0].UserData)
		}
		if ud := sdkacc.Must(jfc.UserData.UnmarkFavoriteItem(ctx, id, adminID)).Answer; pointer.From(ud.IsFavorite) {
			t.Error("UnmarkFavoriteItem returned IsFavorite=true")
		}
		if res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{UserId: adminID, ParentId: moviesID, Recursive: new(true), IsFavorite: new(true)})).Answer; len(res.Items) != 0 {
			t.Errorf("IsFavorite=true still finds %d after unfavouriting", len(res.Items))
		}
	})
	t.Run("Counts", func(t *testing.T) {
		// the counts are the whole server's, which may hold more libraries
		// than these (the messy ones, a run's that crashed): each is what the
		// user's own item query counts, and at least the fixtures'
		counts := sdkacc.Must(jfc.Library.GetItemCounts(ctx, jf.GetItemCountsOptions{UserId: adminID})).Answer
		count := func(kind jf.BaseItemKind) int {
			return sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{UserId: adminID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{kind}, Limit: new(1)})).Answer.TotalRecordCount
		}
		if m, s, e := count(jf.BaseItemKindMovie), count(jf.BaseItemKindSeries), count(jf.BaseItemKindEpisode); counts.MovieCount != m || counts.SeriesCount != s || counts.EpisodeCount != e {
			t.Errorf("counts = %+v, and the user's items count %d films, %d series, %d episodes", counts, m, s, e)
		}
		if counts.MovieCount < len(sdkacc.Movies) || counts.SeriesCount < len(sdkacc.Shows) || counts.EpisodeCount < sdkacc.SDKShows.Episodes {
			t.Errorf("counts = %+v, fewer than the fixtures", counts)
		}
	})
}

// TestJFItem covers the single-item shape, the update round trip, and the
// reads that hang off one item.
//

func TestJFItem(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	id := jfMovie(t, sdkacc.Aliens).Id
	// the album the instant mix is made from
	musicID := jfLibrary(t, sdkacc.SDKMusic)
	album := jfAlbum(t, musicID, sdkacc.DarkSide)

	full := sdkacc.Must(jfc.Library.GetItem(ctx, id, adminID)).Answer
	if full.Name != sdkacc.Aliens || full.ProductionYear != 1986 || full.Path == "" || full.Overview == "" || len(full.MediaSources) == 0 || full.RunTimeTicks == 0 {
		t.Fatalf("GetItem did not decode the full shape: %+v", full)
	}
	if full.UserData == nil {
		t.Error("GetItem with UserId has no UserData")
	}
	if len(full.MediaSources[0].MediaStreams) == 0 || full.MediaSources[0].Container == "" {
		t.Errorf("MediaSources[0] = %+v, want streams and a container", full.MediaSources[0])
	}

	// update: post the DTO back with three fields changed
	original := *full
	full.Overview, full.Genres, full.Tags = "SDK overview", []string{"SDK Genre", "Action"}, []string{"sdk"}
	if _, err := jfc.ItemUpdate.UpdateItem(ctx, id, *full); err != nil {
		t.Fatal(err)
	}
	sdkacc.UndoLater(t, "putting the item back", func() error {
		_, err := jfc.ItemUpdate.UpdateItem(context.WithoutCancel(ctx), id, original)
		return err
	})
	got := sdkacc.Must(jfc.Library.GetItem(ctx, id, adminID)).Answer
	if got.Overview != "SDK overview" || !slices.Equal(got.Genres, []string{"SDK Genre", "Action"}) || !slices.Equal(got.Tags, []string{"sdk"}) {
		t.Errorf("after UpdateItem: overview %q, genres %v, tags %v", got.Overview, got.Genres, got.Tags)
	}
	if got.Name != sdkacc.Aliens || got.ProductionYear != 1986 || got.ProviderIds["Tmdb"] != "679" {
		t.Errorf("UpdateItem disturbed the fields it should not have: %+v", got)
	}

	// a full refresh with ReplaceAllMetadata puts the provider's overview back
	if _, err := jfc.Library.RefreshItem(ctx, id, jf.RefreshItemOptions{
		MetadataRefreshMode: jf.MetadataRefreshModeFullRefresh, ImageRefreshMode: jf.MetadataRefreshModeDefault, ReplaceAllMetadata: new(true),
	}); err != nil {
		t.Fatal(err)
	}
	// (GetItem needs a UserId: without one the server answers 500, "Guid
	// can't be empty", rather than serving the item without user data)
	if !sdkacc.Poll(2*time.Minute, func() bool {
		it, err := jfc.Library.GetItem(ctx, id, adminID)
		return err == nil && it.Answer.Overview != "SDK overview" && it.Answer.Overview != ""
	}) {
		t.Error("RefreshItem never replaced the overview")
	}

	t.Run("Similar", func(t *testing.T) {
		res := sdkacc.Must(jfc.Library.GetSimilarItems(ctx, id, jf.GetSimilarItemsOptions{UserId: adminID, Limit: new(3)})).Answer
		if len(res.Items) == 0 {
			t.Fatal("GetSimilarItems found nothing among seven other films")
		}
		for _, it := range res.Items {
			if it.Id == "" || it.Name == "" || it.Id == id {
				t.Errorf("similar item = %+v", it)
			}
		}
	})
	t.Run("InstantMix", func(t *testing.T) {
		// a mix is music: from a film there is none
		if res := sdkacc.Must(jfc.InstantMix.GetInstantMixFromItem(ctx, id, jf.GetInstantMixFromItemOptions{UserId: adminID, Limit: new(5)})).Answer; len(res.Items) != 0 {
			t.Errorf("an instant mix from a film = %+v, want nothing", res.Items)
		}
		// from an album it is the songs of the album's genre, the other
		// album by the same artist's included
		mix := sdkacc.Must(jfc.InstantMix.GetInstantMixFromItem(ctx, album.Id, jf.GetInstantMixFromItemOptions{UserId: adminID, Limit: new(50)})).Answer
		genre := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{ParentId: musicID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindAudio}, Genres: []string{sdkacc.ProgressiveRock}})).Answer
		if len(genre.Items) == 0 {
			t.Fatalf("no song in %s", sdkacc.ProgressiveRock)
		}
		want := map[string]bool{}
		for _, s := range genre.Items {
			want[s.Id] = true
		}
		got := map[string]bool{}
		for _, it := range mix.Items {
			if it.Type != jf.BaseItemKindAudio || !want[it.Id] {
				t.Errorf("the mix from %s holds %s (%s), which is not a %s song", sdkacc.DarkSide, it.Name, it.Type, sdkacc.ProgressiveRock)
			}
			got[it.Id] = true
		}
		if len(got) != len(want) {
			t.Errorf("the mix from %s holds %d of the %d %s songs", sdkacc.DarkSide, len(got), len(want), sdkacc.ProgressiveRock)
		}
	})
	t.Run("Images", func(t *testing.T) {
		images := sdkacc.Must(jfc.Image.GetItemImageInfos(ctx, id)).Answer
		i := slices.IndexFunc(images, func(img jf.ImageInfo) bool { return img.ImageType == jf.ImageTypePrimary })
		if i < 0 {
			t.Fatalf("no Primary image among %+v; the fixture has a poster.jpg", images)
		}
		if images[i].Path == "" || images[i].Width == 0 || images[i].Height == 0 || images[i].Size == 0 {
			t.Errorf("Primary image = %+v", images[i])
		}
	})
	t.Run("ExternalIds", func(t *testing.T) {
		infos := sdkacc.Must(jfc.ItemLookup.GetExternalIdInfos(ctx, id)).Answer
		if !slices.ContainsFunc(infos, func(e jf.ExternalIdInfo) bool { return e.Key == "Tmdb" && e.Name != "" }) {
			t.Errorf("GetExternalIdInfos = %+v, want a Tmdb entry with a url format", infos)
		}
	})
	t.Run("Ancestors", func(t *testing.T) {
		anc := sdkacc.Must(jfc.Library.GetAncestors(ctx, id, adminID)).Answer
		if !slices.ContainsFunc(anc, func(a jf.BaseItemDto) bool { return a.Name == sdkacc.SDKMovies.Name }) {
			t.Errorf("GetAncestors = %+v, want %s among them", anc, sdkacc.SDKMovies.Name)
		}
	})
	t.Run("PlaybackInfo", func(t *testing.T) {
		info := sdkacc.Must(jfc.MediaInfo.GetPlaybackInfo(ctx, id, adminID)).Answer
		if len(info.MediaSources) == 0 || info.PlaySessionId == "" {
			t.Errorf("GetPlaybackInfo = %+v", info)
		}
	})
	t.Run("UserData", func(t *testing.T) {
		ud := sdkacc.Must(jfc.UserData.GetItemUserData(ctx, id, adminID)).Answer
		if ud.ItemId != id || ud.Key == "" {
			t.Errorf("GetItemUserData = %+v", ud)
		}
	})
	t.Run("Extras", func(t *testing.T) {
		// none of these have content for a bare movie; they must still answer
		if _, err := jfc.Library.GetThemeMedia(ctx, id, jf.GetThemeMediaOptions{UserId: adminID}); err != nil {
			t.Error(err)
		}
		if extras := sdkacc.Must(jfc.Library.GetSpecialFeatures(ctx, id, adminID)).Answer; len(extras) != 0 {
			t.Errorf("GetSpecialFeatures = %+v", extras)
		}
		if trailers := sdkacc.Must(jfc.Library.GetLocalTrailers(ctx, id, adminID)).Answer; len(trailers) != 0 {
			t.Errorf("GetLocalTrailers = %+v", trailers)
		}
		if intros := sdkacc.Must(jfc.Library.GetIntros(ctx, id, adminID)).Answer; len(intros.Items) != 0 {
			t.Errorf("GetIntros = %+v", intros)
		}
		if parts := sdkacc.Must(jfc.Video.GetAdditionalPart(ctx, id, adminID)).Answer; len(parts.Items) != 0 {
			t.Errorf("GetAdditionalPart = %+v", parts)
		}
		if segs := sdkacc.Must(jfc.MediaSegment.GetItemSegments(ctx, id)).Answer; len(segs.Items) != 0 {
			t.Errorf("GetItemSegments = %+v", segs)
		}
		// a movie has no lyrics: a 404, not a decode failure
		if _, err := jfc.Lyric.GetLyrics(ctx, id); !client.IsNotFound(err) {
			t.Errorf("GetLyrics on a movie = %v, want a 404", err)
		}
	})
}

// TestJFCatalogue covers the by-name folders and the browse endpoints.
//

func TestJFCatalogue(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	moviesID := jfLibrary(t, sdkacc.SDKMovies)

	genres := sdkacc.Must(jfc.Genre.GetGenres(ctx, jf.GetGenresOptions{ParentId: moviesID, UserId: adminID})).Answer
	if !slices.ContainsFunc(genres.Items, func(g jf.BaseItemDto) bool { return g.Name == "Horror" && g.Id != "" && g.Type == jf.BaseItemKindGenre }) {
		t.Errorf("GetGenres = %+v, want Horror", genres.Items)
	}
	studios := sdkacc.Must(jfc.Studio.GetStudios(ctx, jf.GetStudiosOptions{ParentId: moviesID, UserId: adminID})).Answer
	if len(studios.Items) == 0 || studios.Items[0].Name == "" || studios.Items[0].Id == "" {
		t.Errorf("GetStudios = %+v, want the studios TMDB fills in", studios.Items)
	}
	years := sdkacc.Must(jfc.Year.GetYears(ctx, jf.GetYearsOptions{ParentId: moviesID, Recursive: new(true), UserId: adminID})).Answer
	if !slices.ContainsFunc(years.Items, func(y jf.BaseItemDto) bool { return y.Name == "1979" }) {
		t.Errorf("GetYears = %+v, want 1979", years.Items)
	}
	filters := sdkacc.Must(jfc.Filter.GetQueryFilters(ctx, jf.GetQueryFiltersOptions{ParentId: moviesID, UserId: adminID, Recursive: new(true)})).Answer
	if !slices.ContainsFunc(filters.Genres, func(g jf.NameGuidPair) bool { return g.Name == "Horror" && g.Id != "" }) {
		t.Errorf("GetQueryFilters = %+v, want Horror among the genres", filters)
	}
	legacy := sdkacc.Must(jfc.Filter.GetQueryFiltersLegacy(ctx, jf.GetQueryFiltersLegacyOptions{ParentId: moviesID, UserId: adminID})).Answer
	if !slices.Contains(legacy.Genres, "Horror") || !slices.Contains(legacy.Years, 1979) {
		t.Errorf("GetQueryFiltersLegacy = %+v", legacy)
	}
	hints := sdkacc.Must(jfc.Search.GetSearchHints(ctx, jf.GetSearchHintsOptions{SearchTerm: "Alien", UserId: adminID, IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindMovie}})).Answer
	if hints.TotalRecordCount < 2 || !slices.ContainsFunc(hints.SearchHints, func(h jf.SearchHint) bool { return h.Name == sdkacc.Alien && h.Id != "" && h.ProductionYear == 1979 }) {
		t.Errorf("GetSearchHints(Alien) = %+v", hints)
	}
	latest := sdkacc.Must(jfc.Library.GetLatestMedia(ctx, jf.GetLatestMediaOptions{UserId: adminID, ParentId: moviesID, Limit: new(3)})).Answer
	if len(latest) != 3 || latest[0].Id == "" {
		t.Errorf("GetLatestMedia = %d items", len(latest))
	}
	suggestions := sdkacc.Must(jfc.Suggestion.GetSuggestions(ctx, jf.GetSuggestionsOptions{UserId: adminID, Type: []jf.BaseItemKind{jf.BaseItemKindMovie}, Limit: new(2)})).Answer
	if len(suggestions.Items) == 0 {
		t.Error("GetSuggestions returned nothing")
	}
	root := sdkacc.Must(jfc.Library.GetRootFolder(ctx, adminID)).Answer
	if root.Id == "" || !pointer.From(root.IsFolder) {
		t.Errorf("GetRootFolder = %+v", root)
	}
	views := sdkacc.Must(jfc.UserView.GetUserViews(ctx, jf.GetUserViewsOptions{UserId: adminID})).Answer
	if !slices.ContainsFunc(views.Items, func(v jf.BaseItemDto) bool { return v.Id == moviesID && v.CollectionType == jf.CollectionTypeMovies }) {
		t.Errorf("GetUserViews = %+v, want %s", views.Items, sdkacc.SDKMovies.Name)
	}

	// /Trailers reads the calling user's claims, which an API key has none
	// of: a NullReferenceException behind a 500, userId or not
	if _, err := jfc.Trailer.GetTrailers(ctx, jf.GetTrailersOptions{UserId: adminID}); client.StatusCode(err) != 500 { //nolint:staticcheck // deprecated in the document and still an operation the SDK serves, which is what this holds it to
		t.Errorf("GetTrailers with an API key = %v, want a 500", err)
	}
	// no channel plugin is installed
	if res := sdkacc.Must(jfc.Channel.GetChannels(ctx, jf.GetChannelsOptions{UserId: adminID})).Answer; res.TotalRecordCount != 0 || len(res.Items) != 0 {
		t.Errorf("GetChannels = %+v", res)
	}

	// the music families, read by library: none in the film library, and the
	// fixtures' in the music one (whatever else the server holds, so the
	// counts do not depend on what ran before). Jellyfin documents no music
	// genre listing: /Genres of a music library lists its music genres.
	for _, lib := range []string{moviesID, jfLibrary(t, sdkacc.SDKMusic)} {
		wantArtists, wantAlbumArtists, wantGenres := []string{}, []string{}, []string{}
		if lib != moviesID {
			// the tracks' artists, The Pink Floyd among them (one track's
			// tag spells it so), and the albums': Various Artists over Battle
			// Tapes' Polygon, which Jellyfin lists as an album artist alone
			wantArtists = []string{"Battle Tapes", "Coyote Kisses", "Pink Floyd", "SirensCeol", "The Pink Floyd"}
			wantAlbumArtists = []string{"Coyote Kisses", "Pink Floyd", "SirensCeol", "Various Artists"}
			wantGenres = []string{"Electronic", "Electronica", sdkacc.ProgressiveRock}
		}
		names := func(items []jf.BaseItemDto) []string {
			out := make([]string, 0, len(items))
			for i := range items {
				out = append(out, items[i].Name)
			}
			slices.Sort(out)
			return out
		}
		artists := sdkacc.Must(jfc.Artist.GetArtists(ctx, jf.GetArtistsOptions{ParentId: lib, UserId: adminID})).Answer //nolint:staticcheck // deprecated in the document and still an operation the SDK serves, which is what this holds it to
		if got := names(artists.Items); !slices.Equal(got, wantArtists) || artists.TotalRecordCount != len(wantArtists) {
			t.Errorf("GetArtists(%s) = %v (%d), want %v", lib, got, artists.TotalRecordCount, wantArtists)
		}
		albumArtists := sdkacc.Must(jfc.Artist.GetAlbumArtists(ctx, jf.GetAlbumArtistsOptions{ParentId: lib, UserId: adminID})).Answer //nolint:staticcheck // deprecated in the document and still an operation the SDK serves, which is what this holds it to
		if got := names(albumArtists.Items); !slices.Equal(got, wantAlbumArtists) || albumArtists.TotalRecordCount != len(wantAlbumArtists) {
			t.Errorf("GetAlbumArtists(%s) = %v (%d), want %v", lib, got, albumArtists.TotalRecordCount, wantAlbumArtists)
		}
		if lib == moviesID {
			continue // the film library's genres are the films'
		}
		genres := sdkacc.Must(jfc.Genre.GetGenres(ctx, jf.GetGenresOptions{ParentId: lib, UserId: adminID})).Answer
		if got := names(genres.Items); !slices.Equal(got, wantGenres) || genres.Items[0].Type != jf.BaseItemKindMusicGenre {
			t.Errorf("GetGenres(%s) = %v (%+v), want the music genres %v", lib, got, genres.Items, wantGenres)
		}
	}
}
