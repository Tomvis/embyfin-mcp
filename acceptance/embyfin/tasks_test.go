//go:build acceptance

package embyfin_test

import (
	"slices"
	"testing"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/sdk/embyfin"
)

// The library scan is one of the server's tasks, found by its name whatever the server calls it, and it can be started; once it has run, no scan
// is running.
func TestTheScanTaskRunAndWaitedOut(t *testing.T) {
	ctx := server(t)

	tasks, err := client.Tasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(tasks, func(task embyfin.Task) bool { return task.Key == "RefreshLibrary" })
	if i < 0 {
		t.Fatalf("no task is the library scan: %+v", tasks)
	}
	scan := tasks[i]

	if got, err := client.FindTask(ctx, scan.Name); err != nil || got.ID != scan.ID {
		t.Errorf("the task named %q = %+v, %v; want %s", scan.Name, got, err, scan.ID)
	}

	started, err := client.RunTask(ctx, scan.Name)
	if err != nil {
		t.Fatal(err)
	}
	if started.ID != scan.ID {
		t.Errorf("the task that was started = %s, want %s", started.ID, scan.ID)
	}

	var running []string
	if !sdkacc.Poll(sdkacc.ScanPatience, func() bool {
		if running, err = client.ScansRunning(ctx); err != nil {
			t.Fatal(err)
		}

		return len(running) == 0
	}) {
		t.Errorf("a scan is still running: %v", running)
	}
	if got := count(ctx, filmLibrary.Name, "Movie"); got != filmLibrary.FilmCount() {
		t.Errorf("the library holds %d films after a scan of everything, want %d", got, filmLibrary.FilmCount())
	}
}
