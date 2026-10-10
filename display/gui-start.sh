#!/bin/sh
# Runs the app, and stops it if sway's socket goes away or is replaced (sway
# restarted): info-gui can sit forever on a dead connection otherwise, and
# compose only restarts it once it exits.
sock="$XDG_RUNTIME_DIR/$WAYLAND_DISPLAY"
until [ -S "$sock" ]; do sleep 1; done
ino=$(stat -c %i "$sock")
"/usr/local/bin/$APP" -kiosk &
pid=$!
trap 'kill $pid' TERM INT
while kill -0 $pid 2>/dev/null; do
	sleep 2
	if [ "$(stat -c %i "$sock" 2>/dev/null)" != "$ino" ]; then
		echo "gui-start: sway's socket changed, restarting" >&2
		kill $pid
		exit 1
	fi
done
wait $pid
