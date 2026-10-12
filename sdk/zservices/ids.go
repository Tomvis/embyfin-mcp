package zservices

import "github.com/katbyte/pandorest/config"

// embyIDs are the kinds of id the Emby SDK tells apart, so that a user's id where an item's goes does not compile. Each is the kind of the record Of
// names; a playlist entry's has no record of its own, so Base says it is text. Emby's document calls most of an item's ids plain Id in the path, so In
// lists every such place, Joined names the parameters that take several ids in one comma-joined value, and Except lists the few places where the
// document declares one of the names as a number.
var embyIDs = []config.ID{
	{
		Name: "ItemID", Of: []string{"BaseItemDto"}, Names: []string{"AlbumId", "ExcludeItemIds", "ItemId", "ItemIds", "ParentBackdropItemId", "ParentId", "ParentLogoItemId", "ParentPrimaryImageItemId", "ParentThumbItemId", "PrimaryImageItemId", "SeasonId", "SeriesId", "StartItemId"}, Joined: []string{"ExcludeItemIds", "Ids", "ItemIds"},
		In: []string{
			"DELETE /Collections/{Id}/Items Id",
			"DELETE /Items/{Id} Id",
			"DELETE /Items/{Id}/Images/{Type} Id",
			"DELETE /Items/{Id}/Images/{Type}/{Index} Id",
			"DELETE /Items/{Id}/Subtitles/{Index} Id",
			"DELETE /Playlists/{Id}/Items Id",
			"DELETE /Users/{UserId}/FavoriteItems/{Id} Id",
			"DELETE /Users/{UserId}/PlayedItems/{Id} Id",
			"DELETE /Users/{UserId}/PlayingItems/{Id} Id",
			"DELETE /Videos/{Id}/AlternateSources Id",
			"DELETE /Videos/{Id}/Subtitles/{Index} Id",
			"GET /Albums/{Id}/InstantMix Id",
			"GET /Albums/{Id}/Similar Id",
			"GET /Artists/{Id}/Similar Id",
			"GET /Audio/{Id}/hls/{PlaylistId}/{SegmentId}.{SegmentContainer} Id",
			"GET /Audio/{Id}/hls1/{PlaylistId}/{SegmentId}.{SegmentContainer} Id",
			"GET /Audio/{Id}/live.m3u8 Id",
			"GET /Audio/{Id}/main.m3u8 Id",
			"GET /Audio/{Id}/master.m3u8 Id",
			"GET /Audio/{Id}/stream Id",
			"GET /Audio/{Id}/stream.{Container} Id",
			"GET /Audio/{Id}/universal Id",
			"GET /Audio/{Id}/universal.{Container} Id",
			"GET /Audio/{Id}/{StreamFileName} Id",
			"GET /Collections/{Id}/Missing Id",
			"GET /Collections/{Id}/ProviderItems Id",
			"GET /Games/{Id}/Similar Id",
			"GET /Items/{Id}/CriticReviews Id",
			"GET /Items/{Id}/DeleteInfo Id",
			"GET /Items/{Id}/Download Id",
			"GET /Items/{Id}/ExternalIdInfos Id",
			"GET /Items/{Id}/File Id",
			"GET /Items/{Id}/Images Id",
			"GET /Items/{Id}/Images/{Type} Id",
			"GET /Items/{Id}/Images/{Type}/{Index} Id",
			"GET /Items/{Id}/Images/{Type}/{Index}/{Tag}/{Format}/{MaxWidth}/{MaxHeight}/{PercentPlayed}/{UnPlayedCount} Id",
			"GET /Items/{Id}/InstantMix Id",
			"GET /Items/{Id}/PlaybackInfo Id",
			"GET /Items/{Id}/RemoteImages Id",
			"GET /Items/{Id}/RemoteImages/Providers Id",
			"GET /Items/{Id}/RemoteSearch/Subtitles/{Language} Id",
			"GET /Items/{Id}/Similar Id",
			"GET /Items/{Id}/ThemeMedia Id",
			"GET /Items/{Id}/ThemeSongs Id",
			"GET /Items/{Id}/ThemeVideos Id",
			"GET /Items/{Id}/ThumbnailSet Id",
			"GET /Items/{Id}/{MediaSourceId}/Subtitles/{Index}/Stream.{Format} Id",
			"GET /Items/{Id}/{MediaSourceId}/Subtitles/{Index}/{StartPositionTicks}/Stream.{Format} Id",
			"GET /Movies/{Id}/Similar Id",
			"GET /Persons/{Id}/Credits Id",
			"GET /Playlists/{Id}/AddToPlaylistInfo Id",
			"GET /Playlists/{Id}/InstantMix Id",
			"GET /Playlists/{Id}/Items Id",
			"GET /Shows/{Id}/Episodes Id",
			"GET /Shows/{Id}/Seasons Id",
			"GET /Shows/{Id}/Similar Id",
			"GET /Songs/{Id}/InstantMix Id",
			"GET /Trailers/{Id}/Similar Id",
			"GET /Videos/{Id}/AdditionalParts Id",
			"GET /Videos/{Id}/hls/{PlaylistId}/{SegmentId}.{SegmentContainer} Id",
			"GET /Videos/{Id}/hls1/{PlaylistId}/{SegmentId}.{SegmentContainer} Id",
			"GET /Videos/{Id}/index.bif Id",
			"GET /Videos/{Id}/live.m3u8 Id",
			"GET /Videos/{Id}/live_subtitles.m3u8 Id",
			"GET /Videos/{Id}/main.m3u8 Id",
			"GET /Videos/{Id}/master.m3u8 Id",
			"GET /Videos/{Id}/stream Id",
			"GET /Videos/{Id}/stream.{Container} Id",
			"GET /Videos/{Id}/subtitles.m3u8 Id",
			"GET /Videos/{Id}/{MediaSourceId}/Attachments/{Index}/Stream Id",
			"GET /Videos/{Id}/{MediaSourceId}/Subtitles/{Index}/Stream.{Format} Id",
			"GET /Videos/{Id}/{MediaSourceId}/Subtitles/{Index}/{StartPositionTicks}/Stream.{Format} Id",
			"GET /Videos/{Id}/{StreamFileName} Id",
			"POST /Collections/{Id}/Items Id",
			"POST /Collections/{Id}/Items/Delete Id",
			"POST /Items/RemoteSearch/Apply/{Id} Id",
			"POST /Items/{Id}/Delete Id",
			"POST /Items/{Id}/Images/{Type} Id",
			"POST /Items/{Id}/Images/{Type}/Delete Id",
			"POST /Items/{Id}/Images/{Type}/{Index} Id",
			"POST /Items/{Id}/Images/{Type}/{Index}/Delete Id",
			"POST /Items/{Id}/Images/{Type}/{Index}/Index Id",
			"POST /Items/{Id}/Images/{Type}/{Index}/Url Id",
			"POST /Items/{Id}/MakePrivate Id",
			"POST /Items/{Id}/MakePublic Id",
			"POST /Items/{Id}/PlaybackInfo Id",
			"POST /Items/{Id}/Refresh Id",
			"POST /Items/{Id}/RemoteImages/Download Id",
			"POST /Items/{Id}/RemoteSearch/Subtitles/{SubtitleId} Id",
			"POST /Items/{Id}/Subtitles/{Index}/Delete Id",
			"POST /Items/{Id}/Tags/Add Id",
			"POST /Items/{Id}/Tags/Delete Id",
			"POST /Playlists/{Id}/Items Id",
			"POST /Playlists/{Id}/Items/Delete Id",
			"POST /Playlists/{Id}/Items/{ItemId}/Move/{NewIndex} Id",
			"POST /Users/{UserId}/FavoriteItems/{Id} Id",
			"POST /Users/{UserId}/FavoriteItems/{Id}/Delete Id",
			"POST /Users/{UserId}/PlayedItems/{Id} Id",
			"POST /Users/{UserId}/PlayedItems/{Id}/Delete Id",
			"POST /Users/{UserId}/PlayingItems/{Id} Id",
			"POST /Users/{UserId}/PlayingItems/{Id}/Delete Id",
			"POST /Users/{UserId}/PlayingItems/{Id}/Progress Id",
			"POST /Videos/{Id}/AlternateSources/Delete Id",
			"POST /Videos/{Id}/Subtitles/{Index}/Delete Id",
		},
		Except: []string{
			"LiveStreamRequest .ItemId",
			"POST /Playlists/{Id}/Items/{ItemId}/Move/{NewIndex} ItemId",
			"POST /Sessions/{Id}/Playing ItemIds",
			"RemoteSearchQueryAlbumInfo .ItemId",
			"RemoteSearchQueryArtistInfo .ItemId",
			"RemoteSearchQueryBookInfo .ItemId",
			"RemoteSearchQueryGameInfo .ItemId",
			"RemoteSearchQueryItemLookupInfo .ItemId",
			"RemoteSearchQueryMovieInfo .ItemId",
			"RemoteSearchQueryMusicVideoInfo .ItemId",
			"RemoteSearchQueryPersonLookupInfo .ItemId",
			"RemoteSearchQuerySeriesInfo .ItemId",
			"RemoteSearchQueryTrailerInfo .ItemId",
			"SyncJob .ItemId",
			"SyncJob .ParentId",
			"SyncJobItem .ItemId",
		},
	},
	{
		Name: "UserID", Of: []string{"UserDto"}, Names: []string{"ControllableByUserId", "ControllingUserId", "CopyFromUserId", "LastUserId", "SourceUserId", "TargetUserId", "ToUserIds", "UserId", "UserIds"},
		Except: []string{
			"SessionPartyMessage .UserId",
			"SyncJob .UserId",
		},
	},
	{
		Name: "SessionID", Of: []string{"Session.SessionInfo"}, Names: []string{"SessionId"},
		In: []string{
			"GET /Sessions Id",
		},
	},
	{
		Name: "TaskID", Of: []string{"TaskInfo"},
	},
	{
		Name: "PlaylistItemID", Base: "string", Of: []string{}, Names: []string{"PlaylistItemId", "PlaylistItemIds"}, Joined: []string{"EntryIds"},
	},
	{
		Name: "MediaSourceID", Of: []string{"MediaSourceInfo"}, Names: []string{"MediaSourceId"},
	},
}

