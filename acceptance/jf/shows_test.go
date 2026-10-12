//go:build acceptance

package jf_test

import (
	"context"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
)

func TestJFShows(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	showsID := jfLibrary(t, sdkacc.SDKShows)

	series := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{
		ParentId: showsID, Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindSeries},
		Fields: []jf.ItemFields{jf.ItemFieldsProviderIds, jf.ItemFieldsOverview}, SortBy: []jf.ItemSortBy{jf.ItemSortByName},
	})).Answer
	if len(series.Items) != len(sdkacc.Shows) {
		t.Fatalf("series = %d, want %d", len(series.Items), len(sdkacc.Shows))
	}
	for _, s := range sdkacc.Shows {
		i := slices.IndexFunc(series.Items, func(it jf.BaseItemDto) bool { return it.Name == s.Title })
		if i < 0 {
			t.Errorf("%s is missing", s.Title)
			continue
		}
		it := series.Items[i]
		if it.Type != jf.BaseItemKindSeries || it.ProductionYear != s.Year || it.ProviderIds["Tmdb"] != s.TMDB || it.ProviderIds["Tvdb"] != s.TVDB || it.Overview == "" {
			t.Errorf("%s = %+v; the nfo says %d/%s/%s", s.Title, it, s.Year, s.TMDB, s.TVDB)
		}
	}

	sev := jfSeries(t, sdkacc.Severance)
	seasons := sdkacc.Must(jfc.Show.GetSeasons(ctx, sev.Id, jf.GetSeasonsOptions{UserId: adminID})).Answer
	if len(seasons.Items) != 2 {
		t.Fatalf("%s has %d seasons, want 2", sdkacc.Severance, len(seasons.Items))
	}
	for _, s := range seasons.Items {
		if s.Id == "" || s.Type != jf.BaseItemKindSeason || sdkacc.Number(s.IndexNumber) <= 0 || s.SeriesId != sev.Id || s.SeriesName != sdkacc.Severance {
			t.Errorf("season = %+v", s)
		}
	}

	episodes := sdkacc.Must(jfc.Show.GetEpisodes(ctx, sev.Id, jf.GetEpisodesOptions{UserId: adminID, Fields: []jf.ItemFields{jf.ItemFieldsOverview, jf.ItemFieldsPath}})).Answer
	if episodes.TotalRecordCount != 4 {
		t.Fatalf("%s has %d episodes, want 4", sdkacc.Severance, episodes.TotalRecordCount)
	}
	for _, e := range episodes.Items {
		if e.Id == "" || e.Type != jf.BaseItemKindEpisode || sdkacc.Number(e.IndexNumber) <= 0 || sdkacc.Number(e.ParentIndexNumber) <= 0 || e.SeasonId == "" || e.SeriesId != sev.Id || e.Overview == "" || e.Path == "" {
			t.Errorf("episode = %+v", e)
		}
	}
	// the first episode by its nfo title
	if e := episodes.Items[0]; e.Name != "Good News About Hell" || sdkacc.Number(e.ParentIndexNumber) != 1 || sdkacc.Number(e.IndexNumber) != 1 {
		t.Errorf("first episode = %s S%02dE%02d", e.Name, sdkacc.Number(e.ParentIndexNumber), sdkacc.Number(e.IndexNumber))
	}

	one := sdkacc.Must(jfc.Show.GetEpisodes(ctx, sev.Id, jf.GetEpisodesOptions{UserId: adminID, Season: new(2)})).Answer
	if len(one.Items) != 2 {
		t.Errorf("season 2 has %d episodes, want 2", len(one.Items))
	}
	bySeason := sdkacc.Must(jfc.Show.GetEpisodes(ctx, sev.Id, jf.GetEpisodesOptions{UserId: adminID, SeasonId: seasons.Items[0].Id})).Answer
	if len(bySeason.Items) != 2 {
		t.Errorf("SeasonId=%s has %d episodes, want 2", seasons.Items[0].Id, len(bySeason.Items))
	}
	// with the fetchers off nothing is known to be missing; the filter
	// still has to be accepted
	if missing := sdkacc.Must(jfc.Show.GetEpisodes(ctx, sev.Id, jf.GetEpisodesOptions{UserId: adminID, IsMissing: new(true)})).Answer; missing.TotalRecordCount != 0 {
		t.Errorf("IsMissing=true = %+v with the providers off", missing.Items)
	}
	if present := sdkacc.Must(jfc.Show.GetEpisodes(ctx, sev.Id, jf.GetEpisodesOptions{UserId: adminID, IsMissing: new(false)})).Answer; present.TotalRecordCount != 4 {
		t.Errorf("IsMissing=false = %d episodes, want 4", present.TotalRecordCount)
	}

	t.Run("NextUp", func(t *testing.T) {
		first := episodes.Items[0].Id
		if _, err := jfc.UserData.MarkPlayedItem(ctx, first, jf.MarkPlayedItemOptions{UserId: adminID}); err != nil {
			t.Fatal(err)
		}
		sdkacc.UndoLater(t, "unmarking "+first+" played", func() error {
			_, err := jfc.UserData.MarkUnplayedItem(context.WithoutCancel(ctx), first, adminID)
			return err
		})

		next := sdkacc.Must(jfc.Show.GetNextUp(ctx, jf.GetNextUpOptions{UserId: adminID, SeriesId: sev.Id, Fields: []jf.ItemFields{jf.ItemFieldsOverview}})).Answer
		if len(next.Items) != 1 {
			t.Fatalf("GetNextUp for %s = %+v, want the second episode", sdkacc.Severance, next.Items)
		}
		if e := next.Items[0]; e.Name != "Half Loop" || sdkacc.Number(e.IndexNumber) != 2 || e.SeriesName != sdkacc.Severance {
			t.Errorf("next up = %+v", e)
		}
		if all := sdkacc.Must(jfc.Show.GetNextUp(ctx, jf.GetNextUpOptions{UserId: adminID, ParentId: showsID})).Answer; len(all.Items) != 1 {
			t.Errorf("GetNextUp across the library = %d, want 1", len(all.Items))
		}
	})

	if up := sdkacc.Must(jfc.Show.GetUpcomingEpisodes(ctx, jf.GetUpcomingEpisodesOptions{UserId: adminID})).Answer; len(up.Items) != 0 {
		t.Errorf("GetUpcomingEpisodes = %+v", up.Items)
	}
	if similar := sdkacc.Must(jfc.Library.GetSimilarShows(ctx, sev.Id, jf.GetSimilarShowsOptions{UserId: adminID, Limit: new(2)})).Answer; len(similar.Items) == 0 {
		t.Error("GetSimilarShows found nothing among the other two shows, one of them a drama too")
	}
}
