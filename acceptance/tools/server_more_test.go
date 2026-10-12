//go:build acceptance

package tools_test

import (
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/katbyte/go-kt/mcp/acctest"

	mediaacc "github.com/katbyte/embyfin-mcp/lib/acceptance"
)

// logStamp is how an entry begins on each server: Emby's own clock with no
// zone, Jellyfin's with one.
func logStamp() *regexp.Regexp {
	if isJellyfin() {
		return regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} [+-]\d{2}:\d{2} [A-Z]{3} `)
	}

	return regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} [A-Z][a-z]+ `)
}

// The server's log is searched, not tailed: its own newest file is read
// whole, and answered as entries, as counts that add up, as a histogram
// that adds up, as gaps, and as the answers it timed.
func TestLogSearch(t *testing.T) {
	out := suite.Call(t, "server_log_search", map[string]any{"limit": 20})
	files := acctest.Rows(t, out["files"], "files")
	lines := acctest.Strs(t, out["lines"], "lines")
	scanned := acctest.Num(t, out["entries_scanned"], "entries_scanned")
	if len(files) != 1 || scanned == 0 || acctest.Num(t, out["matched"], "matched") != scanned || len(lines) != 20 || acctest.Num(t, out["not_shown"], "not_shown") != scanned-20 {
		t.Fatalf("server_log_search = %d files, %d scanned, %v matched, %d lines, %v not shown", len(files), scanned, out["matched"], len(lines), out["not_shown"])
	}
	name := acctest.Str(files[0]["name"])
	if own := strings.HasPrefix(name, "embyserver") || strings.HasPrefix(name, "log_"); !own || acctest.Num(t, files[0]["entries"], "entries") != scanned || acctest.Str(files[0]["first"]) == "" || acctest.Str(files[0]["last"]) == "" {
		t.Errorf("the file read = %v, want the server's own log, read whole", files[0])
	}
	for _, line := range lines {
		if !logStamp().MatchString(line) || strings.Contains(line, "\n") {
			t.Errorf("an entry does not begin with its time and level, or is not on one line: %q", line)
		}
	}
	// the container's clock is UTC, which an Emby file's last line shows
	// against when the server says it was last written; Jellyfin's lines
	// carry the zone, so nothing is judged
	if isJellyfin() {
		if files[0]["utc_offset"] != nil || !strings.Contains(lines[0], " +00:00 ") {
			t.Errorf("Jellyfin's file = %v, its first entry %q", files[0], lines[0])
		}
	} else if acctest.Str(files[0]["utc_offset"]) != "+00:00" || !strings.Contains(acctest.Str(out["note"]), "the server's own clock") {
		t.Errorf("Emby's file = %v, note %q", files[0], out["note"])
	}

	// counts add up, and the message logged most is first
	counts := suite.Call(t, "server_log_search", map[string]any{"mode": "count", "limit": 5})
	total := 0
	levels := mediaacc.ObjectOf(counts["by_level"])
	for _, n := range levels {
		total += acctest.NumOr0(n)
	}
	messages := acctest.Rows(t, counts["messages"], "messages")
	if matched := acctest.Num(t, counts["matched"], "matched"); total != matched || matched < scanned || len(messages) != 5 || acctest.Num(t, messages[0]["count"], "count") < acctest.Num(t, messages[4]["count"], "count") || acctest.Num(t, counts["messages_total"], "messages_total") < 5 {
		t.Errorf("count: levels add to %d of %d matched, messages %v", total, matched, messages)
	}

	// and so does a histogram, here by the hour
	hist := suite.Call(t, "server_log_search", map[string]any{"mode": "histogram", "bucket": "hour"})
	sum := 0
	for _, b := range acctest.Rows(t, hist["buckets"], "buckets") {
		sum += acctest.Num(t, b["count"], "count")
	}
	if sum != acctest.Num(t, hist["matched"], "matched") || acctest.Str(hist["bucket"]) != "1h0m0s" {
		t.Errorf("histogram: buckets add to %d of %v matched, bucket %v", sum, hist["matched"], hist["bucket"])
	}

	// a server that has been set up and scanned was never silent for a
	// year, and was for a hundredth of a second
	if gaps := suite.Call(t, "server_log_search", map[string]any{"mode": "gaps", "seconds": 31_536_000}); acctest.Num(t, gaps["gaps_found"], "gaps_found") != 0 {
		t.Errorf("gaps of a year = %v", gaps)
	}
	gaps := suite.Call(t, "server_log_search", map[string]any{"mode": "gaps", "seconds": 0.01, "limit": 3})
	kept := acctest.Rows(t, gaps["gaps"], "gaps")
	if acctest.Num(t, gaps["gaps_found"], "gaps_found") < 3 || len(kept) != 3 || acctest.Decimal(t, kept[0]["seconds"], "seconds") < 0.01 || acctest.Str(kept[0]["before"]) == "" || acctest.Str(kept[0]["after"]) == "" {
		t.Errorf("gaps of a hundredth of a second = %v", gaps)
	}

	// Emby times each answer it logs - at its default logging the ones to
	// requests that change something, of which this suite has made a great
	// many - and says what it leaves out; Jellyfin logs none unless debug
	// logging is on, and says so
	slow := suite.Call(t, "server_log_search", map[string]any{"mode": "slow", "seconds": 0.001, "limit": 5})
	if isJellyfin() {
		if acctest.Num(t, slow["answers_timed"], "answers_timed") != 0 || !strings.Contains(acctest.Str(slow["note"]), "only when it was slow and debug logging is on") {
			t.Errorf("slow on Jellyfin = %v", slow)
		}
	} else {
		slowest := acctest.Rows(t, slow["slowest"], "slowest")
		if acctest.Num(t, slow["answers_timed"], "answers_timed") == 0 || len(slowest) != 5 || acctest.Num(t, slowest[0]["ms"], "ms") < acctest.Num(t, slowest[4]["ms"], "ms") || !strings.HasPrefix(acctest.Str(slowest[0]["path"]), "/") || strings.ContainsAny(acctest.Str(slowest[0]["path"]), "? ") || acctest.Num(t, slowest[0]["status"], "status") < 100 {
			t.Errorf("slow on Emby = %v", slow)
		}
		// what it leaves out, it says, and it is so: server_info has been asked for all through this suite, a request that
		// only reads, and at its default logging Emby has logged none of them
		reads := suite.Call(t, "server_log_search", map[string]any{"include": []any{`http/1\.1 GET \S*/System/Info\b`}, "mode": "count"})
		if !strings.Contains(acctest.Str(slow["note"]), "Emby logs a request that only reads (a GET) with debug logging on alone") || acctest.Num(t, reads["matched"], "matched") != 0 {
			t.Errorf("slow on Emby says %q, and its log holds %v requests for /System/Info", slow["note"], reads["matched"])
		}
	}
}

