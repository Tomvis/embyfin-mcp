package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// accountServer is a canned Emby holding root and alice, two libraries, and whatever accounts the tools make on it: it keeps what it was sent so a
// test can read the account back the way a real server would give it.
type accountServer struct {
	*fakeServer

	mu        sync.Mutex
	users     []map[string]any
	passwords map[string]string
	bodies    map[string][]string
}

func newAccountServer(t *testing.T) *accountServer {
	t.Helper()

	s := &accountServer{fakeServer: newFakeServer(t), passwords: map[string]string{}, bodies: map[string][]string{}}
	s.users = []map[string]any{
		{"Id": "u1", "Name": "root", "HasPassword": true, "Policy": map[string]any{"IsAdministrator": true, "EnableAllFolders": true, "EnabledFolders": []any{}, "EnableAllChannels": true}},
		{"Id": "u2", "Name": "alice", "HasPassword": true, "LastActivityDate": "2026-09-15T00:00:00.1234567Z", "Policy": map[string]any{"EnableAllFolders": false, "EnabledFolders": []any{"guid-movies"}, "EnableRemoteAccess": true}},
	}
	read := func(r *http.Request, key string) string {
		raw, _ := io.ReadAll(r.Body)
		s.bodies[key] = append(s.bodies[key], string(raw))

		return string(raw)
	}
	find := func(id string) map[string]any {
		for _, u := range s.users {
			if u["Id"] == id {
				return u
			}
		}

		return nil
	}

	s.mux.HandleFunc("GET /Users/Query", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(t, w, map[string]any{"Items": s.users, "TotalRecordCount": len(s.users)})
	})
	s.mux.HandleFunc("GET /Library/VirtualFolders/Query", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"Items": []map[string]any{
			{"Name": "Movies", "ItemId": "3", "Guid": "guid-movies", "CollectionType": "movies", "Locations": []string{"/media/movies"}},
			{"Name": "Shows", "ItemId": "5", "Guid": "guid-shows", "CollectionType": "tvshows", "Locations": []string{"/media/shows"}},
		}})
	})
	s.mux.HandleFunc("POST /Users/New", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var in struct{ Name string }
		_ = json.Unmarshal([]byte(read(r, "new")), &in)
		u := map[string]any{"Id": "u9", "Name": in.Name, "HasPassword": false, "Policy": map[string]any{"EnableAllFolders": true, "EnabledFolders": []any{}, "EnableRemoteAccess": true, "SomethingNew": "kept"}}
		s.users = append(s.users, u)
		writeJSON(t, w, u)
	})
	s.mux.HandleFunc("POST /Users/{id}/Password", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var in struct{ NewPw string }
		_ = json.Unmarshal([]byte(read(r, "password")), &in)
		u := find(r.PathValue("id"))
		if u == nil {
			http.Error(w, "The requested user could not be found.", http.StatusNotFound)
			return
		}
		s.passwords[r.PathValue("id")], u["HasPassword"] = in.NewPw, in.NewPw != ""
		w.WriteHeader(http.StatusNoContent)
	})
	s.mux.HandleFunc("GET /Users/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		u := find(r.PathValue("id"))
		if u == nil {
			http.Error(w, "The requested user could not be found.", http.StatusNotFound)
			return
		}
		writeJSON(t, w, u)
	})
	s.mux.HandleFunc("POST /Users/{id}/Policy", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var policy map[string]any
		_ = json.Unmarshal([]byte(read(r, "policy")), &policy)
		find(r.PathValue("id"))["Policy"] = policy
		w.WriteHeader(http.StatusNoContent)
	})
	s.mux.HandleFunc("DELETE /Users/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.users = slices.DeleteFunc(s.users, func(u map[string]any) bool { return u["Id"] == r.PathValue("id") })
		w.WriteHeader(http.StatusNoContent)
	})

	return s
}

// names is the accounts the canned server holds now.
func (s *accountServer) names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, 0, len(s.users))
	for _, u := range s.users {
		if name, ok := u["Name"].(string); ok {
			out = append(out, name)
		}
	}

	return out
}

