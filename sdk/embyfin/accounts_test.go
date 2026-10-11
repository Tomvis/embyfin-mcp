package embyfin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateUser(t *testing.T) {
	t.Parallel()

	// Emby makes the account bare and takes the password in a second call
	c, f := newFake(t, Emby, map[string]route{
		"POST /Users/New":         ok(`{"Id":"u9","Name":"carol"}`),
		"POST /Users/u9/Password": answer(http.StatusNoContent, ""),
		"GET /Users/Query":        ok(`{"Items":[{"Id":"u1","Name":"root"},{"Id":"u9","Name":"carol","HasPassword":true,"Policy":{"EnableAllFolders":true,"EnableRemoteAccess":true}}]}`),
	})
	u, left, err := c.CreateUser(t.Context(), "carol", "s3cret")
	if err != nil || u.ID != "u9" || u.Name != "carol" || !u.Policy.EnableAllFolders || u.Policy.IsAdministrator || left != (Leftover{}) {
		t.Fatalf("CreateUser = %+v, %+v, %v", u, left, err)
	}
	if body := jsonBody(t, f.only("POST /Users/New").body); body["Name"] != "carol" || len(body) != 1 {
		t.Errorf("Emby was sent %v to make the account, want its name alone", body)
	}
	if body := jsonBody(t, f.only("POST /Users/u9/Password").body); body["NewPw"] != "s3cret" {
		t.Errorf("Emby was sent %v for the password", body)
	}

	// Jellyfin takes the password with the name, and is then read for what it lists the new account with: nothing of its own, here
	c, f = newFake(t, Jellyfin, map[string]route{
		"POST /Users/New": ok(`{"Id":"u9","Name":"carol"}`),
		"GET /Users":      ok(`[{"Id":"u9","Name":"carol","Policy":{"EnableAllFolders":true,"IsHidden":true}}]`),
		"GET /Items":      ok(`{"Items":[{"Id":"a","Name":"Alien","Type":"Movie","UserData":{"Played":false,"PlayCount":0,"IsFavorite":false}},{"Id":"s","Name":"Show","Type":"Series","UserData":{"UnplayedItemCount":3,"PlayedPercentage":0}}],"TotalRecordCount":2}`),
	})
	u, left, err = c.CreateUser(t.Context(), "carol", "s3cret")
	if err != nil || u.ID != "u9" || !u.Policy.IsHidden || left != (Leftover{}) {
		t.Fatalf("CreateUser = %+v, %+v, %v", u, left, err)
	}
	if body := jsonBody(t, f.only("POST /Users/New").body); body["Name"] != "carol" || body["Password"] != "s3cret" {
		t.Errorf("Jellyfin was sent %v", body)
	}
	if read := f.only("GET /Items"); read.query.Get("userId") != "u9" || read.query.Get("enableUserData") != "true" || read.query.Get("recursive") != "true" || read.query.Get("includeItemTypes") != "" {
		t.Errorf("the new account's view was read with %v, want everything it sees, as it, with its watch state", read.query)
	}

	// no account is made without a password, on either
	for _, backend := range []Backend{Emby, Jellyfin} {
		c, f = newFake(t, backend, map[string]route{})
		if _, _, err := c.CreateUser(t.Context(), "carol", ""); !errors.Is(err, ErrNoPassword) {
			t.Errorf("%s: CreateUser with no password = %v, want ErrNoPassword", backend, err)
		}
		if len(f.requests) != 0 {
			t.Errorf("%s: %d requests were sent for an account with no password", backend, len(f.requests))
		}
	}

	// a name the server refuses is its refusal, and nothing more is sent
	c, f = newFake(t, Emby, map[string]route{"POST /Users/New": answer(http.StatusBadRequest, "A user with the name 'carol' already exists.")})
	if _, _, err := c.CreateUser(t.Context(), "carol", "s3cret"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("a taken name = %v", err)
	}
	if len(f.requests) != 1 {
		t.Errorf("%d requests after the refusal, want the one", len(f.requests))
	}
}

