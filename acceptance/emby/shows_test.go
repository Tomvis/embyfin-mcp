//go:build acceptance

package emby_test

import (
	"context"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
)

func TestEmbyShows(t *testing.T) {
	ctx := skipUnlessEmby(t)
	showsID := embyLibrary(t, sdkacc.SDKShows)

	series := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{
		ParentId: showsID, Recursive: new(true), IncludeItemTypes: "Series",
		Fields: "ProviderIds,Overview,ProductionYear", SortBy: "SortName",
	})).Answer
	if len(series.Items) != len(sdkacc.Shows) {
		t.Fatalf("series = %d, want %d", len(series.Items), len(sdkacc.Shows))
	}
	for _, s := range sdkacc.Shows {
		i := slices.IndexFunc(series.Items, func(it emby.BaseItemDto) bool { return it.Name == s.Title })
		if i < 0 {
			t.Errorf("%s is missing", s.Title)
			continue
		}
		it := series.Items[i]
		if it.Type != "Series" || it.ProductionYear != s.Year || it.ProviderIds["Tmdb"] != s.TMDB || it.ProviderIds["Tvdb"] != s.TVDB || it.Overview == "" {
			t.Errorf("%s = %+v; the nfo says %d/%s/%s", s.Title, it, s.Year, s.TMDB, s.TVDB)
		}
	}

	sev := embySeries(t, sdkacc.Severance)
	seasons := sdkacc.Must(embyc.TvShows.GetShowsByIdSeasons(ctx, sev.Id, emby.GetShowsByIdSeasonsOptions{UserId: adminID})).Answer
	if len(seasons.Items) != 2 {
		t.Fatalf("%s has %d seasons, want 2", sdkacc.Severance, len(seasons.Items))
	}
	for _, s := range seasons.Items {
		if s.Id == "" || s.Type != "Season" || sdkacc.Number(s.IndexNumber) <= 0 || s.SeriesId != sev.Id || s.SeriesName != sdkacc.Severance {
			t.Errorf("season = %+v", s)
		}
	}

	episodes := sdkacc.Must(embyc.TvShows.GetShowsByIdEpisodes(ctx, sev.Id, emby.GetShowsByIdEpisodesOptions{UserId: adminID, Fields: "Overview,Path"})).Answer
	if episodes.TotalRecordCount != 4 {
		t.Fatalf("%s has %d episodes, want 4", sdkacc.Severance, episodes.TotalRecordCount)
	}
	for _, e := range episodes.Items {
		if e.Id == "" || e.Type != "Episode" || sdkacc.Number(e.IndexNumber) <= 0 || sdkacc.Number(e.ParentIndexNumber) <= 0 || e.SeasonId == "" || e.SeriesId != sev.Id || e.Overview == "" || e.Path == "" {
			t.Errorf("episode = %+v", e)
		}
	}
	if e := episodes.Items[0]; e.Name != "Good News About Hell" || sdkacc.Number(e.ParentIndexNumber) != 1 || sdkacc.Number(e.IndexNumber) != 1 {
		t.Errorf("first episode = %s S%02dE%02d", e.Name, sdkacc.Number(e.ParentIndexNumber), sdkacc.Number(e.IndexNumber))
	}

	one := sdkacc.Must(embyc.TvShows.GetShowsByIdEpisodes(ctx, sev.Id, emby.GetShowsByIdEpisodesOptions{UserId: adminID, Season: new(2)})).Answer
	if len(one.Items) != 2 {
		t.Errorf("season 2 has %d episodes, want 2", len(one.Items))
	}
	bySeason := sdkacc.Must(embyc.TvShows.GetShowsByIdEpisodes(ctx, sev.Id, emby.GetShowsByIdEpisodesOptions{UserId: adminID, SeasonId: seasons.Items[0].Id})).Answer
	if len(bySeason.Items) != 2 {
		t.Errorf("SeasonId=%s has %d episodes, want 2", seasons.Items[0].Id, len(bySeason.Items))
	}
	// with the fetchers off nothing is known to be missing: the four on
	// disk are all the series holds, none of them virtual (Emby 4.10 has no
	// IsMissing filter on its item queries)
	all := sdkacc.Must(embyc.Items.GetItems(ctx, emby.GetItemsOptions{ParentId: sev.Id, Recursive: new(true), IncludeItemTypes: "Episode"})).Answer
	if all.TotalRecordCount != 4 || slices.ContainsFunc(all.Items, func(e emby.BaseItemDto) bool { return e.LocationType == "Virtual" }) {
		t.Errorf("%d episodes with the providers off, want the 4 on disk", all.TotalRecordCount)
	}

	t.Run("NextUp", func(t *testing.T) {
		first := episodes.Items[0].Id
		if _, err := embyc.Playstate.PostUsersByUserIdPlayedItemsById(ctx, adminID, first); err != nil {
			t.Fatal(err)
		}
		sdkacc.UndoLater(t, "unmarking "+string(first)+" played", func() error {
			_, err := embyc.Playstate.DeleteUsersByUserIdPlayedItemsById(context.WithoutCancel(ctx), adminID, first)
			return err
		})

		// Emby 4.10's default next-up mode answers nothing for an episode
		// marked played through the API; the legacy per-series mode is
		// what lists it, so the default answer is asserted empty and the
		// legacy one checked (LegacyNextUp, which the spec does not declare,
		// is added to the generated options by the emby-next-up-legacy
		// workaround)
		if next := sdkacc.Must(embyc.TvShows.GetShowsNextUp(ctx, emby.GetShowsNextUpOptions{UserId: adminID, ParentId: showsID, Fields: "Overview"})).Answer; len(next.Items) != 0 {
			t.Errorf("GetShowsNextUp = %+v, want nothing from the default mode", next.Items)
		}
		res, err := embyc.TvShows.GetShowsNextUp(ctx, emby.GetShowsNextUpOptions{UserId: adminID, ParentId: showsID, LegacyNextUp: new(true)})
		if err != nil {
			t.Fatal(err)
		}
		next := res.Answer
		if len(next.Items) != 1 {
			t.Fatalf("legacy next up = %+v, want the second episode of %s", next.Items, sdkacc.Severance)
		}
		if e := next.Items[0]; e.Name != "Half Loop" || sdkacc.Number(e.IndexNumber) != 2 || e.SeriesName != sdkacc.Severance {
			t.Errorf("next up = %+v", e)
		}
	})
}
