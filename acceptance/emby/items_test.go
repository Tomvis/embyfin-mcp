//go:build acceptance

package emby_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
	"github.com/katbyte/go-kt/pointer"
	"github.com/katbyte/pandorest/client"
)

// TestEmbyItemsQuery drives /Items and the per-user /Users/{id}/Items
// through every parameter the neutral layer uses, against the facts in the
// fixture table.
//

func TestEmbyItemsQuery(t *testing.T) {
	ctx := skipUnlessEmby(t)
	moviesID := embyLibrary(t, sdkacc.SDKMovies)
	embyLibrary(t, sdkacc.SDKShows) // the counts below span both
	// (ProductionYear is not among the fields the server sends by default)
	const fields = "Path,ProviderIds,Overview,Genres,Tags,People,Studios,DateCreated,CommunityRating,ProductionYear"

	all := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{
		ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie",
		Fields: fields, SortBy: "SortName", SortOrder: "Ascending",
	})).Answer
	if all.TotalRecordCount != len(sdkacc.Movies) || len(all.Items) != len(sdkacc.Movies) {
		t.Fatalf("movies = %d/%d, want %d", len(all.Items), all.TotalRecordCount, len(sdkacc.Movies))
	}
	byTitle := map[string]emby.BaseItemDto{}
	for _, it := range all.Items {
		byTitle[it.Name] = it
		if it.Id == "" || it.Type != "Movie" || it.Path == "" || it.Overview == "" || it.DateCreated == "" || it.ServerId == "" {
			t.Errorf("%s did not decode: %+v", it.Name, it)
		}
		if it.MediaType != "Video" {
			t.Errorf("%s: MediaType %q", it.Name, it.MediaType)
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
		if !slices.ContainsFunc(it.People, func(p emby.BaseItemPerson) bool { return p.Name == m.Director && p.Type == "Director" }) {
			t.Errorf("%s: people %+v lack director %s", m.Title, it.People, m.Director)
		}
	}
	if all.Items[0].Name != sdkacc.Alien || all.Items[len(all.Items)-1].Name != "The Thirteenth Floor" {
		t.Errorf("sorted by name: first %q, last %q", all.Items[0].Name, all.Items[len(all.Items)-1].Name)
	}

	// the providers filled what the nfo does not carry
	if a := byTitle[sdkacc.Alien]; a.CommunityRating == 0 || len(a.Studios) == 0 || a.Studios[0].Name == "" {
		t.Errorf("%s has no rating or studios after a scan with providers on: rating %v, studios %+v", sdkacc.Alien, a.CommunityRating, a.Studios)
	}

	t.Run("PerUser", func(t *testing.T) {
		res := sdkacc.Must(embyc.Items.GetUsersByUserIdItems(ctx, adminID, emby.GetUsersByUserIdItemsOptions{
			ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie", Fields: "Path",
		})).Answer
		if res.TotalRecordCount != len(sdkacc.Movies) {
			t.Errorf("/Users/{id}/Items found %d, want %d", res.TotalRecordCount, len(sdkacc.Movies))
		}
		for _, it := range res.Items {
			if it.UserData == nil || it.Path == "" {
				t.Errorf("%s in the per-user shape has no UserData or Path", it.Name)
			}
		}
	})
	t.Run("NameStartsWith", func(t *testing.T) {
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie", NameStartsWith: "Dune"})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("NameStartsWith=Dune found %d, want Dune and Dune: Part Two", res.TotalRecordCount)
		}
	})
	t.Run("SearchTerm", func(t *testing.T) {
		// the Emby spec has no SearchTerm on /Items (the server reads one);
		// the emby-search-term workaround adds it to the generated options
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie", SearchTerm: "Dune"})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("SearchTerm=Dune found %d, want Dune and Dune: Part Two", res.TotalRecordCount)
		}
	})
	t.Run("Ids", func(t *testing.T) {
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{Ids: byTitle[sdkacc.Alien].Id + "," + byTitle[sdkacc.Aliens].Id})).Answer
		if len(res.Items) != 2 {
			t.Errorf("Ids found %d, want 2", len(res.Items))
		}
	})
	t.Run("Paging", func(t *testing.T) {
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{
			ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie",
			SortBy: "ProductionYear", SortOrder: "Descending", StartIndex: new(2), Limit: new(3), Fields: "ProductionYear",
		})).Answer
		if len(res.Items) != 3 || res.TotalRecordCount != len(sdkacc.Movies) {
			t.Errorf("StartIndex=2 Limit=3: %d items of %d", len(res.Items), res.TotalRecordCount)
		}
		if res.Items[0].ProductionYear != 2016 {
			t.Errorf("third newest is %s (%d), want Arrival (2016)", res.Items[0].Name, res.Items[0].ProductionYear)
		}
	})
	t.Run("Genres", func(t *testing.T) {
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), Genres: "Horror|Animation"})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("Genres=Horror|Animation found %d, want Alien and Princess Mononoke", res.TotalRecordCount)
		}
	})
	t.Run("Years", func(t *testing.T) {
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), Years: "1982,1999"})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("Years=1982,1999 found %d, want Blade Runner and The Thirteenth Floor", res.TotalRecordCount)
		}
	})
	t.Run("PersonIds", func(t *testing.T) {
		people := sdkacc.Must(embyc.Persons.GetPersons(ctx, emby.GetPersonsOptions{NameStartsWith: sdkacc.RidleyScott})).Answer
		i := slices.IndexFunc(people.Items, func(p emby.BaseItemDto) bool { return p.Name == sdkacc.RidleyScott })
		if i < 0 {
			t.Fatalf("GetPersons(%s) = %+v", sdkacc.RidleyScott, people.Items)
		}
		if people.Items[i].Id == "" || people.Items[i].Type != "Person" {
			t.Errorf("person did not decode: %+v", people.Items[i])
		}
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), PersonIds: people.Items[i].Id})).Answer
		if res.TotalRecordCount != 2 {
			t.Errorf("PersonIds=%s found %d, want Alien and Blade Runner", sdkacc.RidleyScott, res.TotalRecordCount)
		}
	})
	t.Run("AnyProviderIdEquals", func(t *testing.T) {
		// within the film library: the messy one holds two more copies
		res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), AnyProviderIdEquals: "imdb.tt0078748"})).Answer
		if len(res.Items) != 1 || res.Items[0].Name != sdkacc.Alien || res.Items[0].Id != byTitle[sdkacc.Alien].Id {
			t.Errorf("AnyProviderIdEquals=imdb.tt0078748 found %+v, want %s", res.Items, sdkacc.Alien)
		}
	})
	t.Run("IsFavorite", func(t *testing.T) {
		id := byTitle[sdkacc.BladeRunner].Id
		if ud := sdkacc.Must(embyc.UserLibrary.PostUsersByUserIdFavoriteItemsById(ctx, adminID, id)).Answer; !pointer.From(ud.IsFavorite) {
			t.Error("favouriting returned IsFavorite=false")
		}
		res := sdkacc.Must(embyc.Items.GetUsersByUserIdItems(ctx, adminID, emby.GetUsersByUserIdItemsOptions{ParentId: moviesID, Recursive: new(true), Filters: "IsFavorite"})).Answer
		if len(res.Items) != 1 || res.Items[0].Id != id {
			t.Errorf("Filters=IsFavorite found %+v, want %s", res.Items, sdkacc.BladeRunner)
		}
		if res.Items[0].UserData == nil || !pointer.From(res.Items[0].UserData.IsFavorite) {
			t.Errorf("UserData = %+v, want IsFavorite", res.Items[0].UserData)
		}
		if ud := sdkacc.Must(embyc.UserLibrary.DeleteUsersByUserIdFavoriteItemsById(ctx, adminID, id)).Answer; pointer.From(ud.IsFavorite) {
			t.Error("unfavouriting returned IsFavorite=true")
		}
		if res := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{UserId: adminID, ParentId: moviesID, Recursive: new(true), IsFavorite: new(true)})).Answer; len(res.Items) != 0 {
			t.Errorf("IsFavorite=true still finds %d after unfavouriting", len(res.Items))
		}
	})
	t.Run("Counts", func(t *testing.T) {
		// the counts are the whole server's, which may hold more libraries
		// than these (the messy ones, a run's that crashed): each is what the
		// user's own item query counts, and at least the fixtures'
		counts := sdkacc.Must(embyc.Library.GetItemsCounts(ctx, emby.GetItemsCountsOptions{UserId: adminID})).Answer
		count := func(kind string) int {
			return sdkacc.Must(embyc.Items.GetUsersByUserIdItems(ctx, adminID, emby.GetUsersByUserIdItemsOptions{Recursive: new(true), IncludeItemTypes: kind, Limit: new(1)})).Answer.TotalRecordCount
		}
		if m, s, e := count("Movie"), count("Series"), count("Episode"); counts.MovieCount != m || counts.SeriesCount != s || counts.EpisodeCount != e {
			t.Errorf("counts = %+v, and the user's items count %d films, %d series, %d episodes", counts, m, s, e)
		}
		if counts.MovieCount < len(sdkacc.Movies) || counts.SeriesCount < len(sdkacc.Shows) || counts.EpisodeCount < sdkacc.SDKShows.Episodes {
			t.Errorf("counts = %+v, fewer than the fixtures", counts)
		}

		// a limit of none answers the total alone, on the plain route and the user's: the same count as a limit of one, and no item fetched
		for _, kind := range []string{"Movie", "Series", "Episode"} {
			plain := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{Recursive: new(true), IncludeItemTypes: kind, Limit: new(0)})).Answer
			users := sdkacc.Must(embyc.Items.GetUsersByUserIdItems(ctx, adminID, emby.GetUsersByUserIdItemsOptions{Recursive: new(true), IncludeItemTypes: kind, Limit: new(0)})).Answer
			if want := count(kind); len(plain.Items) != 0 || plain.TotalRecordCount != want || len(users.Items) != 0 || users.TotalRecordCount != want {
				t.Errorf("a limit of none over %s = %d items of %d, and %d of %d for the user; want none of %d from both", kind, len(plain.Items), plain.TotalRecordCount, len(users.Items), users.TotalRecordCount, want)
			}
		}

		inLibrary := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie", Limit: new(0)})).Answer
		if len(inLibrary.Items) != 0 || inLibrary.TotalRecordCount != sdkacc.SDKMovies.FilmCount() {
			t.Errorf("a limit of none over one library's films = %d items of %d, want none of %d", len(inLibrary.Items), inLibrary.TotalRecordCount, sdkacc.SDKMovies.FilmCount())
		}
	})
}

