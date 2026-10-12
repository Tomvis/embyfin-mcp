# sdk

`sdk/emby`, `sdk/jf` and `sdk/tmdb` are written by [pandorest](https://github.com/katbyte/pandorest) from their vendored OpenAPI documents, each into a folder named for its document's version. pandorest began in this repository and is now a library of its own, shared with the other MCP servers; what stays here is what is embyfin's:

| | |
|---|---|
| `zservices` | the three APIs: where each document, its definitions and its package live, how its methods are named, how its lists page, how it writes a date, which of its ids are told apart (`ids.go`), which base client and authorizer it uses, and the wording of its `New` |
| `emby/workarounds`, `jf/workarounds`, `tmdb/workarounds` | one named fix for each bug in that service's document |
| `main.go` | hands both to pandorest and runs its command |

```text
api-defs/
  emby-openapi-4.10.0.40.json ──┐               ┌─ emby-4.10.0.40/*.json ──┐
  jellyfin-openapi-12.1.0.json ─┼─ import ──────┼─ jellyfin-12.1.0/*.json ─┼─ generate ─ sdk/emby, sdk/jf, sdk/tmdb
  tmdb-openapi-2026.09.22.json ─┘ (workarounds) └─ tmdb-2026.09.22/*.json ─┘
                                                        │
                                         diff ──────────┘  (what a refreshed spec changes)
```

```bash
make generate          # import + generate
make pandorest-diff    # what the specs change against their checked-in definitions, breaking changes marked
make apicheck          # definitions match the specs, and every operation has a method
make gencheck          # regenerate, fail if anything is uncommitted
```

How a document is imported, what the definitions hold, how the diff reads and what the generator writes are in [pandorest's README](https://github.com/katbyte/pandorest#readme). The generated code sends its requests through pandorest's own base client; what is each server's own about being talked to, how it takes a key and what a refusal means on it, is the `service.go` in the package named for the server, beside the generated types.

## Dates and ids

A date is a `Time`, or a `Day` for one with no time, read in every layout the service's entry lists; one written any other way is zero with its text kept in `Unread`, and the live sweeps fail an answer that carried one, which is how a layout a server adds is found. TMDB's document declares its dates plain strings, so two workarounds name them.

An id is one of six kinds on Emby and Jellyfin (`ItemID`, `UserID`, `SessionID`, `TaskID`, `PlaylistItemID`, `MediaSourceID`), so one kind where another goes does not compile. `zservices/ids.go` says which names and places are which kind, and lists the id-looking names left as plain text on purpose; the import reports a name that is in neither.

## Workarounds

A document bug is fixed with a workaround in its service's `workarounds` folder, added to `All` there. For the bugs most documents have, it is one of pandorest's ready-made ones, which says only what is wrong and why: `WrongAnswers` for an operation that declares one answer and gives another (`/Environment/ParentPath` declares a JSON string and answers bare text), `UndeclaredAnswers` for one that declares none, `UndeclaredParameters` for a parameter the server reads and the document leaves out, `JoinedLists` for a text option the server reads as several values joined by commas (Emby's `IncludeItemTypes`), which then takes a list as well, `UndeclaredDays` and `UndeclaredDates` for a date declared a plain string (TMDB's `release_date`). pandorest does the patching and checks the bug is still there.

Anything else is a type with a `Name` (`emby-search-term`), the `Service` it is for, the `Bug` it fixes in a sentence, and `Apply`, which patches the loaded document.

`Apply` must first check the bug is there, and return an error when it is not: the parameter it adds already declared, the operation gone, the schema already carrying the field. A refreshed document that fixes the bug then fails the import, naming the workaround to delete, instead of the workaround silently doing nothing forever. The unit tests enforce this: every workaround must apply to its vendored document and must fail when applied a second time.

Workarounds are for the document's shape, what an operation takes and answers. Behaviour no document could express (Emby keying watch state by provider id, a filter it silently drops, an add it loses mid-refresh) belongs in `sdk/embyfin`, next to the live test that found it; `api-defs/README.md` lists both kinds.

## Refreshing a document

A document and its definitions are named by the document's own version (`info.version`): `api-defs/<service>-openapi-<version>.json` imports into `api-defs/<service>-<version>/` beside it, and the highest version present is the one imported and generated from. To refresh a spec, vendor the new document beside the old one; `make generate` then imports it into its own definitions directory, `make pandorest-diff` says what changed, and the old pair is deleted once the new one is in. `.scripts/spec-refresh.sh` does this from running servers.
