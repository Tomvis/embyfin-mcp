//go:build acceptance

package tools_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/katbyte/go-kt/mcp/acctest"

	mediaacc "github.com/katbyte/embyfin-mcp/lib/acceptance"

	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
)

func TestServerInfo(t *testing.T) {
	out := suite.Call(t, "server_info", nil)
	if acctest.Str(out["backend"]) != string(backend) {
		t.Errorf("backend = %v, want %s", out["backend"], backend)
	}
	if acctest.Str(out["server_version"]) == "" || !strings.HasPrefix(acctest.Str(out["server_name"]), "embyfin-mcp-") {
		t.Errorf("server_info = %v", out)
	}
	// the API this binary's client was generated from, which is not the
	// server's own version when the server has moved on since
	want := emby.APIVersion
	if isJellyfin() {
		want = jf.APIVersion
	}
	if got := acctest.Str(out["sdk_api_version"]); got != want {
		t.Errorf("sdk_api_version = %q, want %q", got, want)
	}
	// and the pair it is built for, whichever of the two it is talking to
	if pair, ok := out["sdk_api_versions"].(map[string]any); !ok || pair["emby"] != emby.APIVersion || pair["jellyfin"] != jf.APIVersion {
		t.Errorf("sdk_api_versions = %v, want emby %s and jellyfin %s", out["sdk_api_versions"], emby.APIVersion, jf.APIVersion)
	}
	// Emby says what it runs on; Jellyfin 12.2 answers with an empty
	// operating system, and the one its log names at startup is given
	if got := acctest.Str(out["operating_system"]); got != "Linux" && (!isJellyfin() || !strings.Contains(got, "Linux")) {
		t.Errorf("operating_system = %q, want Linux, or on Jellyfin the Linux its log names", got)
	}
	// what neither server's API gives is read from the top of its log, which
	// a container started for this suite begins with: the processors it
	// counted, its architecture, when it started and where it keeps things
	paths := mediaacc.ObjectOf(out["paths"])
	if acctest.Num(t, out["processors"], "processors") < 1 || acctest.Str(out["architecture"]) == "" || acctest.Str(out["started"]) == "" || acctest.Str(paths["data"]) != "/config" || acctest.Str(paths["logs"]) == "" || out["note"] != nil {
		t.Errorf("what the server says of its machine = processors %v, architecture %v, started %v, paths %v, note %v", out["processors"], out["architecture"], out["started"], paths, out["note"])
	}
	// a server just set up waits for no restart, and both say so
	if !acctest.IsBool(out["pending_restart"], false) || out["update_available"] == nil {
		t.Errorf("pending_restart = %v, update_available = %v", out["pending_restart"], out["update_available"])
	}
	// this binary's own build, so a session can tell it is not running the
	// fix it thinks it is; a test build stamps nothing and reports dev or the
	// module version, but never nothing
	if acctest.Str(out["embyfin_mcp_version"]) == "" {
		t.Errorf("server_info does not say which embyfin-mcp build answered: %v", out)
	}
}

func TestServerStats(t *testing.T) {
	out := suite.Call(t, "server_stats", nil)
	// the clean movies and the messy ones
	if got, want := acctest.Num(t, out["movies"], "movies"), len(movies)+messyMovies(); got != want {
		t.Errorf("movies = %d, want %d", got, want)
	}
	if got, want := acctest.Num(t, out["series"], "series"), len(shows)+messySeries; got != want {
		t.Errorf("series = %d, want %d", got, want)
	}
	if got, want := acctest.Num(t, out["episodes"], "episodes"), showEpisodes()+messyEpisodes(); got != want {
		t.Errorf("episodes = %d, want %d", got, want)
	}
	if got := acctest.Num(t, out["albums"], "albums"); got != musicAlbums() {
		t.Errorf("albums = %d, want %d", got, musicAlbums())
	}
	if got := acctest.Num(t, out["songs"], "songs"); got != songs() {
		t.Errorf("songs = %d, want %d", got, songs())
	}
	if got := acctest.Num(t, out["users"], "users"); got != 2 {
		t.Errorf("users = %d, want root and alice", got)
	}
	// what is playing is TestSessionWhilePlaying's to show

	// a collection is counted, and one fewer once it is gone
	before := acctest.NumOr0(out["collections"])
	deleteLater(t, "collection_delete", "collection", acctest.Str(suite.Call(t, "collection_create", map[string]any{"name": "Zzyzx Counted", "item_ids": []any{findItem(t, "Movies", "Movie", "Alien")}})["id"]))
	if !acctest.Eventually(func() bool { return acctest.NumOr0(suite.Call(t, "server_stats", nil)["collections"]) == before+1 }) {
		t.Errorf("collections = %v after one was made, want %d", suite.Call(t, "server_stats", nil)["collections"], before+1)
	}
}

