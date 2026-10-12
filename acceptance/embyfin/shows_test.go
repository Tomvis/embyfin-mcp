//go:build acceptance

package embyfin_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// onDisk is the episodes that have a file, by season, each season's numbers in order.
func onDisk(t *testing.T, episodes []embyfin.Item) map[int][]int {
	t.Helper()

	out := map[int][]int{}
	for i := range episodes {
		e := &episodes[i]
		if e.Path == "" || e.IsMissing {
			continue
		}
		if e.ParentIndexNumber == nil || e.IndexNumber == nil {
			t.Errorf("the episode %q has no season or no number: %v, %v", e.Name, e.ParentIndexNumber, e.IndexNumber)

			continue
		}
		out[*e.ParentIndexNumber] = append(out[*e.ParentIndexNumber], *e.IndexNumber)
	}
	for season := range out {
		slices.Sort(out[season])
	}

	return out
}

// Each series lists the episodes its folder holds, by season and number, and one season is asked for by its number.
func TestASeriesEpisodes(t *testing.T) {
	ctx := server(t)

	for _, want := range sdkacc.Shows {
		show := series(t, want.Title)

		episodes, err := client.Episodes(ctx, show.ID, embyfin.EpisodeOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got := onDisk(t, episodes); !maps.EqualFunc(got, want.Episodes, slices.Equal) {
			t.Errorf("%s has the episodes %v, want %v", want.Title, got, want.Episodes)
		}

		for season, numbers := range want.Episodes {
			episodes, err := client.Episodes(ctx, show.ID, embyfin.EpisodeOptions{Season: &season})
			if err != nil {
				t.Fatal(err)
			}
			if got := onDisk(t, episodes); !maps.EqualFunc(got, map[int][]int{season: numbers}, slices.Equal) {
				t.Errorf("%s season %d has the episodes %v, want %v", want.Title, season, got, numbers)
			}
		}
	}
}

// Once an account has watched an episode, the one after it is what is next for that account.
func TestWhatIsNextAfterAnEpisode(t *testing.T) {
	ctx := server(t)

	episodes, err := client.Episodes(ctx, series(t, sdkacc.Severance).ID, embyfin.EpisodeOptions{Season: new(1)})
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(episodes, func(a, b embyfin.Item) int { return sdkacc.Number(a.IndexNumber) - sdkacc.Number(b.IndexNumber) })
	if len(episodes) < 2 {
		t.Fatalf("%s season 1 has %d episodes, want two to watch one and be offered the next", sdkacc.Severance, len(episodes))
	}
	first, second := episodes[0], episodes[1]
	later(t, "un-marking the episode", func(ctx context.Context) error { return client.SetPlayed(ctx, adminID, first.ID, false) })

	if err := client.SetPlayed(ctx, adminID, first.ID, true); err != nil {
		t.Fatal(err)
	}
	var next []embyfin.Item
	if !sdkacc.Poll(sdkacc.EditPatience, func() bool {
		if next, err = client.NextUp(ctx, adminID, 0); err != nil {
			t.Fatal(err)
		}

		return slices.Contains(ids(next), second.ID)
	}) {
		t.Errorf("what is next for the account = %v, want %q among it", names(next), second.Name)
	}
}
