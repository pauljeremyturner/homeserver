# homeserver

Single project for the home server: Docker Compose stacks for third-party services, plus a Go monorepo for services custom-built for this setup. Runs on a Mac Mini 2014 (Fedora Server, headless) — replaces the previous ASUS Tinker Board setup, quieter and safer to leave always-on.

## Services

1. Plex — media source, DLNA server enabled (Settings > Network > Enable DLNA Server). Replaces MiniDLNA, whose `lscr.io/linuxserver/minidlna` image was retired upstream (registry now returns "denied" — the repo no longer exists).
2. Pi-hole — DNS/ad blocking
3. SFTP (host-level, not containerized) + [filebrowserNEXT](https://filebrowsernext.github.io/filebrowserNEXT/) (dockerized, replaces the deprecated upstream Filebrowser) — file up/download
4. Portainer CE (Community Edition, free) — web UI for managing the containers above
5. `nowplaying` — custom Go service; DLNA now-playing display (see below)
6. `info-server` — custom Go service; gRPC streaming source for the Tinker Board's weather/crypto/news dashboard (see below)
7. `control` — custom Go service; phone page with a Shutdown button for the whole setup (see below)

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

### Deploying

Releases are git tags. To deploy one on the server:

```
./deploy.sh          # newest tag
./deploy.sh 1.0.0    # a specific tag or commit
```

It fetches, checks out the tag (detached), and runs `docker compose up -d --build --remove-orphans`, which only recreates services whose image or config changed. `.env` and the runtime state under `config/` are untracked, so `git checkout` never touches them and they carry over as-is. The script refuses to run if `.env` is missing or a tracked file has local edits.

Music lives at `/home/paul/Music` on the Mac Mini's boot drive (XFS, so plain `:z` SELinux relabeling works). `/home/paul/content:/srv` (filebrowserNEXT) is still a placeholder.

## Router DNS (Pi-hole)

The ZTE MC888 5G router can't hand out a custom DNS server over DHCP: clients always get the router itself, which forwards to the mobile network's DNS. Its one DNS setting lives in a manual APN profile (and is hidden in the web UI), so `router/pihole-dns.py` (Python 3, no dependencies) drives the router's web API to use it:

```
router/pihole-dns.py status   # APN mode, DNS, connection; checks whether ads are blocked
router/pihole-dns.py enable   # router forwards all DNS to Pi-hole
router/pihole-dns.py undo     # back to automatic APN: router manages DNS again
```

- `enable` copies the operator's automatic APN profile into a manual one named "Pi-hole DNS" with `PIHOLE_DNS` as its only DNS server, and makes it active. It first checks Pi-hole answers, and refuses if the router is already on some other manual profile.
- `undo` switches back to automatic APN and deletes that profile. Run it whenever anything looks wrong; it only needs the LAN, not the internet.
- Both drop mobile data for roughly 10-30s, as the router only changes APN while disconnected.
- If the server running Pi-hole is down while enabled, nothing on the LAN can resolve names: run `undo`.
- Pi-hole sees every query as coming from the router, so its per-client stats show only the router.

Configure with environment variables or `router/.env` (gitignored; copy `router/.env.example`): `ROUTER_ADDR`, `ROUTER_USER`, `ROUTER_PASSWORD`, `PIHOLE_DNS`.

## Go monorepo

Single `go.mod` at the repo root holds every custom Go service:

- `cmd/nowplaying` — polls a DLNA/UPnP MediaRenderer's `AVTransport` service and serves now-playing metadata + album art as JSON (`/api/now-playing`) and a small auto-updating web page. Currently displayed on an old Android phone via Chrome "Add to Home Screen" (fullscreen, no browser chrome, no root needed). Target renderer is env-driven (`RENDERER_CONTROL_URL` / `RENDERER_SERVICE_TYPE`) — swapping devices is just changing those two values, no code change. Some cheap embedded renderers have flaky SOAP servers under polling load (occasional connection resets) — the poller's `stale` field in the API response reflects this honestly rather than masking it.
- `internal/dlna` — shared UPnP AV control-point client (SOAP calls, DIDL-Lite parsing) used by `nowplaying` and available to future services.
- `cmd/info-server` — gRPC streaming source for weather, crypto, and news. Three separate proto services/streams (`proto/weather`, `proto/crypto`, `proto/news`, generated into `gen/`), each on its own poll cadence (weather/news 15 min, crypto 30 min), so a display client can subscribe to just what it needs and each updates independently. One background poll loop per category fans out to any number of connected clients via `internal/broadcast`, so multiple displays never multiply upstream API calls. Weather is for `LOCATION` in `.env` (`lat,lon`, with optional `LOCATION_NAME` for the place name shown), or, if that's empty, wherever wttr.in places the server's IP, which is only rough and can be far off behind a 5G router. Weather condition codes and moon phase are classified server-side into proto enums (`Category`, `MoonPhase`) — clients never see wttr.in's raw codes.
- `cmd/info-client` — the display side: a bubbletea TUI (moved from the old standalone `~/dev/info` project) that subscribes to all three `info-server` streams and renders them, reconnecting automatically if a stream drops. Board temperature is read locally (`/sys/class/thermal`) since it's specific to whatever hardware the client runs on, not server data. Not containerized — cross-compiled (`GOOS=linux GOARCH=arm GOARM=7`) and deployed straight to the Tinker Board's console, same as the original TUI.
- `cmd/photo-server` — streams a random photo from `/home/paul/Pictures` (or `PHOTOS_DIR` in `.env`, which holds copies, never the originals; `Screenshots` and hidden folders skipped) to photo displays over gRPC (`proto/photo`, port 9092), a new one every `PHOTO_SECONDS` (default 60). Every photo is shown once, in random order, before any repeats; photos added meanwhile are shown next. Photos are turned upright by their EXIF orientation and shrunk to the size each display asks for, so a small board never decodes a full-size photo. JPEG, PNG, GIF, WebP, TIFF and BMP; not HEIC. It also serves a phone-sized upload page on port 8092 (`http://<host>:8092`): pick one or many photos from the phone's gallery and they're saved to `Uploads/` in the photos folder (as `PUID`/`PGID`, time-stamped so phones' repeated `image.jpg` never overwrite each other), each checked to be a photo it can show (up to `PHOTO_UPLOAD_MAX_MB`, default 100). HEIC is refused: on an iPhone, Settings > Camera > Formats > Most Compatible, though Safari usually sends JPEG anyway.
- `cmd/photo-gui` — the photo frame: a Gio app run full screen under cage (like `info-gui`), fading from one photo to the next. `cmd/photo-gui/build.sh arm` builds it for the Tinker Board; `photo-gui.service` runs it on tty1 in place of `info-gui`.
- `cmd/control` — a phone-sized page on port 8093 (`http://<host>:8093`) with one **Shutdown** button (and an are-you-sure). It powers off each host in `CONTROL_HOSTS` (`.env`, comma-separated `root@address`) in turn by running `poweroff` as root over SSH, waiting for each to stop answering before the next, and shows how each one is getting on. List the display box(es) first and this server last, since powering it off ends `control` too. `poweroff` stops Docker and the display apps cleanly on the way down, so there's no separate stop step. A host that's already off counts as off; one that's still up after 90s is marked failed and skipped. `CONTROL_DRY_RUN=1` runs `true` instead of `poweroff`, to try it safely. One-time setup: on first start it makes an SSH key in `config/control/` and logs the public half (`docker logs control`); add that line to `/root/.ssh/authorized_keys` on every host in the list, this server included. It's named `control` rather than `shutdown` so more actions can go on the page later.

Each `cmd/<service>` has its own `Dockerfile`, built with the repo root as context (so it can reach `go.mod`, `go.sum`, `internal/`, and `gen/`) — see `nowplaying` or `info-server` in `docker-compose.yml` for the pattern. Adding a new Go service means a new `cmd/<name>/` directory, a `Dockerfile` following that pattern, and (if it's meant to run as a container rather than deployed as a bare binary) a service block in `docker-compose.yml`.

### Regenerating protobuf/gRPC code

Requires `protoc` plus the `protoc-gen-go` and `protoc-gen-go-grpc` plugins (`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`, then make sure `$(go env GOPATH)/bin` is on `PATH`):

```
protoc -I proto \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  proto/weather/weather.proto proto/crypto/crypto.proto proto/news/news.proto
```
