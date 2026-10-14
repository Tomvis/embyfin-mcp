# Using the Go clients on their own

`sdk/emby`, `sdk/jf` and `sdk/tmdb` are complete typed Go clients for the Emby, Jellyfin and TMDB APIs. They know nothing of MCP, and are useful whether or not you care about AI. If you only want to talk to one of the servers from Go, take the packages and ignore the rest.

- They cover every operation: 499 for Emby, 346 for Jellyfin and 152 for TMDB.
- They depend on nothing but the standard library, [pandorest](https://github.com/katbyte/pandorest)'s base client and `go-kt`.
- Each sits in a folder named for the version of the API document it was generated from, so the import path says exactly which API you are using. Each package's `APIVersion` says it too.

## Emby and Jellyfin

Under the version folder, `client` makes the client, the package named for the server holds every type, and each group of operations is a field on the client.

```go
import (
    "github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/client"
    "github.com/katbyte/embyfin-mcp/sdk/jf/12.1.0/jf"
)

c, err := client.New("http://nas:8096", os.Getenv("EMBYFIN_TOKEN"))
res, err := c.Library.GetItems(ctx, jf.GetItemsOptions{
    Recursive: new(true), IncludeItemTypes: []jf.BaseItemKind{jf.BaseItemKindMovie}, Limit: new(50),
})
for _, it := range res.Answer.Items { ... }

all, err := c.Library.GetItemsComplete(ctx, jf.GetItemsOptions{Recursive: new(true)}) // every page
sessions, err := c.Session.GetSessions(ctx)                                        // options can be left out when none is required
```

- An id says what it is the id of (`jf.ItemID`, `jf.UserID`), so a user's id where an item's goes does not compile.
- A date is a `jf.Time`, read in every form the server writes.

## TMDB

`sdk/tmdb` is the same for TMDB. It is generated from TMDB's own OpenAPI document and named for the day that document was taken. It takes an API Read Access Token or the older API Key.

```go
import (
    "github.com/katbyte/embyfin-mcp/sdk/tmdb/2026.09.22/client"
    "github.com/katbyte/embyfin-mcp/sdk/tmdb/2026.09.22/tmdb"
)

c, err := client.New(tmdb.DefaultBaseURL, os.Getenv("EMBYFIN_TMDB_TOKEN"))
film, err := c.Movie.MovieDetails(ctx, 550, tmdb.MovieDetailsOptions{AppendToResponse: "credits"})
found, err := c.Find.FindById(ctx, "tt0137523", tmdb.FindByIdOptions{ExternalSource: "imdb_id"})
```

## One client for both servers

`sdk/embyfin` is the layer the tools use. It holds the item, library, user, session and provider operations a curation session needs, and answers the same way on both servers.

```go
c, err := embyfin.New(embyfin.Jellyfin, "http://nas:8096", os.Getenv("EMBYFIN_TOKEN"))
```

[api-defs/README.md](../api-defs/README.md) records where the two servers differ and how this layer hides that.

## How they are generated

The clients are generated from the services' own OpenAPI documents, kept under `api-defs/`, with [pandorest](https://github.com/katbyte/pandorest). pandorest began in this repository and is modelled on [hashicorp/pandora](https://github.com/hashicorp/pandora).

- **An importer** turns each document into checked-in definitions, `api-defs/<service>-<version>/`, one file per tag. It goes through named workarounds for the document's known bugs.
- **A differ** reports what a refreshed document changes.
- **A generator** writes one file per operation and model from the definitions.

Each method has typed options, a typed body and an `{Answer, HttpResponse}` result. It knows the status codes the operation documents, and anything else is an error. Every paged list has a `Complete` pager.

What keeps it honest:

- `make apicheck` proves every operation has a method.
- `make gencheck`, and the unit tests, fail when the generated code is stale.
- The SDK acceptance suites prove the shapes against a running server. That is the only thing that catches a server changing shape underneath a document that says otherwise.

This repository's part of it, the services and the workarounds for their documents, is in [sdk/README.md](../sdk/README.md).