// embyPlainIDs are the id-looking names left as the document has them, on purpose: ids of things no kind is kept for (a device, a channel, a genre),
// and Id and Ids themselves, which mean a different thing from one record to the next. A new name the import reports goes here or into a kind.
var embyPlainIDs = []string{"AffectedPropertyId", "AlbumArtistIds", "AlbumIds", "AppId", "ArtistIds", "AssemblyGuid", "AuthenticationProviderId", "CategoryId", "ChannelId", "ChannelIds", "CodecId", "CollectionIds", "CommandId", "CommandParameterPropertyId", "ComposerArtistIds", "ConnectUserId", "ConsumerId", "ContributingArtistIds", "CurrentPlaySessionId", "DeviceId", "DeviceIds", "DeviceProfileId", "DisplayPreferencesId", "EventIds", "ExcludeArtistIds", "ExcludeTagIds", "FeatureId", "featureId", "GameSystemId", "GenreIds", "Guid", "guid", "Id", "id", "Ids", "InternalDeviceId", "InternalId", "InternalTargetId", "InternalTargetIds", "JobId", "LibraryIds", "ListingsChannelId", "ListingsId", "ListingsProviderId", "ListItemIds", "LiveStreamId", "LocalItemIds", "LocalUserId", "ManagementId", "NavMenuId", "OuterIds", "OwnerId", "PageId", "ParentFolderId", "PartyId", "PersonIds", "PlaylistId", "PlaySessionId", "PluginId", "PostbackCommandId", "ProgramId", "ProviderChannelId", "ProviderId", "ReportedDeviceId", "RequestedItemIds", "SectionId", "SegmentId", "SeriesTimerId", "ServerId", "StudioIds", "SubtitleId", "SyncJobId", "SyncJobItemId", "TagIds", "TargetEditorId", "TargetId", "TargetPropertyId", "TimerId", "TunerChannelId", "UserInternalId", "UuId", "VendorId", "ViewId"}