// An Emby account whose password could not be set is not left behind with none: it is removed again, and the error says which of the two happened.
func TestCreateUserTakesBackAnAccountWithNoPassword(t *testing.T) {
	t.Parallel()

	c, f := newFake(t, Emby, map[string]route{
		"POST /Users/New":         ok(`{"Id":"u9","Name":"carol"}`),
		"POST /Users/u9/Password": answer(http.StatusInternalServerError, "boom"),
		"DELETE /Users/u9":        answer(http.StatusNoContent, ""),
	})
	_, _, err := c.CreateUser(t.Context(), "carol", "s3cret")
	if err == nil || !strings.Contains(err.Error(), "was removed again") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("CreateUser = %v, want the account removed again and why", err)
	}
	f.only("DELETE /Users/u9")

	c, _ = newFake(t, Emby, map[string]route{
		"POST /Users/New":         ok(`{"Id":"u9","Name":"carol"}`),
		"POST /Users/u9/Password": answer(http.StatusInternalServerError, "boom"),
		"DELETE /Users/u9":        answer(http.StatusInternalServerError, "bang"),
	})
	_, _, err = c.CreateUser(t.Context(), "carol", "s3cret")
	if err == nil || !strings.Contains(err.Error(), "WITH NO PASSWORD") || !strings.Contains(err.Error(), "u9") || !strings.Contains(err.Error(), "bang") {
		t.Fatalf("CreateUser = %v, want it said that the account is still there with no password", err)
	}
}