// A search is narrowed by time, text and level, and finds what the server
// logged about a thing just done: here, a library scan it was asked for.
func TestLogSearchFindsWhatJustHappened(t *testing.T) {
	suite.Call(t, "library_scan", nil)
	if err := suite.WaitForExpectedScan(isJellyfin()); err != nil {
		t.Fatal(err)
	}
	// both servers log the scan task as it ends, by its name
	var found []string
	if !acctest.Eventually(func() bool {
		out := suite.Call(t, "server_log_search", map[string]any{"last": "10m", "include": []any{"scan media library"}, "newest": true, "limit": 5})
		found = acctest.Texts(out["lines"])

		return slices.ContainsFunc(found, func(l string) bool { return strings.Contains(l, "Completed after") })
	}) {
		t.Fatalf("the scan just run is not in the last ten minutes of the log: %q", found)
	}
	// narrowed to what it is not, nothing matches
	none := suite.Call(t, "server_log_search", map[string]any{"last": "10m", "include": []any{"scan media library"}, "exclude": []any{"scan"}})
	if acctest.Num(t, none["matched"], "matched") != 0 || none["lines"] != nil {
		t.Errorf("included and excluded at once = %v", none)
	}
	// a window is given by the log's own clock, as its entries are stamped:
	// from the last entry's own time on, there is at least that entry
	last := acctest.Str(acctest.Rows(t, suite.Call(t, "server_log_search", map[string]any{"limit": 1, "newest": true})["files"], "files")[0]["last"])
	clock := last[:len("2026-01-05 08:00:00")]
	from := suite.Call(t, "server_log_search", map[string]any{"since": clock, "mode": "count"})
	if acctest.Num(t, from["matched"], "matched") < 1 || acctest.Num(t, from["matched"], "matched") >= acctest.Num(t, from["entries_scanned"], "entries_scanned") {
		t.Errorf("from %s on: %v matched of %v scanned", clock, from["matched"], from["entries_scanned"])
	}
	if errs := suite.Call(t, "server_log_search", map[string]any{"level": "fatal", "mode": "count"}); acctest.Num(t, errs["matched"], "matched") != 0 {
		t.Errorf("a test server logged something fatal: %v", errs)
	}

	// Emby's lines carry no zone, so a time with one is refused there;
	// Jellyfin's do, and it is held against them as an instant
	zoned := map[string]any{"since": strings.Replace(clock, " ", "T", 1) + "Z", "mode": "count"}
	if isJellyfin() {
		if out := suite.Call(t, "server_log_search", zoned); acctest.Num(t, out["matched"], "matched") != acctest.Num(t, from["matched"], "matched") {
			t.Errorf("the same instant with its zone matched %v, without %v", out["matched"], from["matched"])
		}
	} else if msg := suite.CallErr(t, "server_log_search", zoned); !strings.Contains(msg, "cannot be held against Emby's log lines, which carry none") {
		t.Errorf("a zoned time on Emby = %q", msg)
	}
	if msg := suite.CallErr(t, "server_log_search", map[string]any{"mode": "tail"}); !strings.Contains(msg, "is not one of lines, count, histogram, gaps, slow") {
		t.Errorf("an unknown mode = %q", msg)
	}
	if msg := suite.CallErr(t, "server_log_search", map[string]any{"files": []any{"no-such.log"}}); !strings.Contains(msg, `no log file named "no-such.log"`) {
		t.Errorf("an unknown file = %q", msg)
	}

	// Emby hands its log out with each client replaced by a placeholder,
	// and as it is on disk when asked: the suite's own address is then named
	if !isJellyfin() {
		answered := []any{`Response \d+ to `}
		blank := acctest.Texts(suite.Call(t, "server_log_search", map[string]any{"include": answered, "limit": 3})["lines"])
		raw := acctest.Texts(suite.Call(t, "server_log_search", map[string]any{"include": answered, "limit": 3, "raw": true})["lines"])
		address := regexp.MustCompile(`Response \d+ to \d+\.\d+\.\d+\.\d+\. Time: `)
		if len(blank) != 3 || len(raw) != 3 || !strings.Contains(blank[0], " to host") || address.MatchString(blank[0]) || !address.MatchString(raw[0]) {
			t.Errorf("anonymised %q\nas on disk %q", blank, raw)
		}
	}
}

