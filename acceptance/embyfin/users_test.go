//go:build acceptance

package embyfin_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// accounts is the ids of every account the server has.
func accounts(t *testing.T) []string {
	t.Helper()

	users, err := client.Users(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(users))
	for _, u := range users {
		out = append(out, u.ID)
	}

	return out
}

// The accounts are listed, and one is found by its name whatever case it is written in, or by its id.
func TestAccountsAreListedAndFound(t *testing.T) {
	ctx := server(t)

	if got := accounts(t); !slices.Contains(got, adminID) || !slices.Contains(got, aliceID) {
		t.Fatalf("the accounts = %v, want the administrator %s and the ordinary account %s among them", got, adminID, aliceID)
	}

	alice, err := client.ResolveUser(ctx, aliceID)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{alice.Name, "ALICE"} {
		if got, err := client.ResolveUser(ctx, name); err != nil || got.ID != aliceID {
			t.Errorf("the account named %q = %+v, %v; want %s", name, got, err, aliceID)
		}
	}
	if got, err := client.ResolveUser(ctx, "nobody-by-this-name"); err == nil {
		t.Errorf("an account nobody made was found: %+v", got)
	}
}

// An account is never made without a password, and one made with one is listed, and gone once deleted.
func TestAnAccountMadeAndDeleted(t *testing.T) {
	ctx := server(t)

	if got, _, err := client.CreateUser(ctx, "neutral-nobody", ""); !errors.Is(err, embyfin.ErrNoPassword) {
		t.Errorf("an account with no password = %+v, %v; want it refused", got, err)
	}

	made, left, err := client.CreateUser(ctx, "neutral-guest", sdkacc.Password())
	if err != nil {
		t.Fatal(err)
	}
	deleted := false
	later(t, "deleting the account", func(ctx context.Context) error {
		if deleted {
			return nil
		}

		return client.DeleteUser(ctx, made.ID)
	})
	if left.Unchecked != "" {
		t.Errorf("the new account could not be checked for another's watch state: %s", left.Unchecked)
	}
	if got := accounts(t); !slices.Contains(got, made.ID) {
		t.Errorf("the accounts after one was made = %v, want %s among them", got, made.ID)
	}

	if err := client.DeleteUser(ctx, made.ID); err != nil {
		t.Fatal(err)
	}
	deleted = true
	if got := accounts(t); slices.Contains(got, made.ID) {
		t.Errorf("the accounts after it was deleted = %v", got)
	}
}

// An account told it may see one library sees what is in it and not what is in another, and sees both again when told it may see every one.
// The servers name a library differently in an account's settings, and neither keeps what is hidden out of every read by itself.
func TestWhatAnAccountMaySee(t *testing.T) {
	ctx := server(t)
	alien := film(t, sdkacc.Alien)
	show := series(t, sdkacc.Severance)
	later(t, "letting the account see every library again", func(ctx context.Context) error {
		_, _, err := client.SetUserLibraries(ctx, aliceID, true, nil)

		return err
	})

	sees := func(itemID string) bool {
		t.Helper()

		_, visible, err := client.VisibleUserItem(ctx, aliceID, itemID)
		if err != nil {
			t.Fatal(err)
		}

		return visible
	}

	if _, _, err := client.SetUserLibraries(ctx, aliceID, false, []embyfin.VirtualFolder{*library(t, filmLibrary)}); err != nil {
		t.Fatal(err)
	}
	if !sees(alien.ID) {
		t.Error("the account cannot see a film of the one library it may see")
	}
	if sees(show.ID) {
		t.Error("the account can see a series of a library it may not see")
	}

	if _, _, err := client.SetUserLibraries(ctx, aliceID, true, nil); err != nil {
		t.Fatal(err)
	}
	if !sees(alien.ID) || !sees(show.ID) {
		t.Errorf("the account allowed every library sees the film: %v, the series: %v", sees(alien.ID), sees(show.ID))
	}

	if got, visible, err := client.VisibleUserItem(ctx, aliceID, "999999999"); err == nil {
		t.Errorf("an id no item has was read for the account: %+v, visible %v", got, visible)
	}
}