// Jellyfin lists a new account with the watch state of an account deleted before it: each item so listed is cleared of what it is listed with, and
// the view read again to see it gone.
func TestCreateUserClearsWhatJellyfinHandsOn(t *testing.T) {
	t.Parallel()

	const handedOn = `{"Items":[` +
		`{"Id":"a","Name":"Alien","Type":"Movie","UserData":{"Played":true,"PlayCount":1,"LastPlayedDate":"2026-01-02T03:04:05Z"}},` +
		`{"Id":"b","Name":"Arrival","Type":"Movie","UserData":{"IsFavorite":true}},` +
		`{"Id":"c","Name":"Dune","Type":"Movie","UserData":{"Likes":true,"Rating":10}},` +
		`{"Id":"d","Name":"Heat","Type":"Movie","UserData":{"PlaybackPositionTicks":6000000000,"PlayedPercentage":8.6}},` +
		`{"Id":"e","Name":"Tron","Type":"Movie","UserData":{"Played":false}},` +
		`{"Id":"s","Name":"Show","Type":"Series","UserData":{"UnplayedItemCount":3,"PlayedPercentage":50}}],"TotalRecordCount":6}`
	const clean = `{"Items":[{"Id":"a","Name":"Alien","Type":"Movie","UserData":{"Played":false}},{"Id":"b","Name":"Arrival","Type":"Movie","UserData":{"IsFavorite":false}}],"TotalRecordCount":2}`
	// the view as it is listed before anything is cleared, and after
	views := func(first, then string) route {
		var reads atomic.Int32

		return func(*http.Request, string) (int, string) {
			if reads.Add(1) == 1 {
				return http.StatusOK, first
			}

			return http.StatusOK, then
		}
	}
	made := func(routes map[string]route) map[string]route {
		routes["POST /Users/New"] = ok(`{"Id":"u9","Name":"carol"}`)
		routes["GET /Users"] = ok(`[{"Id":"u9","Name":"carol","Policy":{"EnableAllFolders":true}}]`)

		return routes
	}
	clears := []string{"DELETE /UserPlayedItems/a", "DELETE /UserFavoriteItems/b", "DELETE /UserItems/c/Rating", "DELETE /UserPlayedItems/d"}
	sent := func(f *fake) []string {
		f.mu.Lock()
		defer f.mu.Unlock()

		var got []string
		for _, r := range f.requests {
			if r.method == http.MethodDelete {
				got = append(got, r.method+" "+r.path)
				if r.query.Get("userId") != "u9" {
					t.Errorf("%s %s was sent for %q, want the new account", r.method, r.path, r.query.Get("userId"))
				}
			}
		}

		return got
	}

	c, f := newFake(t, Jellyfin, made(map[string]route{
		"GET /Items": views(handedOn, clean), clears[0]: ok(`{}`), clears[1]: ok(`{}`), clears[2]: ok(`{}`), clears[3]: ok(`{}`),
	}))
	u, left, err := c.CreateUser(t.Context(), "carol", "s3cret")
	if err != nil || u == nil || left != (Leftover{Cleared: 4}) {
		t.Fatalf("CreateUser = %+v, %+v, %v, want the four items cleared", u, left, err)
	}
	if got := sent(f); !slices.Equal(got, clears) {
		t.Errorf("cleared with %v, want %v: what each item is listed with and no more, and nothing for an item with none or with only what follows from its episodes", got, clears)
	}
	if len(f.all("GET /Items")) != 2 {
		t.Errorf("the view was read %d times, want once before the clearing and once after", len(f.all("GET /Items")))
	}

	// still listed with some afterwards: the account is made, and told of
	c, _ = newFake(t, Jellyfin, made(map[string]route{
		"GET /Items": views(handedOn, handedOn), clears[0]: ok(`{}`), clears[1]: ok(`{}`), clears[2]: ok(`{}`), clears[3]: ok(`{}`),
	}))
	u, left, err = c.CreateUser(t.Context(), "carol", "s3cret")
	if err != nil || u == nil || left.Cleared != 4 || !strings.Contains(left.Unchecked, "4 are still listed with some (Alien, for one)") {
		t.Errorf("CreateUser when the clearing did not take = %+v, %+v, %v", u, left, err)
	}

	// a clear that fails stops there and says how far it got
	c, f = newFake(t, Jellyfin, made(map[string]route{
		"GET /Items": views(handedOn, clean), clears[0]: ok(`{}`), clears[1]: answer(http.StatusInternalServerError, "boom"),
	}))
	u, left, err = c.CreateUser(t.Context(), "carol", "s3cret")
	if err != nil || u == nil || left.Cleared != 1 || !strings.Contains(left.Unchecked, "1 of the 4 items") || !strings.Contains(left.Unchecked, "clearing Arrival failed") || !strings.Contains(left.Unchecked, "boom") {
		t.Errorf("CreateUser when a clear failed = %+v, %+v, %v", u, left, err)
	}
	if got := sent(f); !slices.Equal(got, clears[:2]) {
		t.Errorf("after the clear that failed %v were sent, want no more than %v", got, clears[:2])
	}

	// the view cannot be read: the account is made, and nothing is cleared
	c, f = newFake(t, Jellyfin, made(map[string]route{"GET /Items": answer(http.StatusForbidden, "no")}))
	u, left, err = c.CreateUser(t.Context(), "carol", "s3cret")
	if err != nil || u == nil || left.Cleared != 0 || left.Unchecked == "" || len(sent(f)) != 0 {
		t.Errorf("CreateUser when its view could not be read = %+v, %+v, %v", u, left, err)
	}

	// a list that says more match than it holds cannot say the rest are clean
	c, _ = newFake(t, Jellyfin, made(map[string]route{"GET /Items": ok(`{"Items":[{"Id":"e","Name":"Tron","Type":"Movie","UserData":{"Played":false}}],"TotalRecordCount":7}`)}))
	if u, left, err = c.CreateUser(t.Context(), "carol", "s3cret"); err != nil || u == nil || left.Unchecked == "" {
		t.Errorf("CreateUser when the list came short = %+v, %+v, %v, want it said that the view could not be checked", u, left, err)
	}
}