func TestUserCreate(t *testing.T) {
	t.Parallel()

	s := newAccountServer(t)
	cs := session(t, s.fakeServer, Options{EnableDelete: true})

	// refused before anything is sent: no name, no password, a name in use in another case, a library that is not there
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"name": "  ", "password": "s3cret"}, "name is required"},
		{map[string]any{"name": "carol", "password": ""}, "a password is needed"},
		{map[string]any{"name": "ALICE", "password": "s3cret"}, `an account named "alice" exists (id u2)`},
		{map[string]any{"name": "carol", "password": "s3cret", "libraries": []string{"Nope"}}, `no library named "Nope" or with that id: the server has Movies, Shows. No account was made`},
	} {
		if msg := mustRefuse(t, cs, "user_create", tc.args); !strings.Contains(msg, tc.want) {
			t.Errorf("user_create %v = %q, want %q in it", tc.args, msg, tc.want)
		}
	}
	if got := s.names(); !slices.Equal(got, []string{"root", "alice"}) || len(s.requests("/Users/New")) != 0 {
		t.Fatalf("a refused create left %v after %d creates sent", got, len(s.requests("/Users/New")))
	}

	// made with its password, and given the one library asked for; the policy goes back with what this client never heard of
	out := mustCall(t, cs, "user_create", map[string]any{"name": " carol ", "password": "s3cret", "libraries": []string{"shows"}})
	if out["name"] != "carol" || out["id"] != "u9" || boolean(t, out["admin"], "admin") || boolean(t, out["all_libraries"], "all_libraries") || strings.Join(texts(out["libraries"]), ",") != "Shows" || !boolean(t, out["remote_access"], "remote_access") || !strings.Contains(text(out["note"]), "outside the local network") {
		t.Errorf("user_create = %v", out)
	}
	if raw, _ := json.Marshal(out); strings.Contains(string(raw), "s3cret") {
		t.Errorf("the answer holds the password: %s", raw)
	}
	if s.passwords["u9"] != "s3cret" || !strings.Contains(s.bodies["new"][0], `"carol"`) || strings.Contains(s.bodies["new"][0], "s3cret") {
		t.Errorf("the password kept is %q, and the create was sent %s", s.passwords["u9"], s.bodies["new"])
	}
	if policy := s.bodies["policy"][0]; !strings.Contains(policy, `"SomethingNew":"kept"`) || !strings.Contains(policy, `"guid-shows"`) || !strings.Contains(policy, `"EnableAllFolders":false`) {
		t.Errorf("the policy sent = %s", policy)
	}
}

func TestUserEdit(t *testing.T) {
	t.Parallel()

	s := newAccountServer(t)
	cs := session(t, s.fakeServer, Options{EnableDelete: true})

	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"user": "", "all_libraries": true}, "user is required"},
		{map[string]any{"user": "alice"}, "nothing to change"},
		{map[string]any{"user": "alice", "libraries": []string{"Movies"}, "all_libraries": true}, "two answers to one question"},
		{map[string]any{"user": "alice", "libraries": []string{"Nope"}}, "Nothing was changed"},
		{map[string]any{"user": "nobody", "all_libraries": true}, `no user named "nobody"`},
	} {
		if msg := mustRefuse(t, cs, "user_edit", tc.args); !strings.Contains(msg, tc.want) {
			t.Errorf("user_edit %v = %q, want %q in it", tc.args, msg, tc.want)
		}
	}
	if len(s.bodies["policy"])+len(s.bodies["password"]) != 0 {
		t.Fatalf("a refused edit sent %v", s.bodies)
	}

	// exactly the libraries named, by name or by either id, each once
	out := mustCall(t, cs, "user_edit", map[string]any{"user": "alice", "libraries": []string{"Shows", "guid-movies", "5"}})
	was, ok := out["was"].(map[string]any)
	if !ok {
		t.Fatalf("user_edit libraries answered was as %v, want what the account saw before", out["was"])
	}
	if strings.Join(texts(out["changed"]), ",") != "libraries" || strings.Join(texts(out["libraries"]), ",") != "Movies,Shows" || boolean(t, out["all_libraries"], "all_libraries") || strings.Join(texts(was["libraries"]), ",") != "Movies" {
		t.Errorf("user_edit libraries = %v", out)
	}
	if policy := s.bodies["policy"][0]; !strings.Contains(policy, `["guid-shows","guid-movies"]`) {
		t.Errorf("the policy sent = %s", policy)
	}

	// every library, and a password with it
	out = mustCall(t, cs, "user_edit", map[string]any{"user": "u2", "all_libraries": true, "password": "n3w"})
	if strings.Join(texts(out["changed"]), ",") != "libraries,password" || !boolean(t, out["all_libraries"], "all_libraries") || s.passwords["u2"] != "n3w" {
		t.Errorf("user_edit = %v, password kept %q", out, s.passwords["u2"])
	}
	if raw, _ := json.Marshal(out); strings.Contains(string(raw), "n3w") {
		t.Errorf("the answer holds the password: %s", raw)
	}

	// a password alone changes no policy
	sent := len(s.bodies["policy"])
	out = mustCall(t, cs, "user_edit", map[string]any{"user": "alice", "password": "0ther"})
	if strings.Join(texts(out["changed"]), ",") != "password" || out["was"] != nil || len(s.bodies["policy"]) != sent || s.passwords["u2"] != "0ther" {
		t.Errorf("user_edit password = %v after %d policy writes", out, len(s.bodies["policy"])-sent)
	}
}

