//go:build acceptance

// The harness: .scripts/testenv.sh brings the container up and exports EMBYFIN_BACKEND, EMBYFIN_SERVER, EMBYFIN_TOKEN and EMBYFIN_TEST_*; runSuite
// builds the SDK client, hands the run to lib/sdkacc, which starts the provider proxy the container's HTTPS_PROXY points at, and removes the
// libraries the suite created.
package jf_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/katbyte/pandorest/client"

	"github.com/katbyte/embyfin-mcp/lib/sdkacc"
	"github.com/katbyte/embyfin-mcp/lib/testenv"
	jfclient "github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/client"
	"github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
)

var (
	// jfc is set when the run is against Jellyfin; without it every test skips
	jfc *jfclient.Client

	adminID, aliceID = jf.UserID(sdkacc.AdminID()), jf.UserID(sdkacc.UserID())
	password         = sdkacc.Password()
)

// runSuite connects, runs the tests, removes the libraries they created, and returns the exit code.
func runSuite(m *testing.M) int {
	if !testenv.Configured() || testenv.Backend() != "jellyfin" {
		return m.Run() // every test skips
	}

	var err error
	if jfc, err = jfclient.New(os.Getenv("EMBYFIN_SERVER"), os.Getenv("EMBYFIN_TOKEN")); err != nil {
		fmt.Fprintln(os.Stderr, "sdk client:", err)

		return 1
	}

	return sdkacc.Run(m, "jellyfin", removeLibraries)
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
		if _, err := jfc.LibraryStructure.RemoveVirtualFolder(ctx, jf.RemoveVirtualFolderOptions{Name: l.Name, RefreshLibrary: new(false)}); err != nil && !client.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "removing %s: %v\n", l.Name, err)
		}
	}
}

// skipUnlessJellyfin skips unless the Jellyfin container is up, and returns the test's context.
func skipUnlessJellyfin(t *testing.T) context.Context {
	t.Helper()

	if jfc == nil {
		t.Skip("not a Jellyfin run; run: eval \"$(EMBYFIN_TEST_BACKEND=jellyfin .scripts/testenv.sh up)\"")
	}

	return t.Context()
}
