# Cleaning up after a removed library

A library whose folder is renamed or removed can leave its items behind. No library lists them and nothing can play them, but every sweep of the server counts them.

Deleting an item deletes its file, so they are only removed once their folder is gone.

1. Load the tools with `--toolsets admin --enable-delete`, without `--read-only`. Deleting is off by default.
2. `audit_orphans` lists them by folder, and says whether the server can still see each folder.
3. `item_orphans_delete folder=...` shows what it would delete. Nothing is deleted without `confirm=true`.
4. `item_orphans_delete folder=... confirm=true` deletes up to `limit` items a call, 2,000 by default. Call it again until `remaining` is 0.
5. Turn `--enable-delete` off again.

What it refuses:

- A folder inside a library, or holding one.
- A folder the server can still see.
- A path with `.` or `..` in it.

It checks the folder again before every batch.
