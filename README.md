# homeserver

Single project for the home server: Docker Compose stacks for third-party services, plus a Go monorepo for services custom-built for this setup. Runs on a Mac Mini 2014 (Fedora Server, headless) — replaces the previous ASUS Tinker Board setup, quieter and safer to leave always-on.

## Services

1. MiniDLNA — media serving
2. Pi-hole — DNS/ad blocking
3. SFTP (host-level, not containerized) + Filebrowser (dockerized) — file up/download
4. Portainer CE (Community Edition, free) — web UI for managing the containers above
5. `nowplaying` — custom Go service; DLNA now-playing display (see below)

## Docker Compose

One `docker-compose.yml` at the repo root holds all services. Per-service config/data lives under `./config/<service>`.

Copy `.env.example` to `.env` and adjust for the host before running:

```
docker compose up -d
```

`/home/paul:/media` and `/home/paul/content:/srv` are placeholders until real paths on the Mac Mini are decided.

## Go monorepo

Single `go.mod` at the repo root holds every custom Go service:

- `cmd/nowplaying` — polls a DLNA/UPnP MediaRenderer's `AVTransport` service and serves now-playing metadata + album art as JSON (`/api/now-playing`) and a small auto-updating web page. Currently displayed on an old Android phone via Chrome "Add to Home Screen" (fullscreen, no browser chrome, no root needed). Target renderer is env-driven (`RENDERER_CONTROL_URL` / `RENDERER_SERVICE_TYPE`) — currently pointed at a temporary software renderer on the dev box until real hardware is in place.
- `internal/dlna` — shared UPnP AV control-point client (SOAP calls, DIDL-Lite parsing) used by `nowplaying` and available to future services.

Each `cmd/<service>` has its own `Dockerfile`, built with the repo root as context (so it can reach `go.mod` and `internal/`) — see the `nowplaying` entry in `docker-compose.yml` for the pattern. Adding a new Go service means a new `cmd/<name>/` directory, a `Dockerfile` following that pattern, and a service block in `docker-compose.yml`.

Planned next: `cmd/info-server` + `cmd/info-client`, splitting the Tinker Board's weather/crypto/news dashboard into a gRPC streaming source and a thin display client, so new display devices don't require re-doing the data-gathering logic.
