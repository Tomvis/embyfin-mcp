# Running it as a service

By default `embyfin-mcp serve` speaks MCP over stdio, started by the client that uses it. This page is for an always-on instance that clients reach over HTTP.

## Over HTTP

`serve --listen :8080` serves the MCP Streamable HTTP transport at `/mcp`, with a health check at `GET /healthz`, instead of stdio.

- `EMBYFIN_AUTH_TOKEN` is required. Clients must send `Authorization: Bearer <token>`.
- The server refuses to start without one, unless `EMBYFIN_ALLOW_NO_AUTH=true` says that anyone who can reach the port may use every tool.

Register it from any machine:

```bash
claude mcp add --transport http embyfin http://nas:8080/mcp \
  --header "Authorization: Bearer $EMBYFIN_AUTH_TOKEN"
```

## Docker

Releases publish an image for amd64 and arm64 to `ghcr.io/katbyte/embyfin-mcp`, tagged `vX.Y.Z`, `vX.Y` and `latest`.

`docker-compose.yml` is the default always-on deployment. It runs that image and reads secrets from a gitignored `.env`. Adjust `EMBYFIN_SERVER`, `EMBYFIN_BACKEND` and `TZ` in the compose file, then:

```bash
cp .env.example .env      # fill in EMBYFIN_TOKEN and EMBYFIN_AUTH_TOKEN
docker compose up -d
```

About the image:

- `make docker` builds the same image from source, tagged `embyfin-mcp`, with version info from git.
- It is alpine-based, so `docker exec -it embyfin-mcp sh` works.
- It runs as a non-root user and has a healthcheck against `/healthz`.
- The binary is the entrypoint. So `docker run --rm ghcr.io/katbyte/embyfin-mcp info` works as a connectivity check, with the `EMBYFIN_*` variables passed with `-e`.