func TestSetUserPassword(t *testing.T) {
	t.Parallel()

	c, f := newFake(t, Emby, map[string]route{"POST /Users/u2/Password": answer(http.StatusNoContent, "")})
	if err := c.SetUserPassword(t.Context(), "u2", "n3w"); err != nil {
		t.Fatal(err)
	}
	if body := jsonBody(t, f.only("POST /Users/u2/Password").body); body["NewPw"] != "n3w" || len(body) != 1 {
		t.Errorf("Emby was sent %v", body)
	}

	c, f = newFake(t, Jellyfin, map[string]route{"POST /Users/Password": answer(http.StatusNoContent, "")})
	if err := c.SetUserPassword(t.Context(), "u2", "n3w"); err != nil {
		t.Fatal(err)
	}
	sent := f.only("POST /Users/Password")
	if body := jsonBody(t, sent.body); body["NewPw"] != "n3w" || len(body) != 1 || sent.query.Get("userId") != "u2" {
		t.Errorf("Jellyfin was sent %v for %q", body, sent.query.Get("userId"))
	}

	// an empty password takes the account's away on both servers, so it is never sent
	for _, backend := range []Backend{Emby, Jellyfin} {
		c, f = newFake(t, backend, map[string]route{})
		if err := c.SetUserPassword(t.Context(), "u2", ""); !errors.Is(err, ErrNoPassword) || len(f.requests) != 0 {
			t.Errorf("%s: an empty password = %v after %d requests", backend, err, len(f.requests))
		}
	}
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()

	c, f := newFake(t, Emby, map[string]route{
		"DELETE /Users/u2": answer(http.StatusNoContent, ""),
		"GET /Users/Query": ok(`{"Items":[{"Id":"u1","Name":"root"}]}`),
	})
	if err := c.DeleteUser(t.Context(), "u2"); err != nil {
		t.Fatal(err)
	}
	f.only("DELETE /Users/u2")

	// answered, and still listed
	c, _ = newFake(t, Emby, map[string]route{
		"DELETE /Users/u2": answer(http.StatusNoContent, ""),
		"GET /Users/Query": ok(`{"Items":[{"Id":"u1","Name":"root"},{"Id":"U2","Name":"alice"}]}`),
	})
	if err := c.DeleteUser(t.Context(), "u2"); err == nil || !strings.Contains(err.Error(), "still lists") {
		t.Errorf("a delete the server answered and did not do = %v", err)
	}

	// an account that is not there is the server's 404
	c, _ = newFake(t, Emby, map[string]route{"DELETE /Users/u2": answer(http.StatusNotFound, "The requested user could not be found.")})
	if err := c.DeleteUser(t.Context(), "u2"); err == nil || !IsNotFound(err) {
		t.Errorf("deleting an account that is gone = %v, want its 404", err)
	}
}

