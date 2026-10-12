// Package zservices lists the APIs embyfin generates SDKs for, the equivalent
// of Pandora's resource-manager.hcl. Paths are relative to the repository
// root, which is where the make targets run pandorest from.
package zservices

import (
	"slices"
	"time"

	"github.com/katbyte/pandorest/config"
)

// All is every service, in the order the make targets process them.
var All = []config.Service{
	{
		Name:                "emby",
		Package:             "emby",
		Outputs:             map[string]string{"4.10.0.40": "sdk/emby/4.10.0.40"},
		Grouped:             true,
		InlineSingleOptions: true,
		Module:              "github.com/katbyte/embyfin-mcp",
		Packages:            map[string]string{"Package": "packages", "Sync": "syncs", "Image": "images", "Plugin": "plugins", "User": "users"},
		Naming:              config.PathNaming,
		TagSuffix:           "Service",
		ClientService:       "service",
		Auth:                "credential",
		Credential:          "API key",
		Notes:               mediaServerDates,
		Paging:              mediaServerPaging,
		Dates:               mediaServerDateLayouts,
		IDs:                 embyIDs,
		Plain:               embyPlainIDs,
		KeepNull:            append(slices.Clone(mediaServerKeepNull), embyKeepNull...),
	},
	{
		Name:                "jellyfin",
		Package:             "jf",
		Outputs:             map[string]string{"12.1.0": "sdk/jf/12.1.0"},
		Grouped:             true,
		InlineSingleOptions: true,
		Module:              "github.com/katbyte/embyfin-mcp",
		Packages:            map[string]string{"Image": "images", "Plugin": "plugins", "User": "users"},
		Naming:              config.OperationIDNaming,
		ClientService:       "service",
		Auth:                "credential",
		Credential:          "API key",
		Notes:               mediaServerDates,
		Paging:              mediaServerPaging,
		Dates:               mediaServerDateLayouts,
		IDs:                 jellyfinIDs,
		Plain:               jellyfinPlainIDs,
		KeepNull:            append(slices.Clone(mediaServerKeepNull), jellyfinKeepNull...),
	},
	{
		Name:                "tmdb",
		Package:             "tmdb",
		Outputs:             map[string]string{"2026.09.22": "sdk/tmdb/2026.09.22"},
		Grouped:             true,
		InlineSingleOptions: true,
		Module:              "github.com/katbyte/embyfin-mcp",
		Naming:              config.OperationIDNaming,
		ClientService:       "service",
		Auth:                "credential",
		Notes:               tmdbDates,
		Paging:              tmdbPaging,
		Dates:               tmdbDateLayouts,
		Days:                config.Dates{Send: time.DateOnly, KeepUnread: true},
	},
}

// mediaServerKeepNull are the fields both servers leave out for "none" where
// 0 means something: an episode filed with no season or no number of its own
// (Jellyfin reads a file named without SxxEyy that way even in a "Season 01"
// folder, and Emby one at the show's root) is not a special, nor episode 0,
// and an account with no parental limit is not one held to the ratings
// scored 0 (Jellyfin scores G and TV-G 0).
var mediaServerKeepNull = []string{"BaseItemDto.ParentIndexNumber", "BaseItemDto.IndexNumber", "UserPolicy.MaxParentalRating"}

// embyKeepNull are the answers Emby leaves out where false means something: whether a stream's decoder and encoder are hardware is said only for a
// stream that has one, and the session tools pass "not said" on as it is.
var embyKeepNull = []string{"TranscodingInfo.VideoDecoderIsHardware", "TranscodingInfo.VideoEncoderIsHardware"}

// jellyfinKeepNull is the same for Jellyfin: a plugin is called bundled only when the server says it cannot be uninstalled, not when it says nothing.
var jellyfinKeepNull = []string{"PluginInfo.CanUninstall"}

// mediaServerPaging is how both servers' lists page: from StartIndex, Limit at
// a time, answering Items and the TotalRecordCount.
var mediaServerPaging = &config.Paging{Start: "StartIndex", Limit: "Limit", Items: "Items", Total: "TotalRecordCount"}

// tmdbPaging is how TMDB's lists page: by page number from 1, at a size TMDB fixes, answering results and total_results.
var tmdbPaging = &config.Paging{Start: "page", Items: "results", Total: "total_results", ByPage: true}

// mediaServerDateLayouts is how both servers write a date and time: seven fractional digits and a zone, which is also how one is sent, then any
// other RFC 3339 form, then one with no zone at all.
var mediaServerDateLayouts = config.Dates{Send: "2006-01-02T15:04:05.0000000Z07:00", Read: []string{time.RFC3339, "2006-01-02T15:04:05"}, KeepUnread: true}

// tmdbDateLayouts is how TMDB writes a date with a time: three fractional digits and a zone for most, and "2006-01-02 15:04:05 UTC" for when a
// session ends and when a change was made.
var tmdbDateLayouts = config.Dates{Send: "2006-01-02T15:04:05.000Z07:00", Read: []string{time.RFC3339, "2006-01-02 15:04:05 MST"}, KeepUnread: true}

// mediaServerDates is what no document says about its dates.
const mediaServerDates = "# Dates\n\nA date and time is a Time. The servers write RFC 3339 with up to seven fractional digits, sometimes with no\nzone, and a date that reads as none of those is held as zero with its text kept in Unread. One is sent with\nseven fractional digits, and a zero one is not sent."

// tmdbDates is what TMDB's document does not say about its dates.
const tmdbDates = "# Dates\n\nA day is a Day, written 2006-01-02, and a date with a time is a Time. One TMDB does not know, which it writes\nas an empty string, is zero, and one written any other way is held as zero with its text kept in Unread."
