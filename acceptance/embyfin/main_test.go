//go:build acceptance

// Package embyfin_test runs the hand-written sdk/embyfin layer against a real Emby or Jellyfin in Docker. That layer's promise is that a caller gets
// the same answer whichever server is behind it, so every test here is written once and must pass on both: where the servers differ (how a user's
// view is asked for, how a library is listed or named, what keys watch state, who may read a playlist) the test says what the caller should see
// and never which server it is talking to. The tools are not involved; ../tools covers those, and ../emby and ../jf the generated SDKs beneath.
//
// The suite creates its own libraries over the clean fixtures with the servers' internet fetchers off, so nothing here asks a provider anything,
// and removes them when the run ends.
//
//	make testacc-embyfin                  # both backends, containers started and torn down
//	make testacc-embyfin-jellyfin         # one backend
//
//	eval "$(EMBYFIN_TEST_BACKEND=jellyfin .scripts/testenv.sh up)"  # or drive it by hand
//	go test -tags acceptance ./acceptance/embyfin/
//	EMBYFIN_TEST_BACKEND=jellyfin .scripts/testenv.sh down
package embyfin_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(runSuite(m)) }