// TestEmbyItem covers the single-item shape, the update round trip, and the
// reads that hang off one item.
//

func TestEmbyItem(t *testing.T) {
	ctx := skipUnlessEmby(t)
	id := embyMovie(t, sdkacc.Aliens).Id
	// the album the instant mix is made from
	musicID := embyLibrary(t, sdkacc.SDKMusic)
	album := embyAlbum(t, musicID, sdkacc.DarkSide)

	full := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, id)).Answer
	if full.Name != sdkacc.Aliens || full.ProductionYear != 1986 || full.Path == "" || full.Overview == "" || len(full.MediaSources) == 0 || full.RunTimeTicks == 0 {
		t.Fatalf("the full item did not decode: %+v", full)
	}
	if full.UserData == nil {
		t.Error("the per-user item has no UserData")
	}
	if len(full.MediaSources[0].MediaStreams) == 0 || full.MediaSources[0].Container == "" {
		t.Errorf("MediaSources[0] = %+v, want streams and a container", full.MediaSources[0])
	}

	original := *full
	// the server keeps the tags a post leaves out, so the restore sends an
	// empty list rather than none to take the test's tag off again
	if original.TagItems == nil {
		original.TagItems = []emby.NameLongIdPair{}
	}
	// Emby reads the named records (GenreItems, and TagItems, which the
	// spec's BaseItemDto does not carry until the emby-item-tag-items
	// workaround adds it) and ignores the plain Genres and Tags lists on an
	// update, so genres are set through GenreItems and tags through TagItems
	full.Overview = "SDK overview"
	full.Genres, full.GenreItems = []string{"SDK Genre", "Action"}, []emby.NameLongIdPair{{Name: "SDK Genre"}, {Name: "Action"}}
	full.Tags, full.TagItems = []string{"sdk"}, []emby.NameLongIdPair{{Name: "sdk"}}
	if _, err := embyc.ItemUpdate.PostItemsByItemId(ctx, id, *full); err != nil {
		t.Fatal(err)
	}
	sdkacc.UndoLater(t, "putting the item back", func() error {
		_, err := embyc.ItemUpdate.PostItemsByItemId(context.WithoutCancel(ctx), id, original)
		return err
	})
	got := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, id)).Answer
	if got.Overview != "SDK overview" || !slices.Equal(got.Genres, []string{"SDK Genre", "Action"}) {
		t.Errorf("after the update: overview %q, genres %v", got.Overview, got.Genres)
	}
	// the tags come back in TagItems, which /Items sends when Fields names
	// it; the plain Tags list stays null
	if len(got.Tags) != 0 {
		t.Errorf("after the update tags = %v; the server now fills the plain Tags list", got.Tags)
	}
	tagged := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{Ids: id, Fields: "Tags,TagItems"})).Answer
	if len(tagged.Items) != 1 || !slices.ContainsFunc(tagged.Items[0].TagItems, func(p emby.NameLongIdPair) bool { return p.Name == "sdk" }) {
		t.Errorf("after the update the item reads %+v, want sdk among its TagItems", tagged.Items)
	}
	if got.Name != sdkacc.Aliens || got.ProductionYear != 1986 || got.ProviderIds["Tmdb"] != "679" {
		t.Errorf("the update disturbed the fields it should not have: %+v", got)
	}

	// a full refresh with ReplaceAllMetadata puts the provider's overview back
	if _, err := embyc.ItemRefresh.PostItemsByIdRefresh(ctx, id, emby.BaseRefreshRequest{}, emby.PostItemsByIdRefreshOptions{
		MetadataRefreshMode: emby.MetadataRefreshModeFullRefresh, ReplaceAllMetadata: new(true),
	}); err != nil {
		t.Fatal(err)
	}
	if !sdkacc.Poll(2*time.Minute, func() bool {
		it, err := embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, id)
		return err == nil && it.Answer.Overview != "SDK overview" && it.Answer.Overview != ""
	}) {
		t.Error("the refresh never replaced the overview")
	}

	t.Run("Similar", func(t *testing.T) {
		// with the genres and studios TMDB filled in, the server finds the
		// other films like it
		res := sdkacc.Must(embyc.Library.GetItemsByIdSimilar(ctx, id, emby.GetItemsByIdSimilarOptions{UserId: adminID, Limit: new(3)})).Answer
		if len(res.Items) == 0 {
			t.Fatalf("nothing is similar to %s among seven other films", sdkacc.Aliens)
		}
		for _, it := range res.Items {
			if it.Id == "" || it.Name == "" || it.Id == id || it.Type != "Movie" {
				t.Errorf("similar item = %+v", it)
			}
		}
	})
	t.Run("InstantMix", func(t *testing.T) {
		// a mix is music: from a film there is none
		if res := sdkacc.Must(embyc.InstantMix.GetItemsByIdInstantMix(ctx, id, emby.GetItemsByIdInstantMixOptions{UserId: adminID, Limit: new(5)})).Answer; len(res.Items) != 0 {
			t.Errorf("an instant mix from a film = %+v, want nothing", res.Items)
		}
		// from an album it is the songs of the album's genre, the other
		// album by the same artist's included
		mix := sdkacc.Must(embyc.InstantMix.GetItemsByIdInstantMix(ctx, album.Id, emby.GetItemsByIdInstantMixOptions{UserId: adminID, Limit: new(50)})).Answer
		genre := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: musicID, Recursive: new(true), IncludeItemTypes: "Audio", Genres: sdkacc.ProgressiveRock})).Answer
		if len(genre.Items) == 0 {
			t.Fatalf("no song in %s", sdkacc.ProgressiveRock)
		}
		want := map[string]bool{}
		for _, s := range genre.Items {
			want[s.Id] = true
		}
		got := map[string]bool{}
		for _, it := range mix.Items {
			if it.Type != "Audio" || !want[it.Id] {
				t.Errorf("the mix from %s holds %s (%s), which is not a %s song", sdkacc.DarkSide, it.Name, it.Type, sdkacc.ProgressiveRock)
			}
			got[it.Id] = true
		}
		if len(got) != len(want) {
			t.Errorf("the mix from %s holds %d of the %d %s songs", sdkacc.DarkSide, len(got), len(want), sdkacc.ProgressiveRock)
		}
	})
	t.Run("Images", func(t *testing.T) {
		images := sdkacc.Must(embyc.Image.GetItemsByIdImages(ctx, id)).Answer
		i := slices.IndexFunc(images, func(img emby.ImageInfo) bool { return img.ImageType == "Primary" })
		if i < 0 {
			t.Fatalf("no Primary image among %+v; the fixture has a poster.jpg", images)
		}
		// (Size is reported as 0)
		if images[i].Path == "" || images[i].Width == 0 || images[i].Height == 0 {
			t.Errorf("Primary image = %+v", images[i])
		}
	})
	t.Run("ExternalIds", func(t *testing.T) {
		infos := sdkacc.Must(embyc.ItemLookup.GetItemsByIdExternalIdInfos(ctx, id)).Answer
		if !slices.ContainsFunc(infos, func(e emby.ExternalIdInfo) bool { return e.Key == "Tmdb" && e.Name != "" }) {
			t.Errorf("external id infos = %+v, want a Tmdb entry", infos)
		}
	})
	t.Run("Ancestors", func(t *testing.T) {
		anc := sdkacc.Must(embyc.Library.GetItemsByIdAncestors(ctx, id, adminID)).Answer
		if !slices.ContainsFunc(anc, func(a emby.BaseItemDto) bool { return a.Name == sdkacc.SDKMovies.Name }) {
			t.Errorf("ancestors = %+v, want %s among them", anc, sdkacc.SDKMovies.Name)
		}
	})
	t.Run("PlaybackInfo", func(t *testing.T) {
		info := sdkacc.Must(embyc.MediaInfo.GetItemsByIdPlaybackInfo(ctx, id, adminID)).Answer
		if len(info.MediaSources) == 0 || info.PlaySessionId == "" {
			t.Errorf("playback info = %+v", info)
		}
	})
	t.Run("Extras", func(t *testing.T) {
		if _, err := embyc.Library.GetItemsByIdCriticReviews(ctx, id, emby.GetItemsByIdCriticReviewsOptions{}); err != nil {
			t.Error(err)
		}
		if _, err := embyc.Library.GetItemsByIdThemeMedia(ctx, id, emby.GetItemsByIdThemeMediaOptions{UserId: adminID}); err != nil {
			t.Error(err)
		}
		if intros := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsByIdIntros(ctx, adminID, id, emby.GetUsersByUserIdItemsByIdIntrosOptions{})).Answer; len(intros.Items) != 0 {
			t.Errorf("intros = %+v", intros)
		}
		if parts := sdkacc.Must(embyc.UserLibrary.GetVideosByIdAdditionalParts(ctx, id, emby.GetVideosByIdAdditionalPartsOptions{UserId: adminID})).Answer; len(parts.Items) != 0 {
			t.Errorf("additional parts = %+v", parts)
		}
		// the delete preview wants a user behind the request: an API key
		// gets a 400, "Value cannot be null (Parameter 'user')"
		if _, err := embyc.Library.GetItemsByIdDeleteInfo(ctx, id); client.StatusCode(err) != 400 {
			t.Errorf("GetItemsByIdDeleteInfo with an API key = %v, want a 400", err)
		}
	})
}

