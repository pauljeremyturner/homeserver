# homeserver

Single project for the home server: Docker Compose stacks for third-party services, plus a Go monorepo for services custom-built for this setup. Runs on a Mac Mini 2014 (Fedora Server, headless) — replaces the previous ASUS Tinker Board setup, quieter and safer to leave always-on.

## Services

1. Plex — media source, DLNA server enabled (Settings > Network > Enable DLNA Server). Replaces MiniDLNA, whose `lscr.io/linuxserver/minidlna` image was retired upstream (registry now returns "denied" — the repo no longer exists).
2. Pi-hole — DNS/ad blocking
3. SFTP (host-level, not containerized) + [filebrowserNEXT](https://filebrowsernext.github.io/filebrowserNEXT/) (dockerized, replaces the deprecated upstream Filebrowser) — file up/download
4. Portainer CE (Community Edition, free) — web UI for managing the containers above
5. `nowplaying` — custom Go service; DLNA now-playing display (see below)
6. `info-server` — custom Go service; gRPC streaming source for the Tinker Board's weather/crypto/news dashboard (see below)

### Plex notes

- First run needs a one-time manual step: claim the server via its web UI (`http://<host>:32400/web`), either signing in or setting `PLEX_CLAIM` (a token from https://plex.tv/claim, valid ~4 minutes) beforehand.
- When adding a library folder in the Plex UI, use the **container** path (`/music`), not whatever host path your OS shows the drive mounted at — they're not the same filesystem view, and Plex will silently scan 0 items if pointed at a path that doesn't exist inside its own container.
- (Historical, dev box only) If the media lives on an **exFAT** drive: exFAT can't hold extended attributes, so SELinux can't label individual files on it, and `:z`/`:Z` bind-mount relabeling is a silent no-op there. Under SELinux enforcing, a container will get "Permission denied" reading it — even as root — regardless of that flag. Fix used here: `security_opt: [label:disable]` on the Plex service, which skips SELinux label enforcement for that one container, rather than trying to relabel a filesystem that structurally can't support it. Mount the drive itself normally (no special SELinux mount options needed).

## Docker Compose

One `docker-compose.yml` at the repo root holds all services. Per-service config/data lives under `./config/<service>`.

Copy `.env.example` to `.env` and adjust for the host before running:

```
docker compose up -d
```

Music lives at `/home/paul/Music` on the Mac Mini's boot drive (XFS, so plain `:z` SELinux relabeling works). `/home/paul/content:/srv` (filebrowserNEXT) is still a placeholder.

## Go monorepo

Single `go.mod` at the repo root holds every custom Go service:

- `cmd/nowplaying` — polls a DLNA/UPnP MediaRenderer's `AVTransport` service and serves now-playing metadata + album art as JSON (`/api/now-playing`) and a small auto-updating web page. Currently displayed on an old Android phone via Chrome "Add to Home Screen" (fullscreen, no browser chrome, no root needed). Target renderer is env-driven (`RENDERER_CONTROL_URL` / `RENDERER_SERVICE_TYPE`) — swapping devices is just changing those two values, no code change. Some cheap embedded renderers have flaky SOAP servers under polling load (occasional connection resets) — the poller's `stale` field in the API response reflects this honestly rather than masking it.
- `internal/dlna` — shared UPnP AV control-point client (SOAP calls, DIDL-Lite parsing) used by `nowplaying` and available to future services.
- `cmd/info-server` — gRPC streaming source for weather, crypto, and news. Three separate proto services/streams (`proto/weather`, `proto/crypto`, `proto/news`, generated into `gen/`), each on its own poll cadence (weather/news 15 min, crypto 30 min), so a display client can subscribe to just what it needs and each updates independently. One background poll loop per category fans out to any number of connected clients via `internal/broadcast`, so multiple displays never multiply upstream API calls. Weather condition codes and moon phase are classified server-side into proto enums (`Category`, `MoonPhase`) — clients never see wttr.in's raw codes.
- `cmd/info-client` — the display side: a bubbletea TUI (moved from the old standalone `~/dev/info` project) that subscribes to all three `info-server` streams and renders them, reconnecting automatically if a stream drops. Board temperature is read locally (`/sys/class/thermal`) since it's specific to whatever hardware the client runs on, not server data. Not containerized — cross-compiled (`GOOS=linux GOARCH=arm GOARM=7`) and deployed straight to the Tinker Board's console, same as the original TUI.

Each `cmd/<service>` has its own `Dockerfile`, built with the repo root as context (so it can reach `go.mod`, `go.sum`, `internal/`, and `gen/`) — see `nowplaying` or `info-server` in `docker-compose.yml` for the pattern. Adding a new Go service means a new `cmd/<name>/` directory, a `Dockerfile` following that pattern, and (if it's meant to run as a container rather than deployed as a bare binary) a service block in `docker-compose.yml`.

### Regenerating protobuf/gRPC code

Requires `protoc` plus the `protoc-gen-go` and `protoc-gen-go-grpc` plugins (`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`, then make sure `$(go env GOPATH)/bin` is on `PATH`):

```
protoc -I proto \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  proto/weather/weather.proto proto/crypto/crypto.proto proto/news/news.proto
```
