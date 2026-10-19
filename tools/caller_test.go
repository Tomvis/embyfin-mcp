package tools

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// usersFake is a server whose first administrator, home, is not the person
// the instance is for.
func usersFake(t *testing.T) *fakeServer {
	t.Helper()

	f := newFakeServer(t)
	admin := map[string]any{"IsAdministrator": true, "EnableAllFolders": true}
	f.mux.HandleFunc("GET /Users/Query", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, page(
			map[string]any{"Id": "h1", "Name": "home", "Policy": admin},
			map[string]any{"Id": "t1", "Name": "tom", "Policy": admin},
			map[string]any{"Id": "l1", "Name": "lera", "Policy": map[string]any{"EnableAllFolders": true}},
		))
	})
	f.mux.HandleFunc("GET /Library/VirtualFolders/Query", func(w http.ResponseWriter, _ *http.Request) { writeJSON(t, w, page()) })

	return f
}

func userNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()

	out, refusal := callTool(t, cs, "user_list", map[string]any{})
	if refusal != "" {
		t.Fatal(refusal)
	}
	var names []string
	for _, u := range out["users"].([]any) {
		names = append(names, u.(map[string]any)["name"].(string))
	}

	return names
}

func TestUserDefaultsToConfigured(t *testing.T) {
	t.Parallel()

	cs := session(t, usersFake(t), Options{User: "tom"})
	if out, refusal := callTool(t, cs, "user_get", map[string]any{}); refusal != "" || out["name"] != "tom" {
		t.Errorf("user_get with no user = %v %q, want tom", out["name"], refusal)
	}
	if out, refusal := callTool(t, cs, "user_get", map[string]any{"user": "lera"}); refusal != "" || out["name"] != "lera" {
		t.Errorf("user_get lera = %v %q, want lera", out["name"], refusal)
	}
	if got := userNames(t, cs); len(got) != 3 {
		t.Errorf("user_list = %v, want every account", got)
	}
}

func TestUserOnlyRefusesOthers(t *testing.T) {
	t.Parallel()

	cs := session(t, usersFake(t), Options{User: "lera", UserOnly: true})
	if out, refusal := callTool(t, cs, "user_get", map[string]any{}); refusal != "" || out["name"] != "lera" {
		t.Errorf("user_get with no user = %v %q, want lera", out["name"], refusal)
	}
	if out, refusal := callTool(t, cs, "user_get", map[string]any{"user": "LERA"}); refusal != "" || out["name"] != "lera" {
		t.Errorf("user_get LERA = %v %q, want lera", out["name"], refusal)
	}
	for _, other := range []string{"tom", "h1"} {
		if _, refusal := callTool(t, cs, "user_get", map[string]any{"user": other}); !strings.Contains(refusal, "only for lera") {
			t.Errorf("user_get %s: refusal %q, want only for lera", other, refusal)
		}
	}
	if got := userNames(t, cs); len(got) != 1 || got[0] != "lera" {
		t.Errorf("user_list = %v, want [lera]", got)
	}
}

func TestUserOnlyNeedsUser(t *testing.T) {
	t.Parallel()

	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	if _, err := RegisterAll(srv, usersFake(t).client(t), Options{UserOnly: true}); err == nil {
		t.Error("RegisterAll with user-only and no user: no error")
	}
}
