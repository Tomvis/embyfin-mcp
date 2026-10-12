//go:build acceptance

package embyfin_test

import (
	"context"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// Watched and favourite are one account's own: marking a film for one shows on that account's reads and lists and on nobody else's, and
// un-marking it takes it off again.
func TestWatchStateIsOneAccountsOwn(t *testing.T) {
	ctx := server(t)
	alien := film(t, sdkacc.Alien)
	later(t, "un-marking the film", func(ctx context.Context) error {
		if err := client.SetFavourite(ctx, adminID, alien.ID, false); err != nil {
			return err
		}

		return client.SetPlayed(ctx, adminID, alien.ID, false)
	})

	state := func(userID string) embyfin.UserData {
		t.Helper()

		it, err := client.UserItem(ctx, userID, alien.ID)
		if err != nil {
			t.Fatal(err)
		}
		if it.UserData == nil {
			t.Fatalf("the film read for %s carries no watch state", userID)
		}

		return *it.UserData
	}
	films := library(t, filmLibrary)
	watched := func(userID string) []string {
		t.Helper()

		items, _, err := client.Search(ctx, embyfin.SearchOptions{ParentID: films.ItemID, IncludeItemTypes: "Movie", UserID: userID, Filters: "IsPlayed"})
		if err != nil {
			t.Fatal(err)
		}

		return names(items)
	}

	if err := client.SetPlayed(ctx, adminID, alien.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := client.SetFavourite(ctx, adminID, alien.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := state(adminID); !got.Played || !got.IsFavourite || got.PlayCount < 1 {
		t.Errorf("the film for the account that watched it = %+v, want played, a favourite and counted", got)
	}
	if got := state(aliceID); got.Played || got.IsFavourite {
		t.Errorf("the film for another account = %+v, want neither played nor a favourite", got)
	}
	if got := watched(adminID); !slices.Equal(got, []string{sdkacc.Alien}) {
		t.Errorf("the films the account has watched = %v, want %s alone", got, sdkacc.Alien)
	}
	if got := watched(aliceID); len(got) != 0 {
		t.Errorf("the films another account has watched = %v, want none", got)
	}

	if err := client.SetPlayed(ctx, adminID, alien.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := state(adminID); got.Played {
		t.Errorf("the film after it was un-marked = %+v", got)
	}
}

// A resume point is kept where it was put, lists the film as part way through, and is cleared by marking the film unwatched.
func TestAResumePoint(t *testing.T) {
	ctx := server(t)
	aliens, err := client.ItemByID(ctx, film(t, sdkacc.Aliens).ID)
	if err != nil {
		t.Fatal(err)
	}
	if aliens.RunTimeTicks == 0 {
		t.Fatalf("%s has no length to stop part way through", aliens.Name)
	}
	later(t, "clearing the resume point", func(ctx context.Context) error { return client.SetPlayed(ctx, adminID, aliens.ID, false) })

	inProgress := func() []string {
		t.Helper()

		items, rerr := client.Resume(ctx, adminID, 0)
		if rerr != nil {
			t.Fatal(rerr)
		}

		return names(items)
	}

	half := aliens.RunTimeTicks / 2
	if err := client.SetProgress(ctx, adminID, aliens.ID, half); err != nil {
		t.Fatal(err)
	}
	it, err := client.UserItem(ctx, adminID, aliens.ID)
	if err != nil || it.UserData == nil {
		t.Fatalf("reading the film back: %+v, %v", it, err)
	}
	if it.UserData.PlaybackPositionTicks != half || it.UserData.Played {
		t.Errorf("the film with a resume point = %+v, want position %d and not played", *it.UserData, half)
	}
	if got := inProgress(); !slices.Contains(got, sdkacc.Aliens) {
		t.Errorf("what the account is part way through = %v, want %s among it", got, sdkacc.Aliens)
	}

	if err := client.SetPlayed(ctx, adminID, aliens.ID, false); err != nil {
		t.Fatal(err)
	}
	if it, err := client.UserItem(ctx, adminID, aliens.ID); err != nil || it.UserData == nil || it.UserData.PlaybackPositionTicks != 0 {
		t.Errorf("the film after it was marked unwatched = %+v, %v; want no resume point", it, err)
	}
	if got := inProgress(); slices.Contains(got, sdkacc.Aliens) {
		t.Errorf("what the account is part way through after that = %v", got)
	}
}