// A task is read whole, and what starts it is changed and read back from
// the server, in words both ways.
func TestTaskGetAndEdit(t *testing.T) {
	var scan map[string]any
	for _, task := range acctest.Rows(t, suite.Call(t, "task_list", nil)["tasks"], "tasks") {
		if strings.EqualFold(acctest.Str(task["name"]), "scan media library") {
			scan = task
		}
	}
	if scan == nil {
		t.Fatal("no scan task")
	}
	before := acctest.Texts(scan["triggers"])
	// both servers scan every twelve hours unless told otherwise
	if !slices.Equal(before, []string{"every 12h"}) {
		t.Errorf("the scan's triggers = %q, want every 12h", before)
	}

	got := suite.Call(t, "task_get", map[string]any{"task": acctest.Str(scan["id"])})
	if acctest.Str(got["key"]) != "RefreshLibrary" || acctest.Str(got["name"]) != acctest.Str(scan["name"]) || acctest.Str(got["description"]) == "" || !slices.Equal(acctest.Texts(got["triggers"]), before) || acctest.Str(got["last_status"]) == "" || acctest.Str(got["last_took"]) == "" || got["error_detail"] != nil {
		t.Errorf("task_get = %v", got)
	}

	back := make([]any, 0, len(before))
	for _, trigger := range before {
		back = append(back, trigger)
	}
	suite.PutBack(t, "task_edit", map[string]any{"task": acctest.Str(scan["id"]), "triggers": back})
	want := []string{"daily 06:00", "weekly Sunday 04:15, for at most 2h", "every 12h", "at startup"}
	edited := suite.Call(t, "task_edit", map[string]any{"task": "Scan Media Library", "triggers": []any{"daily 6:00", "weekly sunday 04:15, for at most 2h", "every 12h", "at startup"}})
	if !slices.Equal(acctest.Texts(edited["before"]), before) || !slices.Equal(acctest.Texts(edited["after"]), want) {
		t.Errorf("task_edit = %v", edited)
	}
	if now := acctest.Texts(suite.Call(t, "task_get", map[string]any{"task": acctest.Str(scan["id"])})["triggers"]); !slices.Equal(now, want) {
		t.Errorf("the scan's triggers read back = %q, want %q", now, want)
	}
	// none at all, and the task still runs when asked
	if none := suite.Call(t, "task_edit", map[string]any{"task": acctest.Str(scan["id"]), "triggers": []any{}}); len(acctest.Texts(none["after"])) != 0 || !strings.Contains(acctest.Str(none["note"]), "runs only when task_run asks") {
		t.Errorf("no triggers = %v", none)
	}
	if msg := suite.CallErr(t, "task_edit", map[string]any{"task": acctest.Str(scan["id"]), "triggers": []any{"hourly"}}); !strings.Contains(msg, `trigger "hourly" is not one this reads`) {
		t.Errorf("a trigger that is no trigger = %q", msg)
	}
	// Jellyfin has no trigger on a system event, and says so before asking
	if isJellyfin() {
		if msg := suite.CallErr(t, "task_edit", map[string]any{"task": acctest.Str(scan["id"]), "triggers": []any{"on wakefromsleep"}}); !strings.Contains(msg, "is Emby's alone") {
			t.Errorf("a system event on Jellyfin = %q", msg)
		}
	}
}

