# homeserver

Config and Docker Compose stacks for the home server running on a Mac Mini 2014 (Fedora Server, headless).

Replaces the previous ASUS Tinker Board setup — quieter, safer to leave always-on.

## Services

Built one at a time, in this order:

1. MiniDLNA — media serving
2. Pi-hole — DNS/ad blocking
3. SFTP (host-level, not containerized) + Filebrowser (dockerized) — file up/download
4. Portainer CE (Community Edition, free) — web UI for managing the containers above

## Layout

One `docker-compose.yml` at the repo root holds all services, added one at a time. Per-service config/data lives under `./config/<service>`.

Copy `.env.example` to `.env` and adjust `PUID`/`PGID`/`TZ` for the host before running.

Currently MiniDLNA, Pi-hole, Filebrowser, and nowplaying are defined. `/home/paul:/media` and `/home/paul/content:/srv` are placeholders until real paths on the Mac Mini are decided.

This repo also holds a Go monorepo (single `go.mod` at the root) for services custom-built for this setup:

- `cmd/nowplaying` — polls a DLNA/UPnP MediaRenderer's AVTransport service and serves now-playing metadata + album art as JSON and a small auto-updating web page (see `internal/dlna`)
- `internal/dlna` — shared UPnP AV control-point client (SOAP calls, DIDL-Lite parsing)

Each `cmd/<service>` has its own `Dockerfile`, built with the repo root as context (needed so it can reach `go.mod` and `internal/`) — see the `nowplaying` service in `docker-compose.yml` for the pattern.

```
docker compose up -d
```
