//go:build acceptance

package jf_test

import (
	"context"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
	"github.com/katbyte/go-kt/pointer"
	"github.com/katbyte/pandorest/client"
)

func TestJFUsers(t *testing.T) {
	ctx := skipUnlessJellyfin(t)

	users := sdkacc.Must(jfc.User.GetUsers(ctx, jf.GetUsersOptions{})).Answer
	if len(users) != 2 {
		t.Fatalf("GetUsers = %d, want root and alice", len(users))
	}
	for _, u := range users {
		if u.Id == "" || u.Name == "" || u.ServerId == "" || u.Policy == nil || u.Configuration == nil {
			t.Errorf("user did not decode: %+v", u)
		}
	}
	root := users[slices.IndexFunc(users, func(u jf.UserDto) bool { return u.Id == adminID })]
	if root.Name != "root" || !pointer.From(root.Policy.IsAdministrator) {
		t.Errorf("root = %+v", root)
	}

	alice := sdkacc.Must(jfc.User.GetUserById(ctx, aliceID)).Answer
	if alice.Name != "alice" || alice.Policy == nil || pointer.From(alice.Policy.IsAdministrator) {
		t.Errorf("GetUserById(alice) = %+v", alice)
	}
	// the public list is the users shown on the login screen, which new
	// users are hidden from: empty until alice is shown, and empty again once
	// she is hidden
	if public := sdkacc.Must(jfc.User.GetPublicUsers(ctx)).Answer; len(public) != 0 {
		t.Errorf("GetPublicUsers = %+v, with every user hidden", public)
	}
	hidden := func(ctx context.Context, to bool) {
		t.Helper()
		policy := *alice.Policy
		policy.IsHidden = new(to)
		if _, err := jfc.User.UpdateUserPolicy(ctx, aliceID, policy); err != nil {
			t.Fatal(err)
		}
	}
	hidden(ctx, false)
	t.Cleanup(func() { hidden(context.WithoutCancel(ctx), true) })
	public := sdkacc.Must(jfc.User.GetPublicUsers(ctx)).Answer
	if len(public) != 1 || public[0].Id != aliceID || public[0].Name != "alice" || !pointer.From(public[0].HasPassword) {
		t.Errorf("GetPublicUsers with alice shown = %+v, want alice", public)
	}
	hidden(ctx, true)
	if public := sdkacc.Must(jfc.User.GetPublicUsers(ctx)).Answer; len(public) != 0 {
		t.Errorf("GetPublicUsers with alice hidden again = %+v", public)
	}
	// an API key has no user behind it
	if _, err := jfc.User.GetCurrentUser(ctx); client.StatusCode(err) != 400 {
		t.Errorf("GetCurrentUser with an API key = %v, want a 400", err)
	}

	auth := sdkacc.Must(jfc.Authentication.AuthenticateUserByName(ctx, jf.AuthenticateUserByName{Username: "root", Pw: password})).Answer
	if auth.AccessToken == "" || auth.ServerId == "" || auth.User == nil || auth.User.Id != adminID || auth.SessionInfo == nil || auth.SessionInfo.Id == "" {
		t.Errorf("AuthenticateUserByName = %+v", auth)
	}
	if _, err := jfc.Authentication.AuthenticateUserByName(ctx, jf.AuthenticateUserByName{Username: "root", Pw: "wrong"}); client.StatusCode(err) != 401 {
		t.Errorf("a wrong password = %v, want a 401", err)
	}
}

func TestJFAPIKeys(t *testing.T) {
	ctx := skipUnlessJellyfin(t)

	if _, err := jfc.Authentication.CreateKey(ctx, sdkacc.SDKApp); err != nil {
		t.Fatal(err)
	}
	keys := sdkacc.Must(jfc.Authentication.GetKeys(ctx)).Answer
	i := slices.IndexFunc(keys.Items, func(k jf.AuthenticationInfo) bool { return k.AppName == sdkacc.SDKApp })
	if i < 0 {
		t.Fatalf("GetKeys does not list %s: %+v", sdkacc.SDKApp, keys.Items)
	}
	key := keys.Items[i]
	// (the server lists keys with Id 0 and IsActive false; the token and
	// dates are what it fills in)
	if key.AccessToken == "" || key.DateCreated.IsZero() {
		t.Errorf("key = %+v", key)
	}
	// the testenv key is there too
	if !slices.ContainsFunc(keys.Items, func(k jf.AuthenticationInfo) bool { return k.AppName == "embyfin-mcp-test" }) {
		t.Errorf("GetKeys does not list the testenv key: %+v", keys.Items)
	}

	if _, err := jfc.Authentication.RevokeKey(ctx, key.AccessToken); err != nil {
		t.Fatal(err)
	}
	if after := sdkacc.Must(jfc.Authentication.GetKeys(ctx)).Answer; slices.ContainsFunc(after.Items, func(k jf.AuthenticationInfo) bool { return k.AccessToken == key.AccessToken }) {
		t.Error("the key is still listed after RevokeKey")
	}
}