// A task that is not running is asked nothing; one that is, is stopped.
func TestTaskStop(t *testing.T) {
	idle := suite.Call(t, "task_stop", map[string]any{"task": "Scan Media Library"})
	if !acctest.IsBool(idle["was_running"], false) || !strings.Contains(acctest.Str(idle["note"]), "nothing was asked of the server") {
		// a scan another test left running is not this test's to judge
		t.Logf("the scan was running before the test asked: %v", idle)
	}

	// a scan slowed by files that take the server a while to read
	slowScan(t)
	suite.Call(t, "task_run", map[string]any{"task": "Scan Media Library"})
	// while it runs, the server's health names it among the running, with how far it has got
	var running []map[string]any
	if !acctest.Eventually(func() bool {
		running = acctest.RowsOf(acctest.Object(t, suite.Call(t, "server_health", map[string]any{"last": "1m"})["tasks"], "tasks")["running"])

		return slices.ContainsFunc(running, func(task map[string]any) bool {
			return strings.EqualFold(acctest.Str(task["name"]), "Scan media library") && task["progress"] != nil
		})
	}) {
		t.Errorf("the scan was never among the running in server_health: %v", running)
	}
	var stopped map[string]any
	if !acctest.Eventually(func() bool {
		stopped = suite.Call(t, "task_stop", map[string]any{"task": "Scan Media Library"})

		return acctest.BoolOf(stopped["was_running"])
	}) {
		t.Fatalf("the scan was never seen running to be stopped: %v", stopped)
	}
	// it stops, at once or once the step it is in ends, and its run is
	// then recorded as cut short
	if !acctest.Eventually(func() bool {
		return acctest.Str(suite.Call(t, "task_get", map[string]any{"task": "Scan Media Library"})["state"]) == "Idle"
	}) {
		t.Fatalf("the scan asked to stop is still %v", suite.Call(t, "task_get", map[string]any{"task": "Scan Media Library"})["state"])
	}
	// both servers record a scan stopped this way as cancelled
	after := suite.Call(t, "task_get", map[string]any{"task": "Scan Media Library"})
	if state := acctest.Str(stopped["state"]); (state != "Idle" && state != "Cancelling") || acctest.Str(after["last_status"]) != "Cancelled" {
		t.Errorf("task_stop left the scan %s, and its run recorded as %v", state, after["last_status"])
	}
	// and the library is scanned whole again, for the tests after
	if err := suite.WaitForScan(); err != nil {
		t.Fatal(err)
	}
	suite.Call(t, "library_scan", nil)
	if err := suite.WaitForExpectedScan(isJellyfin()); err != nil {
		t.Fatal(err)
	}
}

// While something plays, the session says how it reaches the device, and
// with details what the file is.
func TestSessionDetails(t *testing.T) {
	device, token := signInPlayer(t)
	dune := findItem(t, "Movies", "Movie", "Dune: Part Two")
	p := startPlaying(t, token, dune)
	half := int64(5010) * 10_000_000
	p.report("/Sessions/Playing/Progress", half, false)

	var row map[string]any
	if !acctest.Eventually(func() bool {
		for _, s := range acctest.Rows(t, suite.Call(t, "session_list", map[string]any{"details": true})["sessions"], "sessions") {
			if acctest.Str(s["device"]) == device {
				row = s
			}
		}

		return row != nil && acctest.Str(row["now_playing_id"]) == dune && row["progress"] != nil
	}) {
		t.Fatalf("the player is not shown playing: %v", row)
	}
	details := mediaacc.ObjectOf(row["details"])
	source := mediaacc.ObjectOf(details["source"])
	// the client said it plays the file as it is, half way through its 167
	// minutes; the server made no stream for it
	if acctest.Str(row["play_method"]) != "DirectPlay" || acctest.Decimal(t, row["progress"], "progress") != 50 || row["transcoding"] != nil || details["transcode"] != nil || acctest.Str(row["last_activity"]) == "" {
		t.Errorf("the session = %v", row)
	}
	if acctest.Str(details["app_version"]) == "" || acctest.Str(details["remote_address"]) == "" {
		t.Errorf("the device = %v", details)
	}
	if acctest.Str(source["container"]) == "" || acctest.Str(source["video_codec"]) == "" || acctest.Num(t, source["width"], "width") == 0 || acctest.Num(t, source["height"], "height") == 0 || len(acctest.RowsOf(source["audio"])) == 0 {
		t.Errorf("the file being played = %v", source)
	}
	// without details the row says how it plays and nothing of the file
	short := sessionOn(t, device)
	if acctest.Str(short["play_method"]) != "DirectPlay" || short["details"] != nil {
		t.Errorf("the short row = %v", short)
	}
	p.stop(half)
}

