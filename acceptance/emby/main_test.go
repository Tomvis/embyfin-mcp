//go:build acceptance

// Package emby_test runs the generated Emby SDK (sdk/emby) against a real Emby in Docker. It tests one thing: that every request the generated options
// build is one the server accepts, and that what it answers decodes into the generated types without losing what it carries. The bespoke tests
// check the answers the tools rely on field by field, and the read sweep calls every other GET and holds its answer to less (see
// lib/sdkacc for what). Nothing here is about the tools or the neutral sdk/embyfin layer; ../tools covers those.
//
// The suite creates its own libraries, scans them, and removes them again when the run ends: "SDK Movies", "SDK Shows" and "SDK Music" over the clean
// fixtures, "SDK Messy Movies" and "SDK Messy Shows" over the messy ones, and "SDK Scratch" and "SDK Bulk Delete" over folders the delete tests lay
// out themselves. Every test skips unless the run is against Emby.
//
//	make testacc-sdk-emby                # the container started and torn down
//
//	eval "$(EMBYFIN_TEST_BACKEND=emby .scripts/testenv.sh up)"  # or drive it by hand
//	go test -tags acceptance ./acceptance/emby/
//	EMBYFIN_TEST_BACKEND=emby .scripts/testenv.sh down
package emby_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(runSuite(m)) }