func TestJFPlayedAndResume(t *testing.T) {
	ctx := skipUnlessJellyfin(t)
	id := jfMovie(t, "Princess Mononoke").Id

	ud := sdkacc.Must(jfc.UserData.MarkPlayedItem(ctx, id, jf.MarkPlayedItemOptions{UserId: adminID})).Answer
	if !pointer.From(ud.Played) || ud.PlayCount == 0 || ud.LastPlayedDate.IsZero() {
		t.Errorf("MarkPlayedItem = %+v", ud)
	}
	if it := sdkacc.Must(jfc.Library.GetItem(ctx, id, adminID)).Answer; it.UserData == nil || !pointer.From(it.UserData.Played) {
		t.Errorf("after MarkPlayedItem the item reads %+v", it.UserData)
	}
	if res := sdkacc.Must(jfc.Library.GetItems(ctx, jf.GetItemsOptions{UserId: adminID, ParentId: jfLibrary(t, sdkacc.SDKMovies), Recursive: new(true), IsPlayed: new(true)})).Answer; len(res.Items) != 1 || res.Items[0].Id != id {
		t.Errorf("IsPlayed=true = %+v, want Princess Mononoke", res.Items)
	}
	// alice never watched it
	if it := sdkacc.Must(jfc.Library.GetItem(ctx, id, aliceID)).Answer; it.UserData != nil && pointer.From(it.UserData.Played) {
		t.Error("marking played for root marked it for alice too")
	}
	if unplayed := sdkacc.Must(jfc.UserData.MarkUnplayedItem(ctx, id, adminID)).Answer; pointer.From(unplayed.Played) {
		t.Errorf("MarkUnplayedItem = %+v", unplayed)
	}

	// nothing is in progress yet, so the list is empty but must decode
	resume := func() *jf.BaseItemDtoQueryResult {
		return sdkacc.Must(jfc.Library.GetResumeItems(ctx, jf.GetResumeItemsOptions{UserId: adminID, MediaTypes: []jf.MediaType{jf.MediaTypeVideo}, Limit: new(5)})).Answer
	}
	if res := resume(); res.TotalRecordCount != 0 {
		t.Errorf("GetResumeItems = %+v", res)
	}

	// the user data update applies only what is set: a favourite survives a
	// position posted on its own
	if _, err := jfc.UserData.MarkFavoriteItem(ctx, id, adminID); err != nil {
		t.Fatal(err)
	}
	ud = sdkacc.Must(jfc.UserData.UpdateItemUserData(ctx, id, jf.UpdateUserItemDataDto{PlaybackPositionTicks: 600_000_000}, adminID)).Answer
	if ud.PlaybackPositionTicks != 600_000_000 || !pointer.From(ud.IsFavorite) {
		t.Errorf("UpdateItemUserData = %+v, want the position and the favourite kept", ud)
	}
	// a position a minute in is kept, and puts it in progress
	if res := resume(); len(res.Items) != 1 || res.Items[0].Id != id {
		t.Errorf("GetResumeItems after setting a position = %+v", res.Items)
	}
	// marking unplayed clears the position
	if ud := sdkacc.Must(jfc.UserData.MarkUnplayedItem(ctx, id, adminID)).Answer; ud.PlaybackPositionTicks != 0 {
		t.Errorf("MarkUnplayedItem left the position: %+v", ud)
	}
	if _, err := jfc.UserData.UnmarkFavoriteItem(ctx, id, adminID); err != nil {
		t.Fatal(err)
	}
	if res := resume(); len(res.Items) != 0 {
		t.Errorf("GetResumeItems after marking unplayed = %+v", res.Items)
	}
}