// The server's own settings are read in groups on both servers, and on
// Jellyfin the ones that tune its work are changed and read back, with
// every other setting left as it was.
func TestServerConfig(t *testing.T) {
	out := suite.Call(t, "server_config", nil)
	groups := acctest.Object(t, out["groups"], "groups")
	flat := func(groups map[string]any) map[string]any {
		all := map[string]any{}
		for name, group := range groups {
			maps.Copy(all, acctest.Object(t, group, name))
		}

		return all
	}
	before := flat(groups)
	// both servers say how long they keep logs. Jellyfin has a name for
	// itself among its settings, empty until one is given; Emby has none
	// there until one is given, and says where its cache is or what port it
	// listens on
	known := before["ServerName"] != nil
	if !isJellyfin() {
		known = before["CachePath"] != nil || before["HttpServerPortNumber"] != nil
	}

	if acctest.Str(out["backend"]) != string(backend) || len(before) < 40 || acctest.Object(t, groups["logs"], "logs")["LogFileRetentionDays"] == nil || !known {
		t.Fatalf("server_config = %d settings in %d groups: %v", len(before), len(groups), groups)
	}
	if only := acctest.Object(t, suite.Call(t, "server_config", map[string]any{"groups": []any{"logs"}})["groups"], "groups"); len(only) != 1 || only["logs"] == nil {
		t.Errorf("one group asked for = %v", only)
	}
	if msg := suite.CallErr(t, "server_config", map[string]any{"groups": []any{"nope"}}); !strings.Contains(msg, `no group "nope"`) {
		t.Errorf("an unknown group = %q", msg)
	}

	threads := map[string]any{"set": map[string]any{"TrickplayOptions.ProcessThreads": 3, "LibraryScanFanoutConcurrency": 2}}
	if !isJellyfin() {
		// Emby's are read, and none is on the list to change
		if len(acctest.Texts(out["editable"])) != 0 || !strings.Contains(acctest.Str(out["note"]), "none of Emby's server settings is changed") {
			t.Errorf("what can be changed on Emby = %v, note %q", out["editable"], out["note"])
		}
		if msg := suite.CallErr(t, "server_config_edit", threads); !strings.Contains(msg, "none of Emby's server settings is changed by this tool") {
			t.Errorf("a change on Emby = %q", msg)
		}
		if after := flat(acctest.Object(t, suite.Call(t, "server_config", nil)["groups"], "groups")); !reflect.DeepEqual(before, after) {
			t.Errorf("a refused change left Emby's settings different:\nbefore %v\nafter  %v", before, after)
		}

		return
	}

	if editable := acctest.Texts(out["editable"]); !slices.Contains(editable, "TrickplayOptions.ProcessThreads") || !slices.Contains(editable, "LibraryScanFanoutConcurrency") || slices.Contains(editable, "CachePath") {
		t.Errorf("what can be changed on Jellyfin = %v", editable)
	}
	// put back to what a server just set up has, whatever happens below
	suite.PutBack(t, "server_config_edit", map[string]any{"set": map[string]any{
		"TrickplayOptions.ProcessThreads": before["TrickplayOptions.ProcessThreads"], "LibraryScanFanoutConcurrency": before["LibraryScanFanoutConcurrency"],
	}})
	edited := suite.Call(t, "server_config_edit", threads)
	changed := acctest.Rows(t, edited["changed"], "changed")
	if len(changed) != 2 || acctest.Str(changed[0]["setting"]) != "LibraryScanFanoutConcurrency" || acctest.Num(t, changed[0]["after"], "after") != 2 || acctest.Str(changed[1]["setting"]) != "TrickplayOptions.ProcessThreads" || acctest.Num(t, changed[1]["after"], "after") != 3 || len(acctest.Texts(edited["unchanged"])) != 0 {
		t.Errorf("server_config_edit = %v", edited)
	}
	// the whole document went back to the server, and only the two changed
	after := flat(acctest.Object(t, suite.Call(t, "server_config", nil)["groups"], "groups"))
	var differ []string
	for setting := range before {
		if !reflect.DeepEqual(before[setting], after[setting]) {
			differ = append(differ, setting)
		}
	}
	slices.Sort(differ)
	if len(after) != len(before) || !slices.Equal(differ, []string{"LibraryScanFanoutConcurrency", "TrickplayOptions.ProcessThreads"}) {
		t.Errorf("settings that differ after the change = %v, of %d before and %d after", differ, len(before), len(after))
	}
	// asked again, nothing is changed and nothing is said to have been
	if again := suite.Call(t, "server_config_edit", threads); len(acctest.RowsOf(again["changed"])) != 0 || len(acctest.Texts(again["unchanged"])) != 2 {
		t.Errorf("the same change again = %v", again)
	}
	for want, set := range map[string]map[string]any{
		"CachePath is not a setting this tool changes": {"CachePath": "/elsewhere"},
		"takes 2 to 31, not 40":                        {"TrickplayOptions.Qscale": 40},
		"is on or off: give true or false":             {"TrickplayOptions.EnableHwEncoding": "yes"},
	} {
		if msg := suite.CallErr(t, "server_config_edit", map[string]any{"set": set}); !strings.Contains(msg, want) || !strings.Contains(msg, "Nothing was changed") {
			t.Errorf("%v refused with %q, want %q", set, msg, want)
		}
	}
	if now := flat(acctest.Object(t, suite.Call(t, "server_config", nil)["groups"], "groups")); !reflect.DeepEqual(now, after) {
		t.Error("a refused change left the settings different")
	}
}

