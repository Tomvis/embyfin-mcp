# The audits

Each audit sweeps a whole library for one thing that goes wrong. It answers with a short list of what to look at, not a dump of the library. The audits are in the `curation` toolset, apart from `audit_orphans`, which is in `admin`.

A TMDB token means `EMBYFIN_TMDB_TOKEN`; see [configuration.md](configuration.md).

## audit_all

Runs the audits in one call and gives counts only, so one call says where a library needs work. Start here after a scan.

- Every audit has a row.
- Four are not run and are marked skipped, with why: `audit_language` needs a language, `audit_provider` asks TMDB, `audit_anime_ids` reads the Anime-Lists file, and `audit_previews` asks the server about every video.
- A music library is counted for its albums' covers and its genre spellings.

## audit_missing_metadata

Finds items missing what a matched, finished item has. Each finding names which:

- **A provider id of any kind.** The item was never matched, and nothing else can be filled in automatically. A link to a show's website or Facebook page is not an id. `item_identify` fixes it.
- **A primary image.** `item_artwork` shows the choices and `item_artwork_set` sets one.
- **A plot.** Usually a failed match. `item_refresh` or `item_identify` fixes it.

Options:

- `problems` picks which to look for, and `by_problem` counts each.
- `missing` narrows the id check to the providers named. `missing=tmdb` also finds a show matched on TVDB or IMDb but not on TMDB.
- `ignore` leaves out libraries whose items never carry an id, such as YouTube.

## audit_file_path

Finds items whose path disagrees with their metadata. Every version's file is checked. `item_identify` or `item_edit` puts the metadata right, and a rename fixes the path.

Films and series:

- A folder saying `(2021)` under a film matched to 1984 is the wrong edition, or the wrong film.
- A folder or file named for a different title is a row.
- The path is held against every title the item goes by: its name, its original title and its sort name. With a TMDB token, the alternative titles TMDB lists count too.
- A year one either side is the same film.
- With a TMDB token, a path naming another film says which film, beside the id the item carries, and whether the file's runtime backs either.

Episodes are checked for the series, season and episode number the file name claims, against the ones the server holds:

- A file holding two episodes (`S01E01E02`) where the server lists one has the second's content on disk while the server calls it missing.
- A file named after one episode where the server holds another is a file from another series. With a TMDB token it may instead be one numbered in another provider's order, and the row says which TMDB episode the file's title is.

Names:

- A name spelled with a letter of another script that only looks Latin is its own row, since no search for the plain title finds it. A Cyrillic A (U+0410) in a Latin title is one.

Options:

- `checks` narrows to any of title, year, series, season, episode and lookalike.
- `untitled` is one more check, run only when `checks` names it. It lists the episode files whose name gives a number and no title: `Show - 07x08.mkv`, or `Show - 07x08 - .mkv` where a renamer wrote the dash for a title it did not have. Each row carries the server's title to rename the file by.
- `ids` checks a handful of items.

## audit_duplicates

Finds separate entries sharing one TMDB or IMDb id, in one library or across libraries.

- Each group lists every copy with its path and quality.
- A group whose file names another film is marked as probably two films on one id, not copies.
- `folder_groups` are the shows held twice because two folders name the same series. A rename that changed only spacing, case, an accent or punctuation does this. It splits the episodes across two entries that each answer "no" to half the questions. Sharing an id cannot see it when the second entry carries no id.

`quality_compare` says which of two copies is better.

## audit_multiple_versions

Finds one entry the server has merged from several files, such as a 4K and a 1080p copy. This is what `audit_duplicates` cannot see.

- On both servers it reads the library as people see it, so a pair Emby shows as one film is versions, not duplicates.
- A version whose file names another film is marked as probably another film merged in on the same ids.

## audit_runtime

Finds films and episodes whose runtime no film or episode can have:

- **Under 2 minutes:** a download cut off, a sample, or a broken file.
- **12 hours or more:** broken duration metadata.

Nothing is judged against its season or library. A length against TMDB's is `audit_provider`. A disc image (`.iso`) is counted and not judged, because the runtime a server holds for one can be a single title's or a menu's.

## audit_quality

Finds films and episodes worth replacing with a better copy:

- **Below a resolution:** 720 lines by default, so 480p and 576p rips.
- **In a legacy codec:** MPEG-2, XviD and DivX, WMV, VC-1.
- **Below a bitrate,** when one is given.

An item is judged by its best version on both servers, since Emby stores each version apart. The lowest resolution comes first.

Two more lists say which facts cannot be trusted:

- Files the server never probed. Every quality question reads as nothing for these, so they are not judged.
- On Emby, files written over after the server first saw them. Their facts may be the old file's until a scan reads them again. Each such row carries the size the server believes.

## audit_missing_episodes

Finds series with episodes missing:

- The numbers a season skips between the episodes on disk.
- Whole seasons skipped.
- When the server records them, the episodes its provider lists without a file.

`provider: true` reads every series' whole run from the metadata provider instead, which is TMDB today. It makes one request a series and is paged. What a series lacks after its last file is then seen too.

A show held under two entries sharing its ids, or in two folders named alike beside each other, is judged as one.

## audit_spelling

Finds genres, tags and studios that mean the same thing spelled differently:

- `Sci-Fi` and `Sci Fi`, or `Science-Fiction` and `Science Fiction`.
- Two spellings a letter apart.
- A studio cut short: `Warner Bros.` and `Warner Bros. Pictures`.

Each group names the spelling to keep, and `metadata_rename` merges it.

In a music library it reads the tracks' album and artist names too: an album spelled two ways by one album artist, or an artist written with and without `The`. These are the files' own tags, so they are fixed in the files, not with `metadata_rename`.

## audit_whitespace

Finds doubled, leading, trailing and odd spaces, and a space before a colon or a file extension.

- It reads names, sort names, original titles, genres, tags, studios, people, folders and file names.
- Each row has the fixed text beside it.
- A double space in a path usually marks a character a renamer stripped, so the row sets the path beside the item's title.

`item_edit` fixes a name and `metadata_rename` a genre, tag or studio. A folder or file is renamed on disk.

## audit_unwatched

Finds the films, or series, no account on the server has watched: what to archive, or what to recommend.

- The oldest additions come first.
- It can list only those added more than some days ago.
- A watch by an account that has since lost access to the library still counts.
- A watch by an account now held back by a parental rating does not, as neither server lists what the limit hides.

## audit_language

Finds films and episodes by the language of their audio or subtitles:

- What has audio or subtitles in a language.
- What has no audio in it.
- What cannot be watched in it at all.

A track with no language tag is never taken as lacking one.

## audit_duplicate_episodes

Finds one episode's content filed under two episode numbers: a season holding the same episode title twice. Neither other duplicate audit can see this.

Each group says how sure it is:

- **Near certain** only on proof in the files: the same size to the byte, or a runtime no other episode shares.
- **A lead** otherwise.
- **Far apart** when the lengths differ too much to be copies.

## audit_orphans

Finds items a renamed or removed library folder left behind. They are outside every library, so no library lists them, but every sweep counts them.

`item_orphans_delete` removes them once their folder is gone. The steps are in [recovery.md](recovery.md).

## audit_disc_folders

Finds a disc copied in as its own files: a Blu-ray's numbered streams or a DVD's VOBs in a film's folder, with no `BDMV` or `VIDEO_TS` structure. The server makes a film of each stream and matches them separately, which files short clips under other films' names.

## audit_previews

Finds videos whose preview thumbnails are missing, damaged or the wrong length for the video. Emby only so far.

- Preview thumbnails are the frames over the seek bar while scrubbing: one BIF file a video on Emby.
- Damaged means empty, not a thumbnail file, or cut off.
- Each file is read as it is on disk now.
- Only libraries that make thumbnails are read, paged.

Emby's scheduled task can pass over a video it made thumbnails for before (seen on 4.10.1), so a file deleted or damaged since can stay that way. `item_previews_regenerate` makes it again.

## audit_anime_ids

Checks anime against Anime-Lists, the community mapping of AniDB entries to TVDB and TMDB. It finds:

- Ids that disagree: a TMDB id that is the whole show beside an AniDB id that is one of its specials.
- Specials that are an OVA or a film of their own and could be split out into a series.
- Series already kept apart, with the AniDB entry that justifies it.

## audit_provider

Checks films and episodes against their metadata provider. Needs a TMDB token.

A film, one request each:

- The ids it holds agree with each other and exist there. It catches a TMDB id whose film carries a different IMDb id, a TMDB id TMDB no longer has, and an IMDb id that is a series or an episode.
- The file's runtime is the provider's. A difference is a truncated download, a wrong file or a wrong match.

An episode:

- The file's runtime against TMDB's for that episode, one read per series, naming the TMDB episode compared with.

Options:

- `provider` names which to ask. TMDB is the only one yet.
- It is paged with `offset`: every film and episode once, in the same order.
- `ids` checks a handful.
