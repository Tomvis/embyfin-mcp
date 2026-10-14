# Configuration

Every option can be passed as a command-line flag, as an environment variable, or in a settings file. A flag wins over a variable, and a variable over the file.

## Settings

| Variable | Flag | What it does |
|---|---|---|
| `EMBYFIN_BACKEND` | `--backend`, `-b` | `emby`, the default, or `jellyfin` |
| `EMBYFIN_SERVER` | `--server`, `-s` | The media server's address, such as `http://nas:8096` |
| `EMBYFIN_TOKEN` | `--token`, `-t` | The server's API key |
| `EMBYFIN_READ_ONLY` | `--read-only` | Load only tools that never change the server |
| `EMBYFIN_ENABLE_DELETE` | `--enable-delete` | Load the tools that delete; see below |
| `EMBYFIN_TOOLSETS` | `--toolsets` | Groups of tools to load, `core` by default; see [toolsets.md](toolsets.md) |
| `EMBYFIN_ALLOW_TOOLS` | `--allow-tools` | Load these tools by name; see [toolsets.md](toolsets.md#allow-and-deny-lists) |
| `EMBYFIN_DENY_TOOLS` | `--deny-tools` | Never load these tools |
| `EMBYFIN_TMDB_TOKEN` | `--tmdb-token` | A TMDB key; see below |
| `EMBYFIN_ANIME_LIST` | `--anime-list` | Where `audit_anime_ids` reads the Anime-Lists mapping: a URL or a file |
| `EMBYFIN_LOG` | | How much to log; see below |
| `EMBYFIN_LISTEN` | `--listen` | Serve over HTTP on this address, such as `:8080`, instead of stdio; see [service.md](service.md) |
| `EMBYFIN_AUTH_TOKEN` | `--auth-token` | The bearer token an HTTP client must send; required with `--listen` |
| `EMBYFIN_ALLOW_NO_AUTH` | `--allow-no-auth` | Serve HTTP with no token at all: anyone who can reach the port can use every tool |
| `EMBYFIN_CONFIG` | `--config` | Read settings from this file and no other |

### The API key

Make the key in the server's dashboard: under Advanced, API Keys on Emby, and under API Keys on Jellyfin.

An API key acts as the server, not as a user. So the tools that read or change watch state take a `user`, by name or id, and default to the first administrator.

### Deleting

`EMBYFIN_ENABLE_DELETE` loads the six tools that delete:

- `item_delete`, which removes the item's files, and a film's or a series' whole folder.
- `item_orphans_delete`, `library_delete`, `playlist_delete`, `collection_delete` and `user_delete`.

It also allows `library_edit`'s `remove_paths`, and `task_run` of anything but a scan.

### The TMDB key

`EMBYFIN_TMDB_TOKEN` takes a TMDB API Read Access Token, or the older API Key. `EMBYFIN_TMDB_KEY` and `--tmdb-key` still work. It turns on:

- `audit_provider`: films' ids and runtimes, and episodes' runtimes, checked against TMDB.
- `audit_missing_episodes` with `provider: true`: each series' whole run.
- In `audit_file_path`: the episode TMDB gives a file's title to, a film's alternative titles, and what TMDB's search finds by a path's title.
- `show_missing` on servers that keep no record of a series' run.

Answers are kept an hour. `provider_cache_clear` forgets them at once.

### The anime list

`EMBYFIN_ANIME_LIST` defaults to the list on GitHub, fetched once a day.

### Logging

`EMBYFIN_LOG` sets the log level. It has no flag.

- `WARN` is the default.
- `INFO` adds a line for every call of a write or delete tool.
- `TRACE` adds every request to the server and its answer, with keys blanked.

## The settings file

Put a `.embyfin-mcp` file in your home folder for settings everywhere, or in the current folder for one project. The one in the current folder is read in place of the one at home.

Keys are the long flag names, written the way the environment writes them:

```env
BACKEND=jellyfin
SERVER=http://nas:8096
TOKEN=ey...
TMDB_TOKEN=...
```

A two-word flag takes an underscore: `TMDB_TOKEN` for `--tmdb-token`.

`--config` or `EMBYFIN_CONFIG` names the file to read, and it is then the only one read. Nothing in `~/.embyfin-mcp` reaches that run. A file that is named and is not there stops the server from starting.

## Registering it with Claude Code

From the shell, passing settings as environment variables:

```bash
claude mcp add embyfin -e EMBYFIN_SERVER=http://nas:8096 -e EMBYFIN_TOKEN=... -- embyfin-mcp serve
```

Or in `.mcp.json`:

```json
{
  "mcpServers": {
    "embyfin": {
      "command": "embyfin-mcp",
      "args": ["serve"],
      "env": {
        "EMBYFIN_BACKEND": "jellyfin",
        "EMBYFIN_SERVER": "http://nas:8096",
        "EMBYFIN_TOKEN": "...",
        "EMBYFIN_TOOLSETS": "curation"
      }
    }
  }
}
```

`embyfin-mcp info` is a quick connectivity check.

## Two servers at once

Run one instance for each server, each with a settings file of its own, and register each under its own name. A client keeps their tools apart by that name, so the same question can be put to both.

```json
{
  "mcpServers": {
    "emby": { "command": "embyfin-mcp", "args": ["serve"] },
    "jellyfin": { "command": "embyfin-mcp", "args": ["serve", "--config", "~/.embyfin-mcp-jf"] }
  }
}
```

The first reads `~/.embyfin-mcp` as usual. The second reads `~/.embyfin-mcp-jf`, which sets `BACKEND=jellyfin` with its own `SERVER` and `TOKEN`.
