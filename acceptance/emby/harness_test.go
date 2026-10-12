//go:build acceptance

// The harness: .scripts/testenv.sh brings the container up and exports EMBYFIN_BACKEND, EMBYFIN_SERVER, EMBYFIN_TOKEN and EMBYFIN_TEST_*; runSuite
// builds the SDK client, hands the run to lib/sdkacc, which starts the provider proxy the container's HTTPS_PROXY points at, and removes the
// libraries the suite created.
package emby_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/katbyte/pandorest/client"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/lib/testenv"
	embyclient "github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/client"
	"github.com/katbyte/embyfin-mcp/sdk/emby/4.10.0.40/emby"
)

var (
	// embyc is set when the run is against Emby; without it every test skips
	embyc *embyclient.Client

	adminID, aliceID = emby.UserID(sdkacc.AdminID()), emby.UserID(sdkacc.UserID())
	password         = sdkacc.Password()
)

// runSuite connects, runs the tests, removes the libraries they created, and returns the exit code.
func runSuite(m *testing.M) int {
	if !testenv.Configured() || testenv.Backend() != "emby" {
		return m.Run() // every test skips
	}

	var err error
	if embyc, err = embyclient.New(os.Getenv("EMBYFIN_SERVER"), os.Getenv("EMBYFIN_TOKEN")); err != nil {
		fmt.Fprintln(os.Stderr, "sdk client:", err)

		return 1
	}

	return sdkacc.Run(m, "emby", removeLibraries)
}

// removeLibraries deletes every library the suite created, after the last test. The fixtures are created lazily by whichever test runs first and
// shared by the rest, so a t.Cleanup on the creator would pull them out from under the others. EMBYFIN_TEST_KEEP=1 leaves them in place, to look at
// what a run left behind.
func removeLibraries() {
	if os.Getenv("EMBYFIN_TEST_KEEP") != "" {
		return
	}

	ctx := context.Background()
	for _, l := range sdkacc.Libraries() {
		if err := embyDeleteVirtualFolder(ctx, l.Name); err != nil && !client.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "removing %s: %v\n", l.Name, err)
		}
	}
}

// embyLibraryID resolves a library name to its ItemId, "" when absent. Emby
// 4.10 identifies a library by Id and answers 500, "Unrecognized Guid
// format", to a Name.
func embyLibraryID(ctx context.Context, name string) (emby.ItemID, error) {
	folders, err := embyc.LibraryStructure.GetLibraryVirtualFoldersQuery(ctx, emby.GetLibraryVirtualFoldersQueryOptions{})
	if err != nil {
		return "", err
	}
	for _, f := range folders.Answer.Items {
		if f.Name == name {
			return f.ItemId, nil
		}
	}

	return "", nil
}

// embyDeleteVirtualFolder removes a library by name, a no-op when it is
// already gone.
func embyDeleteVirtualFolder(ctx context.Context, name string) error {
	id, err := embyLibraryID(ctx, name)
	if err != nil || id == "" {
		return err
	}
	_, err = embyc.LibraryStructure.PostLibraryVirtualFoldersDelete(ctx, emby.LibraryRemoveVirtualFolder{Id: string(id)})

	return err
}

// embyRetry500 runs a write again, two seconds apart, while the server
// answers 500: Emby saves items with a delete-and-insert inside a
// transaction, and a playlist or collection write that lands while a
// background refresh is rewriting one of its members fails a FOREIGN KEY
// constraint rather than waiting. Any other outcome is returned as is.
func embyRetry500(f func() error) error {
	var err error
	for range 15 {
		if err = f(); client.StatusCode(err) != 500 {
			return err
		}
		time.Sleep(2 * time.Second)
	}

	return err
}

// skipUnlessEmby skips unless the Emby container is up, and returns the test's context.
func skipUnlessEmby(t *testing.T) context.Context {
	t.Helper()

	if embyc == nil {
		t.Skip("not an Emby run; run: eval \"$(EMBYFIN_TEST_BACKEND=emby .scripts/testenv.sh up)\"")
	}

	return t.Context()
}