// jellyfinIDs are the same six kinds for Jellyfin, whose document names its ids by what they are (itemId, userId), so the names alone place them. An
// artist, a genre, a person and a studio are items there, and their ids are an item's.
var jellyfinIDs = []config.ID{
	{
		Name: "ItemID", Of: []string{"BaseItemDto"}, Names: []string{"albumArtistIds", "albumId", "albumIds", "appearsInItemId", "artistIds", "BackdropImageItemId", "collectionId", "contributingArtistIds", "excludeArtistIds", "excludeItemIds", "genreIds", "ids", "itemId", "itemIds", "ParentArtItemId", "ParentBackdropItemId", "parentId", "ParentLogoItemId", "ParentPrimaryImageItemId", "ParentThumbItemId", "personIds", "playlistId", "PrimaryImageItemId", "routeItemId", "seasonId", "seriesId", "startItemId", "studioIds", "ThumbImageItemId", "videoId"},
	},
	{
		Name: "UserID", Of: []string{"UserDto"}, Names: []string{"controllableByUserId", "controllingUserId", "LastUserId", "userId"},
	},
	{
		Name: "SessionID", Of: []string{"SessionInfoDto"}, Names: []string{"sessionId"},
	},
	{
		Name: "TaskID", Of: []string{"TaskInfo"}, Names: []string{"taskId"},
	},
	{
		Name: "PlaylistItemID", Base: "string", Of: []string{}, Names: []string{"entryIds", "PlaylistItemId", "PlaylistItemIds"},
	},
	{
		Name: "MediaSourceID", Of: []string{"MediaSourceInfo"}, Names: []string{"mediaSourceId", "routeMediaSourceId"},
	},
}

// jellyfinPlainIDs are Jellyfin's id-looking names left as the document has them, for the reasons embyPlainIDs gives.
var jellyfinPlainIDs = []string{"assemblyGuid", "AuthenticationProviderId", "CastReceiverId", "CategoryId", "ChannelId", "channelId", "ChannelIds", "channelIds", "DeviceId", "deviceId", "deviceProfileId", "DisplayPreferencesId", "displayPreferencesId", "DvBlSignalCompatibilityId", "ExternalChannelId", "ExternalId", "ExternalProgramId", "ExternalSeriesTimerId", "folderId", "GroupId", "Guid", "guid", "Id", "id", "imdbId", "LibrarySeriesId", "librarySeriesId", "ListingsId", "LiveStreamId", "liveStreamId", "lyricId", "MessageId", "OwnerId", "packageId", "PasswordResetProviderId", "PlaySessionId", "playSessionId", "PluginId", "pluginId", "ProgramId", "programId", "ProviderChannelId", "ProviderId", "providerId", "recordingId", "SeriesTimerId", "seriesTimerId", "ServerId", "streamId", "subtitleId", "TimerId", "timerId", "tmdbId", "TunerChannelId", "tunerId", "tvdbId"}