// What is installed into the server is listed by name. Both servers come
// with plugins of their own; Jellyfin says of each whether it is running
// and that it came with the server, and Emby says neither.
func TestServerPlugins(t *testing.T) {
	out := suite.Call(t, "server_plugins", nil)
	rows := acctest.Rows(t, out["plugins"], "plugins")
	if acctest.Str(out["backend"]) != string(backend) || len(rows) == 0 || acctest.Num(t, out["count"], "count") != len(rows) {
		t.Fatalf("server_plugins = %v", out)
	}
	names := make([]string, 0, len(rows))
	var notRunning, bundled int
	for _, p := range rows {
		names = append(names, strings.ToLower(acctest.Str(p["name"])))
		if acctest.Str(p["name"]) == "" || acctest.Str(p["version"]) == "" {
			t.Errorf("a plugin with no name or no version: %v", p)
		}
		if status := acctest.Str(p["status"]); isJellyfin() == (status == "") {
			t.Errorf("status on %s = %q: %v", backend, status, p)
		} else if status != "" && status != "Active" {
			notRunning++
		}
		if acctest.BoolOf(p["bundled"]) {
			bundled++
		}
	}
	if !slices.IsSorted(names) {
		t.Errorf("plugins are not in order of name: %v", names)
	}
	// the ones a server just set up has all came with it, which Jellyfin says and Emby does not
	if isJellyfin() != (bundled > 0) {
		t.Errorf("%d of %d plugins said to have come with %s", bundled, len(rows), backend)
	}
	if note := acctest.Str(out["note"]); (notRunning > 0) != strings.HasPrefix(note, "not running: ") {
		t.Errorf("%d plugins not running, and the note is %q", notRunning, note)
	}
}

