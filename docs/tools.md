# The tools

There are 92 tools: 62 read, 24 write and 6 delete. `embyfin-mcp tools --toolsets all --enable-delete` prints every one with what it does, and needs no server. Which of them load is in [toolsets.md](toolsets.md).

## What holds for every tool

- **Named by what they act on.** `library_*`, `item_*`, `audit_*`, `session_*` and so on.
- **Marked.** Every tool carries MCP annotations, read-only or destructive, and a tool that changes the server says so in its description.
- **Names as well as ids.** Wherever a tool takes a library, user, session, playlist or collection it accepts a name as well as an id. An unknown name comes back as an error listing what exists.
- **Whose watch state.** An API key acts as the server, not as a user. So a tool that reads or changes watch state takes a `user`, by name or id, and defaults to the first administrator.
- **Timeframes.** A tool that takes a timeframe defaults to the last 60 days.
- **Short answers.** Every answer is a trimmed view of what a decision needs, never the server's raw record. The server's own record of an item, `BaseItemDto`, runs to 150 fields; `item_get` returns about 15.

## Audits

`audit_all` and the 17 audits are in [audits.md](audits.md).

## Server

- `server_info`: the server's name and version, whether a restart is pending, and the machine as the server's log describes it.
- `server_health`: how the server is doing in one call. Tasks running and failed, who is playing and how, and what the last stretch of the log shows.
- `server_stats`: counts of films, series, episodes, music, users and active sessions.
- `server_activity`: the activity log, or the entries about one item or one user.
- `server_devices`: the devices and apps that have connected.
- `server_plugins`: what is installed into the server, and on Jellyfin whether each is running.
- `server_log`: the newest log's tail, and every log file.
- `server_log_search`: the log between two times or over its last stretch, filtered. It answers as entries, counts, a histogram, the gaps of a freeze, or the slow answers.
- `server_config`: the server's own settings, in groups.
- `server_config_edit`: on Jellyfin, the settings that tune trickplay images and scanning, and no others.

## Tasks

- `task_list`: the scheduled tasks, with progress, what starts each, and how its last run went.
- `task_get`: one task, with its last failure in full.
- `task_run`: start a task.
- `task_stop`: stop a running task.
- `task_edit`: change when a task starts by itself.

## Libraries

- `library_list`: every library, with its type and folders.
- `library_get`: one library, with counts by type.
- `library_options`: every library's settings side by side, and which libraries are set differently.
- `library_items`: a title search, a structured filter, or both, sorted and paged. It filters by genre, tag, studio, rating, year, person and watch state. `added_since` gives what came in after a moment.
- `library_filters`: every genre, tag, studio, rating and year, with counts.
- `library_episodes`: every episode of a show, one season of it, or a whole library, paged, with quality.
- `library_export`: a whole library to a new file on the machine embyfin-mcp runs on, never over an existing one.
- `library_scan`: scan every library, or one.
- `library_create`: make a library over folders on the server.
- `library_edit`: rename, add and remove folders, switch nfo saving.
- `library_delete`: remove a library. See [Deleting](#deleting).

## Items

- `item_get`: one item, with every version the server shows it in, and a warning when a film's file names another film.
- `item_find_by_metadata_id`: the films or series matching a TMDB, IMDb or TVDB id. The definitive "do I already have this?"
- `item_similar`: items the server considers similar.
- `item_refresh`: fetch an item's metadata again. It waits for the refresh to land.
- `item_previews_regenerate`: make a video's preview thumbnails again, one video at a time, and name anything else the refresh changed. Emby only so far.
- `item_edit`: one item's fields, or the same genres, tags, studios or rating across many. `add_*` and `remove_*` edit each item's own list.
- `item_instant_mix`: a music mix seeded from a song, album, artist or genre.
- `item_last_watched`: each user's watch state for one item.
- `item_set_state`: watched, favourite and resume point, any or all.
- `metadata_rename`: rename a genre, tag or studio everywhere it is used. Renaming onto an existing value merges the two, and `remove` drops it.
- `person_get`: an actor, director or writer, and everything the library holds with them in it.
- `quality_compare`: which of two copies is better, by how much, and why.
- `plan_check`: before writing files, what is at each destination path now, which series it would join, and which entries collide.

## Matching, artwork and subtitles

- `item_identify`: candidates from the server's providers.
- `item_identify_apply`: apply one. It takes the candidate and its `candidate_ids`; a candidate without ids cannot be applied.
- `item_artwork`: an item's current images, plus remote candidates.
- `item_artwork_set`: set one. On Emby this deletes the poster file beside the media it replaces.
- `item_subtitle_search`: search remote subtitles for an item in a language.
- `item_subtitle_download`: download one.

## Shows

- `show_seasons`: a show's seasons. Its episodes come from `library_episodes`.
- `show_episodes_exist`: does the library hold these episodes? Up to 50 series a call, with the match score and any duplicate entries.
- `show_missing`: what a series is missing, and whether it could tell.
- `show_resolve`: a release name to a series, scored.

## Users

- `user_list`: the accounts.
- `user_get`: one account's permissions, libraries and playback preferences.
- `user_history`: what a user has played recently.
- `user_next_up`: the next episode of each series, and everything part way through, with positions.
- `user_stats`: films and episodes watched, in progress and favourited, hours, series finished, top genres and series, in one pass.
- `user_create`: an ordinary account, always with a password, seeing every library or only the ones named.
- `user_edit`: a new password, or which libraries the account sees.
- `user_delete`: remove an account. See [Deleting](#deleting).

## Sessions

- `session_list`: what each device plays and how. `details` adds the file and the transcode.
- `session_play`: play items on a device.
- `session_command`: pause, stop, seek and the like.
- `session_message`: show a message on a device's screen.

## Playlists and collections

- `playlist_list`, `playlist_get`, `playlist_create`.
- `playlist_edit`: rename, `add_items`, `remove_entries`, or move an entry. A removal and a move name an entry by its entry id and the item it holds, plus the playlist's `fingerprint` when an item is held twice.
- `playlist_delete`: the items stay in the library and only the list goes. Without `confirm` it says what the list holds.
- `collection_list`, `collection_get`, `collection_create`.
- `collection_edit`: rename, sort name, overview, `add_items`, `remove_items`.
- `collection_delete`: the items stay in the library and only the grouping goes. Without `confirm` it says what the collection holds.

## Provider answers

- `provider_cache_clear`: forget the answers kept from TMDB. They are kept an hour otherwise.

## Deleting

Six tools delete, and none of them can be undone. They load only when `--enable-delete` or `EMBYFIN_ENABLE_DELETE` is set.

- `item_delete` permanently removes the item's files.
  - A film alone in its folder, or a series, goes with the whole folder.
  - A film sharing a folder takes every sidecar whose name starts with its file's name, another film's too.
  - Without `confirm` it says what it would remove.
  - It refuses collections, playlists, libraries, genres, studios, people and artists, naming the tool that removes each.
- `item_orphans_delete` removes what a removed library left behind, once its folder is gone. The steps are in [recovery.md](recovery.md).
- `library_delete` removes a library.
- `playlist_delete` and `collection_delete` remove the list or the grouping, never the items. Without `confirm` they say what it holds.
- `user_delete` removes an account.
  - It never removes the last administrator.
  - The account's playlists stay.
  - On Jellyfin it waits three seconds first and refuses an account that is playing. Jellyfin 12.2 stops when an account is deleted in the half second after its watch state changes.

The same switch also allows `library_edit`'s `remove_paths`, and `task_run` of anything but a scan.

`--read-only` loads the 62 read tools and nothing else. A write tool is then absent from the tool list, not refused when called.