// TestEmbyCatalogue covers the by-name folders and the browse endpoints.
//

func TestEmbyCatalogue(t *testing.T) {
	ctx := skipUnlessEmby(t)
	moviesID := embyLibrary(t, sdkacc.SDKMovies)

	genres := sdkacc.Must(embyc.Genres.GetGenres(ctx, emby.GetGenresOptions{ParentId: moviesID, Recursive: new(true), UserId: adminID})).Answer
	if !slices.ContainsFunc(genres.Items, func(g emby.BaseItemDto) bool { return g.Name == "Horror" && g.Id != "" && g.Type == "Genre" }) {
		t.Errorf("GetGenres = %+v, want Horror", genres.Items)
	}
	studios := sdkacc.Must(embyc.Studios.GetStudios(ctx, emby.GetStudiosOptions{ParentId: moviesID, Recursive: new(true), UserId: adminID})).Answer
	if len(studios.Items) == 0 || studios.Items[0].Name == "" || studios.Items[0].Id == "" {
		t.Errorf("GetStudios = %+v, want the studios TMDB fills in", studios.Items)
	}
	years := sdkacc.Must(embyc.Tag.GetYears(ctx, emby.GetYearsOptions{ParentId: moviesID, Recursive: new(true), UserId: adminID})).Answer
	if !slices.ContainsFunc(years.Items, func(y emby.UserLibraryTagItem) bool { return y.Name == "1979" }) {
		t.Errorf("GetYears = %+v, want 1979", years.Items)
	}
	tags := sdkacc.Must(embyc.Tag.GetTags(ctx, emby.GetTagsOptions{ParentId: moviesID, Recursive: new(true), UserId: adminID})).Answer
	if tags.TotalRecordCount != 0 {
		t.Errorf("GetTags = %+v on a library with no tags", tags.Items)
	}
	prefixes := sdkacc.Must(embyc.Tag.GetItemsPrefixes(ctx, emby.GetItemsPrefixesOptions{ParentId: moviesID, Recursive: new(true), IncludeItemTypes: "Movie", UserId: adminID})).Answer
	if !slices.ContainsFunc(prefixes, func(p emby.NameValuePair) bool { return p.Name == "A" }) {
		t.Errorf("GetItemsPrefixes = %+v, want A for Alien", prefixes)
	}
	latest := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsLatest(ctx, adminID, emby.GetUsersByUserIdItemsLatestOptions{ParentId: moviesID, Limit: new(3)})).Answer
	if len(latest) != 3 || latest[0].Id == "" {
		t.Errorf("latest = %d items", len(latest))
	}

	// /Trailers answers with the user's libraries rather than any trailer
	// (there is none): the movie library is among them
	trailers := sdkacc.Must(embyc.Trailers.GetTrailers(ctx, emby.GetTrailersOptions{UserId: adminID})).Answer.Items
	if !slices.ContainsFunc(trailers, func(it emby.BaseItemDto) bool { return it.Id == moviesID }) ||
		slices.ContainsFunc(trailers, func(it emby.BaseItemDto) bool { return it.Type == "Trailer" }) {
		t.Errorf("GetTrailers = %+v, want the user's libraries and no trailer", trailers)
	}
	// no channel plugin is installed
	if res := sdkacc.Must(embyc.Channel.GetChannels(ctx, emby.GetChannelsOptions{UserId: adminID})).Answer; res.TotalRecordCount != 0 || len(res.Items) != 0 {
		t.Errorf("GetChannels = %+v", res)
	}

	// the music families, read by library: none in the film library, and the
	// fixtures' in the music one (whatever else the server holds, so the
	// counts do not depend on what ran before)
	for _, lib := range []string{moviesID, embyLibrary(t, sdkacc.SDKMusic)} {
		wantArtists, wantAlbumArtists, wantGenres := []string{}, []string{}, []string{}
		if lib != moviesID {
			// the tracks' artists, The Pink Floyd among them (one track's
			// tag spells it so), and the albums': Various Artists over Battle
			// Tapes' Polygon, which Emby lists as an artist too
			wantArtists = []string{"Battle Tapes", "Coyote Kisses", "Pink Floyd", "SirensCeol", "The Pink Floyd", "Various Artists"}
			wantAlbumArtists = []string{"Coyote Kisses", "Pink Floyd", "SirensCeol", "Various Artists"}
			wantGenres = []string{"Electronic", "Electronica", sdkacc.ProgressiveRock}
		}
		names := func(items []emby.BaseItemDto) []string {
			out := make([]string, 0, len(items))
			for i := range items {
				out = append(out, items[i].Name)
			}
			slices.Sort(out)
			return out
		}
		artists := sdkacc.Must(embyc.Artists.GetArtists(ctx, emby.GetArtistsOptions{ParentId: lib, UserId: adminID})).Answer
		if got := names(artists.Items); !slices.Equal(got, wantArtists) || artists.TotalRecordCount != len(wantArtists) {
			t.Errorf("GetArtists(%s) = %v (%d), want %v", lib, got, artists.TotalRecordCount, wantArtists)
		}
		albumArtists := sdkacc.Must(embyc.Artists.GetArtistsAlbumArtists(ctx, emby.GetArtistsAlbumArtistsOptions{ParentId: lib, UserId: adminID})).Answer
		if got := names(albumArtists.Items); !slices.Equal(got, wantAlbumArtists) || albumArtists.TotalRecordCount != len(wantAlbumArtists) {
			t.Errorf("GetArtistsAlbumArtists(%s) = %v (%d), want %v", lib, got, albumArtists.TotalRecordCount, wantAlbumArtists)
		}
		genres := sdkacc.Must(embyc.MusicGenres.GetMusicGenres(ctx, emby.GetMusicGenresOptions{ParentId: lib, UserId: adminID})).Answer
		if got := names(genres.Items); !slices.Equal(got, wantGenres) || genres.TotalRecordCount != len(wantGenres) {
			t.Errorf("GetMusicGenres(%s) = %v (%d), want %v", lib, got, genres.TotalRecordCount, wantGenres)
		}
	}
}