// Jellyfin exits when an account is deleted within half a second of its watch state changing, so it is asked only once that time has passed, and
// never while the account is playing.
func TestDeleteUserGivesJellyfinTime(t *testing.T) {
	t.Parallel()

	var deleted atomic.Bool
	listed := func(*http.Request, string) (int, string) {
		if deleted.Load() {
			return http.StatusOK, `[{"Id":"u1","Name":"root"}]`
		}

		return http.StatusOK, `[{"Id":"u1","Name":"root"},{"Id":"u2","Name":"alice"}]`
	}
	var began time.Time
	var waited time.Duration
	c, f := newFake(t, Jellyfin, map[string]route{
		"GET /Users":    listed,
		"GET /Sessions": ok(`[{"Id":"s1","UserName":"root","DeviceName":"tv","NowPlayingItem":{"Id":"1","Name":"Alien"}},{"Id":"s2","UserName":"alice","DeviceName":"phone"}]`),
		"DELETE /Users/u2": func(*http.Request, string) (int, string) {
			waited = time.Since(began)
			deleted.Store(true)

			return http.StatusNoContent, ""
		},
	})
	c.userGrace = 150 * time.Millisecond
	began = time.Now()
	if err := c.DeleteUser(t.Context(), "u2"); err != nil {
		t.Fatal(err)
	}
	if waited < c.userGrace {
		t.Errorf("Jellyfin was asked to delete after %v, want no sooner than %v", waited, c.userGrace)
	}
	f.only("GET /Sessions")

	// playing: not deleted, and Jellyfin is not asked
	c, f = newFake(t, Jellyfin, map[string]route{
		"GET /Users":    ok(`[{"Id":"u1","Name":"root"},{"Id":"u2","Name":"alice"}]`),
		"GET /Sessions": ok(`[{"Id":"s2","UserName":"Alice","DeviceName":"phone","NowPlayingItem":{"Id":"1","Name":"Alien"}}]`),
	})
	c.userGrace = time.Millisecond
	err := c.DeleteUser(t.Context(), "u2")
	if !errors.Is(err, ErrUserPlaying) || !strings.Contains(err.Error(), "alice is playing Alien on phone") || len(f.all("DELETE /Users/u2")) != 0 {
		t.Errorf("deleting an account that is playing = %v, after %d deletes sent", err, len(f.all("DELETE /Users/u2")))
	}

	// whether it is playing cannot be told: not deleted
	c, f = newFake(t, Jellyfin, map[string]route{
		"GET /Users":    ok(`[{"Id":"u2","Name":"alice"}]`),
		"GET /Sessions": answer(http.StatusForbidden, "no"),
	})
	c.userGrace = time.Millisecond
	if err := c.DeleteUser(t.Context(), "u2"); err == nil || !strings.Contains(err.Error(), "could not tell whether alice is playing") || len(f.all("DELETE /Users/u2")) != 0 {
		t.Errorf("deleting when the sessions cannot be read = %v", err)
	}

	// a call given up on during the wait deletes nothing
	c, f = newFake(t, Jellyfin, map[string]route{
		"GET /Users":    ok(`[{"Id":"u2","Name":"alice"}]`),
		"GET /Sessions": ok(`[]`),
	})
	c.userGrace = time.Minute
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := c.DeleteUser(ctx, "u2"); !errors.Is(err, context.DeadlineExceeded) || len(f.all("DELETE /Users/u2")) != 0 {
		t.Errorf("a delete given up on while waiting = %v, after %d deletes sent", err, len(f.all("DELETE /Users/u2")))
	}

	// Emby is asked at once, and not about its sessions
	c, f = newFake(t, Emby, map[string]route{
		"DELETE /Users/u2": answer(http.StatusNoContent, ""),
		"GET /Users/Query": ok(`{"Items":[{"Id":"u1","Name":"root"}]}`),
	})
	c.userGrace = time.Minute
	if err := c.DeleteUser(t.Context(), "u2"); err != nil || len(f.all("GET /Sessions")) != 0 {
		t.Errorf("Emby's delete = %v, after %d reads of its sessions", err, len(f.all("GET /Sessions")))
	}
}

