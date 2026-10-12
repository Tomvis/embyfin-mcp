// Command sdk generates the Emby, Jellyfin and TMDB SDKs (sdk/emby, sdk/jf, sdk/tmdb) from their vendored OpenAPI documents, with
// github.com/katbyte/pandorest: this is embyfin's part of it, the services and the workarounds for their documents. Run from the repository root (the
// make targets do):
//
//	go run ./sdk import
//	go run ./sdk generate -service jellyfin
//	go run ./sdk diff
package main

import (
	"slices"

	"github.com/katbyte/pandorest"

	emby "github.com/katbyte/embyfin-mcp/sdk/emby/workarounds"
	jf "github.com/katbyte/embyfin-mcp/sdk/jf/workarounds"
	tmdb "github.com/katbyte/embyfin-mcp/sdk/tmdb/workarounds"
	"github.com/katbyte/embyfin-mcp/sdk/zservices"
)

// workarounds is every service's workarounds, in the order they are applied.
var workarounds = slices.Concat(emby.All, jf.All, tmdb.All)

func main() {
	pandorest.Main(pandorest.Config{Services: zservices.All, Workarounds: workarounds})
}