// A library's settings are read side by side. The suite's own libraries
// differ where it made them differ: Movies fetches metadata and Messy
// Movies was made to fetch none.
func TestLibraryOptions(t *testing.T) {
	out := suite.Call(t, "library_options", nil)
	libraryRows := acctest.Rows(t, out["libraries"], "libraries")
	names := make([]string, 0, len(libraryRows))
	for _, l := range libraryRows {
		names = append(names, acctest.Str(l["name"]))
		if acctest.Str(l["id"]) == "" || acctest.Str(l["type"]) == "" {
			t.Errorf("a library with no id or no kind: %v", l)
		}
	}
	for _, want := range []string{"Movies", "Messy Movies", "Shows", "Messy Shows", "Music"} {
		if !slices.Contains(names, want) {
			t.Errorf("libraries compared = %v, want %s among them", names, want)
		}
	}
	rows := map[string]map[string]any{}
	for _, row := range acctest.Rows(t, out["options"], "options") {
		rows[acctest.Str(row["option"])] = row
		// a row is one value, or each library's own
		if (row["value"] == nil) == (row["values"] == nil) {
			t.Errorf("a row with both or neither of value and values: %v", row)
		}
	}
	if acctest.Str(out["backend"]) != string(backend) || len(rows) < 30 || acctest.Num(t, out["alike"], "alike")+acctest.Num(t, out["differing"], "differing") != len(rows) {
		t.Fatalf("library_options = %d rows, %v alike and %v differing", len(rows), out["alike"], out["differing"])
	}
	// both servers keep these for a library, and each the images it makes from a video its own way
	own := "ThumbnailImagesIntervalSeconds"
	if isJellyfin() {
		own = "EnableTrickplayImageExtraction"
	}
	for _, want := range []string{"SaveLocalMetadata", "EnableRealtimeMonitor", "EnableChapterImageExtraction", own} {
		if rows[want] == nil {
			t.Errorf("no row for %s", want)
		}
	}
	// a library's folders and kind are not settings
	for _, never := range []string{"PathInfos", "TypeOptions", "ContentType"} {
		if rows[never] != nil {
			t.Errorf("%s is given as a setting: %v", never, rows[never])
		}
	}

	// held against Movies, Messy Movies differs in what fetches a film's metadata: it was made with none, which Emby
	// lists as an empty list beside Movies' own and Jellyfin as an empty list where Movies has no list at all
	pair := suite.Call(t, "library_options", map[string]any{"libraries": []any{"messy movies", "Movies"}, "differ_from": "Movies", "differing": true})
	differ := acctest.Object(t, pair["differ_from"], "differ_from")
	others := acctest.Rows(t, differ["others"], "others")
	if acctest.Str(differ["library"]) != "Movies" || len(others) != 1 || acctest.Str(others[0]["library"]) != "Messy Movies" || !slices.Contains(acctest.Texts(others[0]["differs"]), "Movie.MetadataFetchers") {
		t.Fatalf("Messy Movies held against Movies = %v", differ)
	}
	var fetchers map[string]any
	for _, row := range acctest.Rows(t, pair["options"], "options") {
		if row["values"] == nil {
			t.Errorf("asked for the differing alone, and given %v", row)
		}
		if acctest.Str(row["option"]) == "Movie.MetadataFetchers" {
			fetchers = row
		}
	}
	values := acctest.Object(t, fetchers["values"], "values")
	if none, ok := values["Messy Movies"]; !ok || len(acctest.Texts(none)) != 0 {
		t.Errorf("Messy Movies' fetchers = %v", fetchers)
	}
	if isJellyfin() {
		if !slices.Equal(acctest.Texts(fetchers["unset"]), []string{"Movies"}) || values["Movies"] != nil {
			t.Errorf("on Jellyfin Movies lists no fetchers and uses the server's own: %v", fetchers)
		}
	} else if len(acctest.Texts(values["Movies"])) == 0 || fetchers["unset"] != nil {
		t.Errorf("on Emby Movies lists its fetchers: %v", fetchers)
	}
	// a library of shows is not held against one of films on a film's settings
	for _, other := range acctest.Rows(t, acctest.Object(t, suite.Call(t, "library_options", map[string]any{"differ_from": "Movies"})["differ_from"], "differ_from")["others"], "others") {
		if acctest.Str(other["library"]) == "Shows" && slices.ContainsFunc(acctest.Texts(other["differs"]), func(o string) bool { return strings.HasPrefix(o, "Movie.") || strings.HasPrefix(o, "Series.") }) {
			t.Errorf("Shows held against Movies on a kind's own settings: %v", other)
		}
	}

	// narrowed by a part of a setting's name, whatever its case
	only := acctest.Rows(t, suite.Call(t, "library_options", map[string]any{"options": []any{"savelocal"}})["options"], "options")
	if len(only) == 0 || slices.ContainsFunc(only, func(row map[string]any) bool {
		return !strings.Contains(strings.ToLower(acctest.Str(row["option"])), "savelocal")
	}) {
		t.Errorf("settings named savelocal = %v", only)
	}
	if msg := suite.CallErr(t, "library_options", map[string]any{"libraries": []any{"no such library"}}); !strings.Contains(msg, `no library named "no such library" or with that id`) {
		t.Errorf("an unknown library = %q", msg)
	}
}