// The activity log is where the user history tools read from, so the scan
// and the logins the harness caused must show up in it.
func TestServerActivity(t *testing.T) {
	out := suite.Call(t, "server_activity", map[string]any{"days": 1, "limit": 100})
	entries := acctest.Rows(t, out["entries"], "entries")
	if len(entries) < 2 {
		t.Fatalf("activity = %v, want the logins and more", entries)
	}
	if acctest.Num(t, out["total"], "total") < len(entries) || acctest.Num(t, out["offset"], "offset") != 0 {
		t.Errorf("total %v offset %v for %d entries", out["total"], out["offset"], len(entries))
	}
	// paged by offset: the second entry heads a page that skips the first.
	// Matched by what it says rather than its stamp: Emby orders two entries
	// written milliseconds apart either way round from one read to the next
	page := suite.Call(t, "server_activity", map[string]any{"days": 1, "limit": 1, "offset": 1})
	got := acctest.Rows(t, page["entries"], "entries")
	if acctest.Num(t, page["offset"], "offset") != 1 || len(got) != 1 || acctest.Str(got[0]["type"]) != acctest.Str(entries[1]["type"]) || acctest.Str(got[0]["summary"]) != acctest.Str(entries[1]["summary"]) {
		t.Errorf("page at offset 1 = %v, want %v", page, entries[1])
	}
	// every entry is dated, typed, said, and graded the way its server
	// grades: Emby Info, Warn or Error, Jellyfin in full words
	severities := []string{"Info", "Warn", "Error"}
	if isJellyfin() {
		severities = []string{"Information", "Warning", "Error"}
	}
	stamps := make([]time.Time, 0, len(entries))
	for _, e := range entries {
		if acctest.Str(e["type"]) == "" || acctest.Str(e["summary"]) == "" || !slices.Contains(severities, acctest.Str(e["severity"])) {
			t.Errorf("entry = %v, want a type, a summary and one of %v", e, severities)
		}
		stamps = append(stamps, stamp(t, e["date"]))
	}
	// newest first, the stamps read as times: as strings, a fraction of a
	// second written shorter sorts after a longer one
	if !slices.IsSortedFunc(stamps, func(a, b time.Time) int { return b.Compare(a) }) {
		t.Errorf("entries are not newest first: %v", stamps)
	}
	// the setup's login is among the day's, as the information it is
	day := acctest.Rows(t, suite.Call(t, "server_activity", map[string]any{"days": 1, "limit": 5000})["entries"], "entries")
	login := slices.IndexFunc(day, func(e map[string]any) bool {
		return strings.HasPrefix(acctest.Str(e["summary"]), "root ") && strings.Contains(strings.ToLower(acctest.Str(e["type"])), "auth")
	})
	if login < 0 || acctest.Str(day[login]["severity"]) != severities[0] {
		t.Errorf("no login by root among the day's %d entries", len(day))
	}
}

func TestServerDevices(t *testing.T) {
	out := suite.Call(t, "server_devices", nil)
	// the testenv script logged in as root from a device of its own
	var setup map[string]any
	for _, d := range acctest.Rows(t, out["devices"], "devices") {
		if acctest.Str(d["name"]) == "testenv" {
			setup = d
		}
	}
	if setup == nil || acctest.Str(setup["app"]) != "embyfin-mcp-testenv 0" || acctest.Str(setup["last_user"]) != "root" {
		t.Errorf("the setup's device = %v, among %v", setup, out["devices"])
	}
	if setup != nil && stamp(t, setup["last_activity"]).After(time.Now().Add(time.Minute)) {
		t.Errorf("the setup's device was last active at %v", setup["last_activity"])
	}
}

func TestServerLog(t *testing.T) {
	// one call lists the files and tails the server's own log
	tail := suite.Call(t, "server_log", map[string]any{"lines": 5})
	files := acctest.Rows(t, tail["files"], "files")
	if len(files) == 0 {
		t.Fatal("no log files")
	}
	// the newest by when it was written, read as times rather than strings
	newest := files[0]
	for _, f := range files {
		if acctest.Str(f["name"]) == "" {
			t.Errorf("log file row lacks a name: %v", f)
		}
		if stamp(t, f["modified"]).After(stamp(t, newest["modified"])) {
			newest = f
		}
	}
	// in bytes, like every size a tool answers with: the logs a fresh server
	// writes are a few kilobytes, which in megabytes read as 0
	if acctest.NumOr0(newest["size"]) < 1024 {
		t.Errorf("the newest log's size = %v, want bytes", newest["size"])
	}

	// the default is the server's own log, not whichever file changed last
	// (a transcode's, while something plays): Emby's embyserver.txt, and
	// Jellyfin's newest log_<date>.log - on a fresh server the newest file
	// either way. The tail is the lines asked for
	own := "embyserver.txt"
	if isJellyfin() {
		own = acctest.Str(newest["name"])
		if !strings.HasPrefix(own, "log_") {
			t.Errorf("Jellyfin's newest log is %s, want its own log_<date>.log", own)
		}
	}
	if acctest.Str(tail["name"]) != own || tail["note"] != nil {
		t.Errorf("server_log read %v (note %v), want the server's own log %s", tail["name"], tail["note"], own)
	}
	if n := len(strings.Split(acctest.Str(tail["tail"]), "\n")); n != 5 {
		t.Errorf("tail has %d lines, want 5", n)
	}

	// a named file
	named := suite.Call(t, "server_log", map[string]any{"name": acctest.Str(files[len(files)-1]["name"]), "lines": 1})
	if acctest.Str(named["name"]) != acctest.Str(files[len(files)-1]["name"]) || strings.Contains(acctest.Str(named["tail"]), "\n") {
		t.Errorf("server_log of %v = %v", files[len(files)-1]["name"], named)
	}
	// Jellyfin answers 404, Emby 500; either way the tool fails rather than
	// handing back an empty tail
	status := "HTTP 500"
	if isJellyfin() {
		status = "HTTP 404"
	}
	if msg := suite.CallErr(t, "server_log", map[string]any{"name": "no-such.log"}); !strings.Contains(msg, status) {
		t.Errorf("a log that is not there: %s", msg)
	}
}