func TestUserDelete(t *testing.T) {
	t.Parallel()

	s := newAccountServer(t)
	cs := session(t, s.fakeServer, Options{EnableDelete: true})

	// refused without confirm, saying what the account is
	msg := mustRefuse(t, cs, "user_delete", map[string]any{"user": "alice"})
	for _, want := range []string{"refusing to delete the account alice (id u2) without confirm=true: nothing was changed", "an ordinary account that sees Movies", "last active 2026-09-15T00:00:00.1234567Z", "playlists are not removed with it"} {
		if !strings.Contains(msg, want) {
			t.Errorf("user_delete without confirm = %q, want %q in it", msg, want)
		}
	}
	// the only administrator is never deleted, confirmed or not
	if msg := mustRefuse(t, cs, "user_delete", map[string]any{"user": "root", "confirm": true}); !strings.Contains(msg, "only administrator") || !strings.Contains(msg, "Nothing was deleted") {
		t.Errorf("deleting the only administrator = %q", msg)
	}
	if msg := mustRefuse(t, cs, "user_delete", map[string]any{"user": "", "confirm": true}); !strings.Contains(msg, "user is required") {
		t.Errorf("deleting nobody = %q", msg)
	}
	if got := s.names(); !slices.Equal(got, []string{"root", "alice"}) {
		t.Fatalf("a refused delete left %v", got)
	}

	out := mustCall(t, cs, "user_delete", map[string]any{"user": "ALICE", "confirm": true})
	if out["deleted"] != "alice" || out["id"] != "u2" || !strings.Contains(text(out["note"]), "playlists are not removed") || !slices.Equal(s.names(), []string{"root"}) {
		t.Errorf("user_delete = %v, leaving %v", out, s.names())
	}
}

// The delete tool is one a session has only when deleting is enabled, and the other two are writes a read-only session does not have.
func TestAccountToolsAreGated(t *testing.T) {
	t.Parallel()

	names := func(opts Options) []string {
		opts.Toolsets = []string{"all"}
		list, err := Describe(opts)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, ti := range list {
			if strings.HasPrefix(ti.Name, "user_") {
				out = append(out, ti.Name)
			}
		}
		slices.Sort(out)

		return out
	}
	if got := names(Options{ReadOnly: true}); slices.Contains(got, "user_create") || slices.Contains(got, "user_edit") || slices.Contains(got, "user_delete") {
		t.Errorf("a read-only session has %v", got)
	}
	if got := names(Options{}); !slices.Contains(got, "user_create") || !slices.Contains(got, "user_edit") || slices.Contains(got, "user_delete") {
		t.Errorf("a session without deletes has %v", got)
	}
	if got := names(Options{EnableDelete: true}); !slices.Contains(got, "user_delete") {
		t.Errorf("a session with deletes has %v", got)
	}
}

// What user_create says of the watch state Jellyfin listed a new account with: nothing when there was none, how much when it was cleared, and that
// lists may still be wrong when it could not be.
func TestLeftoverNote(t *testing.T) {
	t.Parallel()

	if note := leftoverNote(embyfin.Leftover{}); note != "" {
		t.Errorf("the note for an account handed nothing = %q, want none", note)
	}
	if note := leftoverNote(embyfin.Leftover{Cleared: 3}); !strings.Contains(note, "on 3 of the items it sees, each cleared again") || strings.Contains(note, "could not") {
		t.Errorf("the note for three items cleared = %q", note)
	}
	if note := leftoverNote(embyfin.Leftover{Cleared: 1, Unchecked: "clearing Alien failed: boom"}); !strings.Contains(note, "could not be cleared of them: clearing Alien failed: boom") || !strings.Contains(note, "may be another account's") {
		t.Errorf("the note for a clearing that failed = %q, want why, and that its lists may be another's", note)
	}
}
