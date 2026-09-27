# Changelog

Notable changes to this repo, newest first. Versions are git tags, deployed
with `./deploy.sh [tag]`.

## 1.3.2 - 2026-09-27

### Fixed
- info-gui: no mouse pointer on the Tinker Board's screen. cage draws one even
  with no mouse attached, so `info-gui.service` points it at a cursor theme
  (`cmd/info-gui/cursors/blank`, installed to `/usr/local/share/icons`) whose
  cursor is a single transparent pixel.

## 1.3.1 - 2026-09-27

### Changed
- info-gui: the wind panel's heading is now WIND, with the Beaufort name
  moved under the direction and speed, so longer names ("Moderate breeze")
  are no longer cut off.

## 1.3.0 - 2026-09-27

### Added
- info-gui: a line under the location with the client and server versions
  and the board temperature, which turns amber at 70C and red at 80C.
- info-server: sends its version as `server-version` gRPC header metadata on
  every stream. Both binaries are stamped with `git describe` at build time
  (`build.sh` for info-gui; `VERSION`, set by `deploy.sh`, for info-server's
  image).

## 1.2.0 - 2026-09-27

### Added
- nowplaying: the music source and renderer ("from Plex Media Server: fedora",
  "on S10+_2748") in small text at the bottom right of the text area. Names
  are the devices' UPnP friendly names, read from their device descriptions
  (the source is found from the track's URL). A source without one shows its
  host instead.
- nowplaying: `source` and `renderer` fields in `/api/now-playing`.

## 1.1.1 - 2026-09-27

### Added
- This changelog.

## 1.1.0 - 2026-09-27

### Added
- info-gui: a SUN panel showing the sun's position on a semicircle between
  today's sunrise (left) and sunset (right), with the next event underneath:
  today's sunrise before dawn, sunset during the day, tomorrow's sunrise after
  dark.
- info-gui: a WIND panel with a compass whose pointer shows the direction the
  wind comes from, the Beaufort name above (e.g. "Gentle breeze") and direction
  and speed below.
- info-server: `wind_beaufort`, `tomorrow_sunrise` and `timezone` fields on
  `WeatherUpdate`. The time zone is looked up from wttr.in's coordinates via
  timeapi.io.
- info-gui: `-time HH:MM` to render a `-screenshot` at a given time of day.

### Changed
- info-gui: the weather band is now NOW | TOMORROW over MOON | SUN | WIND. The
  sunrise/sunset line under MOON and the wind line under NOW moved into the
  new panels.
- info-gui: the clock, date and sun follow the weather location's time zone
  rather than the device's, so the display can run on UTC anywhere.