// One call says how the server is doing: who it is, what its tasks and
// devices are at, and what the end of its log shows. While something plays
// it is among the playing. (TestTaskStop sees a running task in it.)
func TestServerHealth(t *testing.T) {
	device, token := signInPlayer(t)
	dune := findItem(t, "Movies", "Movie", "Dune: Part Two")
	p := startPlaying(t, token, dune)
	half := int64(5010) * 10_000_000
	p.report("/Sessions/Playing/Progress", half, false)

	var out, playing map[string]any
	if !acctest.Eventually(func() bool {
		out = suite.Call(t, "server_health", nil)
		for _, s := range acctest.RowsOf(acctest.Object(t, out["sessions"], "sessions")["playing"]) {
			if acctest.Str(s["device"]) == device {
				playing = s
			}
		}

		return playing != nil && playing["progress"] != nil
	}) {
		t.Fatalf("the player is not among the playing: %v", out["sessions"])
	}
	p.stop(half)
	sessions := acctest.Object(t, out["sessions"], "sessions")
	if acctest.Str(playing["now_playing"]) != "Dune: Part Two" || acctest.Str(playing["play_method"]) != "DirectPlay" || acctest.Decimal(t, playing["progress"], "progress") != 50 || playing["transcoding"] != nil || acctest.Num(t, sessions["connected"], "connected") < 1 || acctest.Num(t, sessions["transcoding"], "transcoding") != 0 {
		t.Errorf("the session = %v of %v", playing, sessions)
	}

	info := suite.Call(t, "server_info", nil)
	if acctest.Str(out["backend"]) != string(backend) || out["server_name"] != info["server_name"] || out["server_version"] != info["server_version"] || out["pending_restart"] != info["pending_restart"] {
		t.Errorf("the server = %v, and server_info says %v", out, info)
	}

	// the tasks said to have failed are the ones task_list says failed
	var failed []string
	for _, task := range acctest.Rows(t, suite.Call(t, "task_list", nil)["tasks"], "tasks") {
		if status := acctest.Str(task["last_status"]); acctest.Str(task["state"]) == "Idle" && (status == "Failed" || status == "Aborted") {
			failed = append(failed, acctest.Str(task["name"]))
		}
	}
	tasks := acctest.Object(t, out["tasks"], "tasks")
	rows := acctest.RowsOf(tasks["failed"])
	said := make([]string, 0, len(rows))
	for _, task := range rows {
		said = append(said, acctest.Str(task["name"]))
	}
	slices.Sort(failed)
	slices.Sort(said)
	if !slices.Equal(said, failed) || tasks["running"] == nil {
		t.Errorf("tasks = %v, and task_list has these failed: %v", tasks, failed)
	}

	// the log's last stretch: read, with entries that add up by level, and counted back from a last entry that is given
	log := acctest.Object(t, out["log"], "log")
	entries, byLevel := acctest.Num(t, log["entries"], "entries"), 0
	for level := range acctest.Object(t, log["by_level"], "by_level") {
		byLevel += acctest.Num(t, acctest.Object(t, log["by_level"], "by_level")[level], level)
	}
	if log["error"] != nil || len(acctest.Texts(log["files"])) == 0 || entries < 1 || byLevel != entries || acctest.Str(log["from"]) == "" || acctest.Str(log["to"]) == "" || acctest.Str(log["quiet_for"]) == "" {
		t.Errorf("the log read = %v", log)
	}
	// Emby times the answers to requests that change something, and the player above just made some; Jellyfin times none unless set to
	if timed := acctest.Num(t, log["answers_timed"], "answers_timed"); isJellyfin() == (timed > 0) {
		t.Errorf("%d answers timed on %s", timed, backend)
	}
	if summary := acctest.Str(out["summary"]); !strings.Contains(summary, "1 playing of ") || !strings.Contains(summary, "in the last 15m0s of the log: ") {
		t.Errorf("summary = %q", summary)
	}

	// asked for less of the log there is no more in it, each list is cut at limit, and no silence in fifteen minutes is a day long
	narrow := acctest.Object(t, suite.Call(t, "server_health", map[string]any{"last": "1s", "limit": 1})["log"], "log")
	wide := acctest.Object(t, suite.Call(t, "server_health", map[string]any{"last": "1h", "gap_seconds": 86400, "slow_seconds": 0.001, "limit": 1})["log"], "log")
	if n, w := acctest.Num(t, narrow["entries"], "entries"), acctest.Num(t, wide["entries"], "entries"); n < 1 || n > w || w < entries || len(acctest.RowsOf(narrow["errors"])) > 1 || len(acctest.RowsOf(wide["slowest"])) > 1 || acctest.Num(t, wide["gaps_found"], "gaps_found") != 0 {
		t.Errorf("the last second = %v\nthe last hour = %v", narrow, wide)
	}
	// every answer Emby timed took a thousandth of a second or more, or near enough: the slow ones are counted whole, past the one row given
	if !isJellyfin() && (acctest.Num(t, wide["slow_found"], "slow_found") < 1 || len(acctest.RowsOf(wide["slowest"])) != 1) {
		t.Errorf("answers of a millisecond or more in the last hour = %v", wide)
	}
	if msg := suite.CallErr(t, "server_health", map[string]any{"last": "2d"}); !strings.Contains(msg, "is not a length of time") {
		t.Errorf("a stretch in days = %q", msg)
	}
}
