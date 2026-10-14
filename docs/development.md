# Development

```bash
make            # format and build
make check-all  # build, unit tests, every live suite on both servers (needs docker), and every linter
make help       # every target
```

Dev tools are pinned in `.tools/go.mod` and built into `.tools/bin` by make. On a noexec checkout point `TOOLS_BIN` somewhere local, for example `make TOOLS_BIN=~/.cache/embyfin-mcp/bin lint`.

How the generated clients are made and refreshed is in [sdk/README.md](../sdk/README.md).

## Unit tests

`make test` is hermetic and fast. It covers the pure logic:

- Tool registration and toolsets.
- The audit rules.
- The CLI's flags, settings files and HTTP auth.
- Against a canned server, the requests the generated clients and `sdk/embyfin` build for each server, and the answers they decode.

It also imports the API documents again and regenerates the SDKs, to check the checked-in code is current. And it applies every importer workaround twice, to prove each one notices when its bug is fixed.

## Live tests

Everything else runs against a real Emby and a real Jellyfin in Docker, because a stub can only confirm what you already believed. It all lives under `acceptance/`, behind the `acceptance` build tag, each suite in its own container.

| Suite | Covers | Command |
|---|---|---|
| `acceptance/emby`, `acceptance/jf` | The generated `sdk/emby` and `sdk/jf` clients | `make testacc-sdk` |
| `acceptance/tmdb` | The generated `sdk/tmdb` client, replayed from recordings | `make test-tmdb` |
| `acceptance/embyfin` | The hand-written `sdk/embyfin` layer | `make testacc-embyfin` |
| `acceptance/tools` | The tools, and the built binary | `make testacc-tools` |

```bash
make testacc                        # every live suite on both servers, each in a throwaway container
make testacc-tools-jellyfin         # one suite on one server (testacc-sdk-jellyfin for the SDK's)
make cover                          # every suite merged into one coverage number
```

The suites skip when `EMBYFIN_SERVER` and `EMBYFIN_TOKEN` are unset, so they never fail for want of a daemon. They need docker, ffmpeg, jq and openssl.

### What each suite does

**The SDK suites:**

- Bespoke tests that the calls the tools rely on decode with their fields filled in, and do what they say.
- A sweep that calls every GET in each document against the server, and classifies the ones that cannot answer in a container.
- `sdk/tmdb` gets the same sweep against the real TMDB API, recorded once with a token and replayed with none: `make test-tmdb` and `make record-tmdb`.

**The `sdk/embyfin` suite.** Its promise is the same answer from either server. Each test is written once and must pass on both, over libraries, items, watch state, collections, playlists, episodes, accounts and tasks.

**The tool suite:**

- Name resolution, trimmed answers, audits and provider flows.
- Journeys that chain the tools. One goes from nothing to audited to fixed and back to nothing: an account, a library over its own folder, every finding fixed with the tools, a collection, a playlist, watch state, and all of it removed again.
- Other journeys: edits during a library scan and through a refresh, every fixable audit fixed and audited again, the fixes each audit names with the fetchers on and off, a client's playback reaching the history tools, a series watched through, a user restricted to one library, a library's whole life, deletes letting go of what held their items, writes repeated and made in parallel, copies of a film watched, a genre added, renamed and removed, and lookups that must change nothing.
- The built binary itself over stdio and HTTP: flags and environment reaching the server, nothing but protocol on stdout, the bearer check, clean shutdown, and refusing to start without a token.

### Every tool is exercised on both servers

Tool coverage is enforced, not claimed. The tool suite records every tool it calls, and fails if the server registered one that nothing called. So a new tool cannot ship untested.

### Shared test pieces

The environment, the proxy and the way the tools are driven as a client drives them come from go-kt: `test/env`, `test/replayproxy` and `mcp/acctest`. They are shared with the other MCP servers. `lib/testenv` holds this suite's settings for them, and `lib/acceptance` adds the waits on scans.

## Coverage

Coverage has to span every suite or it lies. `go test -cover ./...` reports a fraction for `tools/`, because almost everything real happens in the live suites.

- `make cover` runs each suite into its own coverage directory and merges them with `go tool covdata`. That is standard tooling, with no third-party merger, and it is what the badge reports.
- The generated `sdk/emby`, `sdk/jf` and `sdk/tmdb` are left out of the number and reported on a line of their own. They are one method per operation, and the SDK suites exercise the ones the tools rely on, not all 997.

## Recorded provider calls

The servers' own calls out to TMDB, TheTVDB, OMDb and the image CDNs go through a record and replay proxy, go-kt's `test/replayproxy`. The containers are started with `HTTPS_PROXY` pointing at it and trust its certificate authority. So no suite needs a network.

```bash
make record         # re-record the cassettes against the real providers (EMBYFIN_TMDB_TOKEN for the runtime audit)
make record-check   # check the cassettes still match, without rewriting them
```

`record-check` compares the shape of live answers against the recordings: renamed fields, vanished fields, changed types. It ignores values. So it goes red when a provider changes its contract, not when a poster changes.

## Fixtures

Fixtures are generated, never committed. `.scripts/testenv.sh`:

- Writes videos with `ffmpeg`. Every film and episode runs TMDB's length for it, as a still frame joined end to end, so it costs a few megabytes.
- Puts them under `~/.cache/embyfin-mcp`. `EMBYFIN_TEST_DATA` moves them. Do not use `$TMPDIR`, which Docker Desktop does not share.
- Gives each a Kodi-style `.nfo` carrying the real film's ids.
- Completes the server's setup wizard, makes an API key and a second user, and prints the environment.

`.scripts/testenv.sh fixtures` writes just the media tree, if you want to look at the layout.

The suites create the libraries through `library_create` and fill them with `library_scan`, so building the fixtures is itself part of the coverage.

- **Two clean libraries** fetch their metadata from the providers, through the proxy. So the audits have something to leave alone, and identify has something to find.
- **Two `Messy` libraries** keep the providers off and are seeded with every defect the audits exist to find: a film with no ids, one with no plot and no poster, a folder whose year disagrees with its metadata, one film twice, one film in two versions, a file cut short, and a runtime that cannot be right. Each audit has a test against them.
- **A music library** holds four artists, five albums and twenty one-second tracks tagged with the real MusicBrainz ids, with art beside and inside them. So the mixes have something to work with, and the audits can be pointed at albums. It keeps the providers off too, and it carries a rip with no art and a genre spelled two ways.
