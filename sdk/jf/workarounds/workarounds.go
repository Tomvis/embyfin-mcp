// Package workarounds holds the fixes for bugs in the vendored Jellyfin document, one named workaround per bug; pandorest applies them before it imports the
// document.
package workarounds

import (
	pandorest "github.com/katbyte/pandorest/importer/workarounds"
)

// The helpers the workarounds share are pandorest's.
var (
	operation    = pandorest.Operation
	jsonResponse = pandorest.JSONResponse
)

// All is every Jellyfin workaround, in the order they are applied.
var All = []pandorest.Workaround{
	jellyfinCreatePlaylistQuery{},
	jellyfinPluginConfiguration{},
}
