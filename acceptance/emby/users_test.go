//go:build acceptance

package emby_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
	"github.com/katbyte/go-kt/pointer"
	"github.com/katbyte/pandorest/client"
)

func TestEmbyUsers(t *testing.T) {
	ctx := skipUnlessEmby(t)

	users := sdkacc.Must(embyc.User.GetUsersQuery(ctx, emby.GetUsersQueryOptions{})).Answer.Items
	if len(users) != 2 {
		t.Fatalf("GetUsersQuery = %d, want root and alice", len(users))
	}
	for _, u := range users {
		if u.Id == "" || u.Name == "" || u.ServerId == "" || u.Policy == nil || u.Configuration == nil {
			t.Errorf("user did not decode: %+v", u)
		}
	}
	root := users[slices.IndexFunc(users, func(u emby.UserDto) bool { return u.Id == adminID })]
	if root.Name != "root" || !pointer.From(root.Policy.IsAdministrator) || !pointer.From(root.HasPassword) {
		t.Errorf("root = %+v", root)
	}

	alice := sdkacc.Must(embyc.User.GetUsersById(ctx, aliceID)).Answer
	if alice.Name != "alice" || alice.Policy == nil || pointer.From(alice.Policy.IsAdministrator) {
		t.Errorf("GetUsersById(alice) = %+v", alice)
	}
	if public := sdkacc.Must(embyc.User.GetUsersPublic(ctx)).Answer; len(public) == 0 {
		t.Error("GetUsersPublic listed nobody")
	}

	auth := sdkacc.Must(embyc.User.PostUsersAuthenticateByName(ctx, emby.AuthenticateUserByName{Username: "root", Pw: password}, "")).Answer
	if auth.AccessToken == "" || auth.ServerId == "" || auth.User == nil || auth.User.Id != adminID || auth.SessionInfo == nil || auth.SessionInfo.Id == "" {
		t.Errorf("PostUsersAuthenticateByName = %+v", auth)
	}
	if _, err := embyc.User.PostUsersAuthenticateByName(ctx, emby.AuthenticateUserByName{Username: "root", Pw: "wrong"}, ""); client.StatusCode(err) != 401 {
		t.Errorf("a wrong password = %v, want a 401", err)
	}
}

// embyKeys decodes the API key list. The spec declares no response schema
// for GET /Auth/Keys, so the SDK hands back the raw JSON.
type embyKeys struct {
	Items            []embyKey `json:"Items"`
	TotalRecordCount int       `json:"TotalRecordCount"`
}

type embyKey struct {
	AccessToken string `json:"AccessToken"`
	AppName     string `json:"AppName"`
	DateCreated string `json:"DateCreated"`
}

func TestEmbyAPIKeys(t *testing.T) {
	ctx := skipUnlessEmby(t)

	if _, err := embyc.Sessions.PostAuthKeys(ctx, sdkacc.SDKApp); err != nil {
		t.Fatal(err)
	}
	var keys embyKeys
	if err := json.Unmarshal(sdkacc.Must(embyc.Sessions.GetAuthKeys(ctx, emby.GetAuthKeysOptions{})).Answer, &keys); err != nil {
		t.Fatalf("GetAuthKeys did not decode: %v", err)
	}
	i := slices.IndexFunc(keys.Items, func(k embyKey) bool { return k.AppName == sdkacc.SDKApp })
	if i < 0 {
		t.Fatalf("GetAuthKeys does not list %s: %+v", sdkacc.SDKApp, keys.Items)
	}
	key := keys.Items[i]
	// (TotalRecordCount is 0 whatever the list holds)
	if key.AccessToken == "" || key.DateCreated == "" {
		t.Errorf("key = %+v", key)
	}

	if _, err := embyc.Sessions.DeleteAuthKeysByKey(ctx, key.AccessToken); err != nil {
		t.Fatal(err)
	}
	var after embyKeys
	if err := json.Unmarshal(sdkacc.Must(embyc.Sessions.GetAuthKeys(ctx, emby.GetAuthKeysOptions{})).Answer, &after); err != nil {
		t.Fatal(err)
	}
	for _, k := range after.Items {
		if k.AccessToken == key.AccessToken {
			t.Error("the key is still listed after DeleteAuthKeysByKey")
		}
	}
}