// taskRun is when task_list says a task last finished, the zero time when it
// never has.
func taskRun(t *testing.T, name string) (finished time.Time, task map[string]any) {
	t.Helper()

	for _, task := range acctest.Rows(t, suite.Call(t, "task_list", nil)["tasks"], "tasks") {
		if strings.EqualFold(acctest.Str(task["name"]), name) {
			if task["last_run"] == nil {
				return time.Time{}, task
			}
			return stamp(t, task["last_run"]), task
		}
	}
	t.Fatalf("no task %s", name)

	return time.Time{}, nil
}

func TestTasks(t *testing.T) {
	out := suite.Call(t, "task_list", nil)
	tasks := acctest.Rows(t, out["tasks"], "tasks")
	var scan map[string]any
	for _, task := range tasks {
		if strings.EqualFold(acctest.Str(task["name"]), "scan media library") {
			scan = task
		}
		if acctest.Str(task["name"]) == "" || acctest.Str(task["state"]) == "" {
			t.Errorf("task row = %v", task)
		}
	}
	if scan == nil {
		t.Fatalf("no scan task among %d tasks", len(tasks))
	}
	// the harness ran a scan, so it has a last result
	if acctest.Str(scan["last_status"]) != "Completed" || acctest.Str(scan["category"]) != "Library" {
		t.Errorf("scan task = %v", scan)
	}

	// run one by name, case-insensitively, and it runs: its last run moves
	if err := suite.WaitForScan(); err != nil {
		t.Fatal(err)
	}
	was, _ := taskRun(t, acctest.Str(scan["name"]))
	run := suite.Call(t, "task_run", map[string]any{"task": "SCAN MEDIA LIBRARY"})
	if acctest.Str(run["started"]) != acctest.Str(scan["name"]) {
		t.Errorf("task_run started %v, want %v", run["started"], scan["name"])
	}
	if !acctest.EventuallyWithin(scanPatience, func() bool { at, _ := taskRun(t, acctest.Str(scan["name"])); return at.After(was) }) {
		t.Errorf("the scan task's last run is still %v after task_run", was)
	}
	if err := suite.WaitForScan(); err != nil {
		t.Fatal(err)
	}

	// and by id: task_list does not give them, so the id comes from the
	// server's own list. The cache cleanup is quick and touches nothing a
	// test reads
	status, raw := api(t, http.MethodGet, "/ScheduledTasks", "", nil)
	type scheduledTask struct {
		ID   string `json:"Id"`
		Name string
		Key  string
	}
	var listed []scheduledTask
	if status != http.StatusOK || json.Unmarshal(raw, &listed) != nil {
		t.Fatalf("listing the tasks: HTTP %d: %.200s", status, raw)
	}
	i := slices.IndexFunc(listed, func(task scheduledTask) bool { return task.Key == "DeleteCacheFiles" })
	if i < 0 {
		t.Fatalf("no cache cleanup task among %v", listed)
	}
	cleanup := listed[i]
	was, _ = taskRun(t, cleanup.Name)
	if run := suite.Call(t, "task_run", map[string]any{"task": cleanup.ID}); acctest.Str(run["started"]) != cleanup.Name {
		t.Errorf("task_run %s = %v, want %s", cleanup.ID, run, cleanup.Name)
	}
	var task map[string]any
	if !acctest.Eventually(func() bool {
		var at time.Time
		at, task = taskRun(t, cleanup.Name)
		return at.After(was) && acctest.Str(task["state"]) == "Idle"
	}) {
		t.Errorf("%s never ran: %v", cleanup.Name, task)
	}
	if acctest.Str(task["last_status"]) != "Completed" {
		t.Errorf("%s = %v, want it completed", cleanup.Name, task)
	}

	if msg := suite.CallErr(t, "task_run", map[string]any{"task": "no such task"}); !strings.Contains(msg, `no task named "no such task" (have: `) {
		t.Errorf("an unknown task: %s", msg)
	}
}

// task_run takes a task's name or id from task_list, which gives both, and
// every task has an id.
func TestTaskListGivesIDs(t *testing.T) {
	for _, task := range acctest.Rows(t, suite.Call(t, "task_list", nil)["tasks"], "tasks") {
		if acctest.Str(task["id"]) == "" {
			t.Errorf("task %v has no id", task["name"])
		}
	}
}
