# Choosing which tools load

The default is `core`: six read-only tools, about 2,000 tokens.

Every tool's definition takes room in the client's context before anyone asks a question. The whole surface is about 36,800 tokens, which is a poor way to spend a client's context by default. So a session loads the groups it needs.

## Toolsets

`--toolsets` or `EMBYFIN_TOOLSETS` names the groups to load. `core` comes along with whatever else is asked for, because nothing else can find a library or open an item.

| Toolset | Tools | With core | About, in tokens |
|---|---|---|---|
| `core` (default) | 6 | 6 | 2,000 |
| `remote` | 4 | 10 | 2,800 |
| `admin` | 25 | 31 | 10,400 |
| `watching` | 8 | 14 | 3,800 |
| `organise` | 10 | 16 | 5,100 |
| `curation` | 39 | 45 | 22,700 |
| `all` | 92 | 92 | 36,800 |

- **`curation`** is for curating a library: every audit but `audit_orphans`, and everything that fixes what they find.
- **`admin`** holds cleaning up after a removed library; see [recovery.md](recovery.md).
- **`all`** loads every tool.

How the tokens are counted:

- They are what the model sees: each tool's name, description and input schema.
- They are measured over a real `tools/list` at four bytes a token, with `--enable-delete`.
- Every tool also carries an output schema, another 66,000 tokens across `all`. Clients keep that to themselves to check answers, and do not send it to the model.

## Families

`--toolsets` also takes a resource family, which is every tool with that prefix: `audit`, `collection`, `item`, `library`, `metadata`, `person`, `plan`, `playlist`, `provider`, `quality`, `server`, `session`, `show`, `task` and `user`.

```sh
EMBYFIN_TOOLSETS=all                # every tool
EMBYFIN_TOOLSETS=curation           # audits plus everything that fixes what they find
EMBYFIN_TOOLSETS=watching,remote    # a client that plays things rather than curates them
EMBYFIN_TOOLSETS=audit              # read-only detection, nothing that writes
EMBYFIN_TOOLSETS=core,item,show     # core plus two whole families
```

## Seeing what would load

`embyfin-mcp tools` prints what the current flags would load, grouped by toolset. It needs no server.

```sh
embyfin-mcp tools                   # the default set
embyfin-mcp tools --toolsets all    # every tool
embyfin-mcp tools --read-only -q    # names only
```

## Allow and deny lists

`--allow-tools` and `--deny-tools` take comma-separated tool names, or patterns with a leading or trailing `*`.

- **An allow list with no toolsets** is the whole of what loads: only the tools it names.
- **An allow list beside toolsets** adds to them: the sets, and the tools named as well.
- **A deny list** takes tools out of whatever was asked for, which is how a set is narrowed.
- **`essential`** is a ready-made allow list: `library_list`, `library_items`, `item_get`, `user_next_up` and `item_set_state`.

```sh
EMBYFIN_ALLOW_TOOLS=essential                      # exactly those five
EMBYFIN_ALLOW_TOOLS=library_*,item_get,user_*      # every library_ and user_ tool, and item_get
EMBYFIN_ALLOW_TOOLS=audit_*                        # the audits alone
EMBYFIN_TOOLSETS=curation EMBYFIN_ALLOW_TOOLS=task_list,task_run   # curation, and two tools from admin as well
EMBYFIN_TOOLSETS=curation EMBYFIN_DENY_TOOLS=audit_*   # curation without its audits
EMBYFIN_DENY_TOOLS=*_delete,session_*
```

A pattern that matches no tool stops the server from starting and names the pattern, so a typo cannot silently hide a tool.

## Read-only and deleting

Neither list gets a tool past these two switches.

- `--read-only` loads the 62 read tools and nothing else.
- The six tools that delete load only with `--enable-delete`. They are listed in [tools.md](tools.md#deleting).
