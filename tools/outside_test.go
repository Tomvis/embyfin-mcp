package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Every name the registry is told to leave as it is, or to blank addresses under, is one some tool answers under: a name left behind when a field is
// renamed would protect nothing, and nobody would notice.
func TestTheAnswerRulesNameWhatToolsAnswer(t *testing.T) {
	t.Parallel()

	r := &registry{client: newTestClient(t)}
	queueTools(r)

	if unused := r.queued().LeaveAsIsUnused(); len(unused) != 0 {
		t.Errorf("handles names %v, which no tool answers under", unused)
	}
	if unused := r.queued().BlankAddressesUnused(); len(unused) != 0 {
		t.Errorf("serverSaid names %v, which no tool answers under", unused)
	}
}

// What a tool answers is cleaned around the tool: text has what does not show taken out and is cut when longer than anything a tool means to send,
// a path and a value to hand back are left exactly as held, and a server's words lose what an address in them may carry a key in.
func TestAnAnswerIsCleanedAndItsHandlesAreNot(t *testing.T) {
	t.Parallel()

	type out struct {
		Name     string `json:"name"`
		Overview string `json:"overview"`
		Path     string `json:"path"`
		Value    string `json:"value"`
		Error    string `json:"error"`
		URL      string `json:"url"`
	}
	const hidden = "Zzy\u200bzx\tFiles\u2060"
	const address = "http://quux:hunter2@nas:8096/Items?api_key=abc123&Limit=5#frag" //nolint:gosec // a made-up address with a made-up password, for the test that neither is shown
	r := &registry{client: newTestClient(t)}
	type none struct{}
	add(r, readTool, &mcp.Tool{Name: "zzyzx_text"}, func(context.Context, *mcp.CallToolRequest, none) (*mcp.CallToolResult, out, error) {
		return nil, out{Name: hidden, Overview: strings.Repeat("z", answerLength+50), Path: "/media/" + hidden, Value: hidden, Error: "asking " + address + " failed", URL: address}, nil
	})
	got := mustCall(t, hostRegistry(t, r), "zzyzx_text", map[string]any{})

	if got["name"] != "Zzyzx Files" {
		t.Errorf("a name = %q, want what does not show taken out and the tab a space", got["name"])
	}
	if overview := text(got["overview"]); len([]rune(overview)) != answerLength || !strings.HasSuffix(overview, "…") {
		t.Errorf("a very long overview is %d characters ending %q, want it cut to %d and marked", len([]rune(overview)), overview[max(0, len(overview)-6):], answerLength)
	}
	if got["path"] != "/media/"+hidden || got["value"] != hidden || got["url"] != address {
		t.Errorf("a path, a value to hand back and a link = %q, %q, %q; want each exactly as held", got["path"], got["value"], got["url"])
	}
	if said := text(got["error"]); strings.Contains(said, "hunter2") || strings.Contains(said, "abc123") || !strings.Contains(said, "nas:8096/Items") || !strings.Contains(said, "api_key=") {
		t.Errorf("a server's words = %q, want the address without who it signs in as or the value of a parameter, its host and path kept", said)
	}
}

// A log's tail is shown as a search of the same log shows its lines: a key in an address is blanked, and the line breaks stay.
func TestALogsTailHidesTheKeysInIt(t *testing.T) {
	t.Parallel()

	f := newFakeServer(t)
	f.mux.HandleFunc("GET /System/Logs/Query", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, page(map[string]any{"Name": "embyserver.txt", "Size": 20, "DateModified": "2026-09-20T00:00:00.0000000Z"}))
	})
	f.mux.HandleFunc("GET /System/Logs/{name}", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "Info HttpServer: HTTP GET http://nas:8096/emby/Items/9/Images/Primary?maxHeight=300&api_key=abc123def456\nInfo App: two\n")
	})

	tail := text(mustCall(t, session(t, f, Options{}), "server_log", map[string]any{})["tail"])
	if tail != "Info HttpServer: HTTP GET http://nas:8096/emby/Items/9/Images/Primary?maxHeight=300&api_key=***\nInfo App: two" {
		t.Errorf("the tail = %q, want the key blanked and the rest as the log has it", tail)
	}
}
