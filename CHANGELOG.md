# Changelog

Notable changes to this repo, newest first. Versions are git tags, deployed
with `./deploy.sh [tag]`.

## 1.9.0 - 2026-09-30

### Added
- info-gui: a second markets page, the same four charts over six months
  (daily prices, months along the bottom, change marked "6m"), shown after
  the week's.
- info-gui and photo-gui: a line along the bottom of every page (and
  photo) counting down its time on screen, white for what's left and grey
  from the right for what's gone, worked out on the display from the
  page's times (`internal/countdown`), redrawn each time it moves a pixel.
- info-server: the page rotation is weather 60s, markets (week) 30s,
  markets (six months) 30s, planets 60s: `PAGE_SECONDS` (compose
  `INFO_PAGE_SECONDS`) now sets the weather and planets pages only, and
  the new `MARKETS_PAGE_SECONDS` (`INFO_MARKETS_PAGE_SECONDS`, default 30)
  each markets page. `PageUpdate.cycle` carries the whole rotation, so a
  display that loses the server keeps to it (`internal/pagecycle`).
- proto: `StreamCryptoRequest.days` picks the window of prices, 7 (the
  default, so older clients are unchanged) or 180, and `CryptoUpdate.days`
  says which; `PAGE_MARKETS_6M` is the new page.

### Changed
- info-server: CoinGecko is asked one request at a time, 10s apart, with
  throttled requests retried after 2 minutes (up to 3 tries), as eight
  requests at once got throttled; a non-200 reply is now logged as such.

### Fixed
- info-server: an hour Open-Meteo codes as rain, snow or thunder but gives
  under a 20% chance and no amount of is forecast as cloud, so a 3% chance
  no longer shows a rain icon in the hourly strip or the week's day and
  night icons.

## 1.8.0 - 2026-09-30

### Added
- info-gui: the MOON block has its phase name and how much is lit under
  the moon, and beside it an arc like the sun's: the Moon, drawn in its
  phase, crossing from moonrise to moonset, or the next moonrise while
  it's down.
- info-server: `moon_passes` on `WeatherUpdate`, the Moon's rise and set
  times from the pass under way through the next two days, computed at the
  weather location by `internal/moon` (Astronomical Almanac low-precision
  formulae; within 3 minutes of the US Naval Observatory in tests,
  including moonrise-less days and passes over midnight).

## 1.7.0 - 2026-09-30

### Added
- photo-server: a new compose service streaming a random photo from
  `/home/paul/Pictures` (`PHOTOS_DIR` in `.env` overrides it; skipping
  `Screenshots` and hidden folders) every
  `PHOTO_SECONDS` (default 60), upright and shrunk to each display's size,
  every photo once before any repeats.
- photo-gui: a photo frame for the Tinker Board under cage, fading between
  photos; `photo-gui.service` runs it on tty1 in place of info-gui.

## 1.6.0 - 2026-09-30

### Added
- info-gui: the weather page has a THIS WEEK list (7 days: chance of rain,
  day and night icons, low and high on a bar across the week's range, with
  the current temperature marked on today's) and a NEXT 12 HOURS strip
  along the bottom (time, icon, temperature on a line, chance of rain).
  Clear and partly cloudy skies get a crescent moon at night. NOW is bigger
  and shows the UV index.
- info-server: `hourly` (next 24 hours) and `daily` (7 days) forecasts and
  `is_day` on `WeatherUpdate`, from Open-Meteo (free, no key) at wttr.in's
  coordinates. Current conditions come from Open-Meteo too, so they agree
  with the forecast, falling back to wttr.in's if Open-Meteo fails.
- info-gui: a PLANETS page: the inner (Mercury to Mars) and outer (Jupiter
  to Neptune) planets side by side, each to true scale with their real
  elliptical orbits, animating through the last 20 years (easing in and
  out, with the year and date) over two thirds of the page's time, then
  holding on today. Inner planets trail their last two months; outer
  planets trail everywhere they've been. `-page 4 -anim 20s` screenshots it
  part way through.
- info-server: `PlanetsUpdate` carries each planet's x/y, its orbit's
  shape, and its history: ~1800 positions evenly spread over
  `PLANETS_YEARS` (compose `INFO_PLANETS_YEARS`, default 20) up to now.
  `internal/planets` gains `Orbits` and `History`.

### Changed
- info-server/info-gui: pages cycle weather, markets, planets, a minute
  each by default (was 30s).
- info-gui: designed for a 1024x600 screen, drawn 1:1 there (was 800x480
  scaled to fit).
- info-gui: the solar system dial is off the weather page, replaced by the
  PLANETS page.
- info-gui/info-server: the METALS page is merged into MARKETS, which now
  has four charts (BTC and ETH above, gold and silver below) with the
  wallet and news under them. `-page 3` is gone, and a `PAGE_METALS` from
  an older server shows the markets page.

### Removed
- info-server: the `tomorrow_*` fields of `WeatherUpdate` (except
  `tomorrow_sunrise`); `daily[1]` replaces them. info-client's tomorrow
  line uses it.

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
