// Package workarounds holds the fixes for bugs in the vendored Emby document, one named workaround per bug; pandorest applies them before it imports the
// document.
package workarounds

import (
	pandorest "github.com/katbyte/pandorest/importer/workarounds"
)

// The helpers the workarounds share are pandorest's.
type param = pandorest.Param

var (
	operation     = pandorest.Operation
	jsonResponse  = pandorest.JSONResponse
	addParameters = pandorest.AddParameters
	property      = pandorest.Property
)

// All is every Emby workaround, in the order they are applied.
var All = []pandorest.Workaround{
	embyUndeclaredResponses,
	embyUndeclaredPathParameters,
	embyNoContentStatus{},
	embyPlaylistCreateUserID,
	embyLibraryAvailableOptionsQuery,
	embyNextUpLegacy,
	embyPlaystateSeekQuery{},
	embyLibraryDeleteID{},
	embyCommaSeparatedArrays{},
	embyNullResultNoContent{},
	embyOpenAPIDocuments,
	embyRecordingFoldersQueryResult,
	embyParentPathText,
	embyCodecDisplayText{},
	embyPropertyConditionValue{},
	embyUndeclaredQuery{},
	embyToneMapOptions{},
}
