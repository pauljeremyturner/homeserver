# Changelog

Notable changes to this repo, newest first. Versions are git tags, deployed
with `./deploy.sh [tag]`.

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
