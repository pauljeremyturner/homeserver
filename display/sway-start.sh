#!/bin/sh
# Writes which output each app goes on and how each screen is rotated
# (sway's config can't read env vars), then starts sway on the shared
# socket dir.
set -eu
# compose passes these even when .env leaves them out, and wlroots reads an
# empty WLR_BACKENDS as "no backends" rather than "pick one".
[ -n "${WLR_BACKENDS:-}" ] || unset WLR_BACKENDS
[ -n "${WLR_HEADLESS_OUTPUTS:-}" ] || unset WLR_HEADLESS_OUTPUTS
mkdir -p "$XDG_RUNTIME_DIR"
chmod 0700 "$XDG_RUNTIME_DIR"
# A restarted container keeps the sockets of the last run, and seatd
# refuses to start over its old one.
rm -f "$XDG_RUNTIME_DIR"/wayland-* "$XDG_RUNTIME_DIR"/sway-ipc.* /run/seatd.sock
cat > /etc/sway/outputs.config <<CONF
workspace 1 output ${INFO_OUTPUT}
workspace 2 output ${PHOTO_OUTPUT}
output ${INFO_OUTPUT} transform ${INFO_TRANSFORM:-normal}
output ${PHOTO_OUTPUT} transform ${PHOTO_TRANSFORM:-normal}
CONF
exec seatd-launch -- sway -c /etc/sway/kiosk.config