func TestSetUserLibraries(t *testing.T) {
	t.Parallel()

	// the policy goes back whole, a setting this client has never heard of included, with the two that say which libraries changed. Emby lists a
	// library by its Guid
	var sent atomic.Bool
	listed := func(*http.Request, string) (int, string) {
		if sent.Load() {
			return http.StatusOK, `{"Items":[{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":false,"EnabledFolders":["guid-movies"]}}]}`
		}

		return http.StatusOK, `{"Items":[{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":true}}]}`
	}
	c, f := newFake(t, Emby, map[string]route{
		"GET /Users/Query": listed,
		"GET /Users/u2":    ok(`{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":true,"EnabledFolders":[],"SomethingNew":{"Kept":12345678901234567890},"IsAdministrator":false}}`),
		"POST /Users/u2/Policy": func(*http.Request, string) (int, string) {
			sent.Store(true)

			return http.StatusNoContent, ""
		},
	})
	before, after, err := c.SetUserLibraries(t.Context(), "u2", false, []VirtualFolder{{Name: "Movies", ItemID: "3", GUID: "guid-movies"}})
	if err != nil {
		t.Fatal(err)
	}
	if !before.Policy.EnableAllFolders || after.Policy.EnableAllFolders || !slices.Equal(after.Policy.EnabledFolders, []string{"guid-movies"}) {
		t.Errorf("before %+v, after %+v", before.Policy, after.Policy)
	}
	raw := f.only("POST /Users/u2/Policy").body
	if !strings.Contains(raw, `"Kept":12345678901234567890`) {
		t.Errorf("the policy sent lost a setting or its digits: %s", raw)
	}
	if body := jsonBody(t, raw); !anyIs(body["EnableAllFolders"], false) || !slices.Equal(anyStrings(body["EnabledFolders"]), []string{"guid-movies"}) || !anyIs(body["IsAdministrator"], false) {
		t.Errorf("the policy sent = %v", body)
	}

	// Jellyfin lists a library by its ItemId, and every library is the setting with an empty list
	c, f = newFake(t, Jellyfin, map[string]route{
		"GET /Users":            ok(`[{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":false,"EnabledFolders":["lib-a"]}}]`),
		"GET /Users/u2":         ok(`{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":false,"EnabledFolders":["lib-a"]}}`),
		"POST /Users/u2/Policy": answer(http.StatusNoContent, ""),
	})
	if _, _, err := c.SetUserLibraries(t.Context(), "u2", false, []VirtualFolder{{Name: "Shows", ItemID: "lib-b"}}); err != nil {
		t.Fatal(err)
	}
	if body := jsonBody(t, f.only("POST /Users/u2/Policy").body); !slices.Equal(anyStrings(body["EnabledFolders"]), []string{"lib-b"}) {
		t.Errorf("Jellyfin was sent %v", body)
	}

	c, f = newFake(t, Jellyfin, map[string]route{
		"GET /Users":            ok(`[{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":false,"EnabledFolders":["lib-a"]}}]`),
		"GET /Users/u2":         ok(`{"Id":"u2","Name":"alice","Policy":{"EnableAllFolders":false,"EnabledFolders":["lib-a"]}}`),
		"POST /Users/u2/Policy": answer(http.StatusNoContent, ""),
	})
	if _, _, err := c.SetUserLibraries(t.Context(), "u2", true, nil); err != nil {
		t.Fatal(err)
	}
	if body := jsonBody(t, f.only("POST /Users/u2/Policy").body); !anyIs(body["EnableAllFolders"], true) || len(anyStrings(body["EnabledFolders"])) != 0 {
		t.Errorf("every library was sent as %v", body)
	}

	// an account with no policy in the answer is left alone
	c, f = newFake(t, Emby, map[string]route{
		"GET /Users/Query": ok(`{"Items":[{"Id":"u2","Name":"alice"}]}`),
		"GET /Users/u2":    ok(`{"Id":"u2","Name":"alice"}`),
	})
	if _, _, err := c.SetUserLibraries(t.Context(), "u2", true, nil); err == nil || len(f.all("POST /Users/u2/Policy")) != 0 {
		t.Errorf("an account read without its policy = %v, and %d writes", err, len(f.all("POST /Users/u2/Policy")))
	}
}

// anyStrings is a decoded JSON list as strings, what is not a string as it prints.
func anyStrings(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			s = fmt.Sprint(e)
		}
		out = append(out, s)
	}

	return out
}

// anyIs says whether a decoded JSON value is the boolean wanted.
func anyIs(v any, want bool) bool {
	got, ok := v.(bool)

	return ok && got == want
}
