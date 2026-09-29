# Changelog

Notable changes to this repo, newest first. Versions are git tags, deployed
with `./deploy.sh [tag]`.

## 1.5.0 - 2026-09-29

### Added
- info-gui: a third page, METALS, with gold (XAU / GBP) and silver
  (XAG / GBP) charts like the crypto ones, and the news ticker below.
  Prices under £1,000 at a small scale show pence (e.g. silver at £48.82).
  `-page 3` screenshots it.
- info-server: streams gold and silver per troy ounce via CoinGecko's
  PAX Gold and Kinesis Silver tokens (each backed by an ounce of the metal),
  as `CryptoUpdate`s with symbols XAU and XAG; `PAGE_METALS` in the page
  cycle.

### Changed
- info-server/info-gui: pages cycle weather, markets, metals, each shown for
  30s by default (was 60s; compose `INFO_PAGE_SECONDS`).

## 1.4.2 - 2026-09-27

### Changed
- info-gui: the solar system dial names each planet in full beside it, on
  its side away from the Sun, flipping sides where a name would run off the
  edge, in place of initials.

## 1.4.1 - 2026-09-27

### Changed
- info-gui: the crypto charts have a price axis of 7 round-number labels
  (e.g. £60,000 to £66,000 in £1,000 steps) with gridlines, in place of just
  the week's high and low.

## 1.4.0 - 2026-09-27

### Added
- info-gui: two pages under the clock, switching every minute with a fade
  out and in (the header stays put). Page 1 is the weather: NOW, TOMORROW
  and MOON, then SUN and WIND, then a SOLAR SYSTEM dial showing the planets
  (with their initials) on evenly spaced orbits seen from above the north
  pole. Page 2 is the markets: large BTC and ETH charts with the week's high
  and low and the days marked, the wallet on one line, and the news ticker.
- info-server: `PlanetService.StreamPlanets`, the planets' heliocentric
  positions every hour, computed with JPL's approximate orbital elements
  (`internal/planets`, tested against JPL Horizons).
- info-server: `DisplayService.StreamPage`, which page displays should show,
  switching every `PAGE_SECONDS` (compose: `INFO_PAGE_SECONDS`, default 60)
  on the minute. info-gui follows it, and keeps cycling on its own clock if
  the server goes quiet.
- info-gui: `-page 1|2` to screenshot a given page.

### Changed
- info-server: crypto prices are now hourly over the 7 days rather than one
  per day, with their times in the new `times_unix` field.
- info-gui: the news ticker only runs on the markets page, so the board is
  mostly idle while the weather page shows.

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