func TestEmbyPlayedAndResume(t *testing.T) {
	ctx := skipUnlessEmby(t)
	id := embyMovie(t, "Princess Mononoke").Id

	ud := sdkacc.Must(embyc.Playstate.PostUsersByUserIdPlayedItemsById(ctx, adminID, id)).Answer
	if !pointer.From(ud.Played) || ud.PlayCount == 0 || ud.LastPlayedDate == "" {
		t.Errorf("marking played = %+v", ud)
	}
	if it := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, id)).Answer; it.UserData == nil || !pointer.From(it.UserData.Played) {
		t.Errorf("after marking played the item reads %+v", it.UserData)
	}
	if res := sdkacc.Must(embyc.Items.GetUsersByUserIdItems(ctx, adminID, emby.GetUsersByUserIdItemsOptions{ParentId: embyLibrary(t, sdkacc.SDKMovies), Recursive: new(true), IsPlayed: new(true)})).Answer; len(res.Items) != 1 || res.Items[0].Id != id {
		t.Errorf("IsPlayed=true = %+v, want Princess Mononoke", res.Items)
	}
	// alice never watched it
	if it := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, aliceID, id)).Answer; it.UserData != nil && pointer.From(it.UserData.Played) {
		t.Error("marking played for root marked it for alice too")
	}
	if ud := sdkacc.Must(embyc.Playstate.DeleteUsersByUserIdPlayedItemsById(ctx, adminID, id)).Answer; pointer.From(ud.Played) {
		t.Errorf("marking unplayed = %+v", ud)
	}

	// nothing is in progress yet, so the list is empty but must decode
	resume := func() *emby.QueryResultBaseItemDto {
		return sdkacc.Must(embyc.Items.GetUsersByUserIdItemsResume(ctx, adminID, emby.GetUsersByUserIdItemsResumeOptions{Recursive: new(true), MediaTypes: "Video", Limit: new(5)})).Answer
	}
	if res := resume(); len(res.Items) != 0 {
		t.Errorf("resume items = %+v", res)
	}

	// the user data update takes the position and the played flag from the
	// post, a missing one as zero and false, and leaves the favourite alone
	if _, err := embyc.Playstate.PostUsersByUserIdPlayedItemsById(ctx, adminID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := embyc.UserLibrary.PostUsersByUserIdFavoriteItemsById(ctx, adminID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := embyc.Playstate.PostUsersByUserIdItemsByItemIdUserData(ctx, adminID, id, emby.UserItemDataDto{PlaybackPositionTicks: 1_200_000_000}); err != nil {
		t.Fatal(err)
	}
	if ud := sdkacc.Must(embyc.UserLibrary.GetUsersByUserIdItemsById(ctx, adminID, id)).Answer.UserData; ud == nil || ud.PlaybackPositionTicks != 1_200_000_000 || pointer.From(ud.Played) || !pointer.From(ud.IsFavorite) {
		t.Errorf("after posting only a position the user data reads %+v, want the position, not played, still a favourite", ud)
	}
	// a position two minutes in is kept, and puts it in progress
	if res := resume(); len(res.Items) != 1 || res.Items[0].Id != id || res.Items[0].UserData == nil || res.Items[0].UserData.PlaybackPositionTicks != 1_200_000_000 {
		t.Errorf("resume items after setting a position = %+v", res.Items)
	}
	// marking unplayed clears the position
	if _, err := embyc.Playstate.DeleteUsersByUserIdPlayedItemsById(ctx, adminID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := embyc.UserLibrary.DeleteUsersByUserIdFavoriteItemsById(ctx, adminID, id); err != nil {
		t.Fatal(err)
	}
	if res := resume(); len(res.Items) != 0 {
		t.Errorf("resume items after marking unplayed = %+v", res.Items)
	}
}
