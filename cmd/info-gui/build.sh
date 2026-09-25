#!/usr/bin/env bash
# Builds/renders info-gui inside the Dockerfile.build container.
#   ./build.sh amd64             -> bin/info-gui-amd64 (run locally under Wayland)
#   ./build.sh arm               -> bin/info-gui-arm   (Tinker Board, armv7l)
#   ./build.sh shot [args...]    -> renders one 800x480 frame to bin/shot.png
#                                   (extra args go to info-gui, e.g. -demo)
set -euo pipefail
cd "$(dirname "$0")/../.."
image=homeserver-info-gui-build
docker build -q -t "$image" -f cmd/info-gui/Dockerfile.build cmd/info-gui >/dev/null
mkdir -p cmd/info-gui/bin

run() {
	# label=disable rather than :z — relabeling the repo would clobber the
	# private SELinux labels on config/ that the running services rely on.
	docker run --rm --network host --security-opt label=disable -u "$(id -u):$(id -g)" \
		-e HOME=/tmp -e GOCACHE=/cache/build -e GOMODCACHE=/cache/mod -e TZ="${TZ:-Europe/London}" \
		-v "$PWD:/src" -v homeserver-go-cache:/cache -w /src "$image" "$@"
}
docker volume create homeserver-go-cache >/dev/null
docker run --rm -v homeserver-go-cache:/cache "$image" chown "$(id -u):$(id -g)" /cache

tags=nox11,novulkan
case "${1:-}" in
amd64)
	run go build -tags "$tags" -o cmd/info-gui/bin/info-gui-amd64 ./cmd/info-gui
	;;
arm)
	run env CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc \
		PKG_CONFIG_LIBDIR=/usr/lib/arm-linux-gnueabihf/pkgconfig:/usr/share/pkgconfig \
		go build -tags "$tags" -trimpath -ldflags "-s -w" -o cmd/info-gui/bin/info-gui-arm ./cmd/info-gui
	;;
shot)
	shift
	run env EGL_PLATFORM=surfaceless LIBGL_ALWAYS_SOFTWARE=1 \
		go run -tags "$tags" ./cmd/info-gui -screenshot cmd/info-gui/bin/shot.png "$@"
	;;
*)
	echo "usage: $0 amd64|arm|shot [args...]" >&2
	exit 2
	;;
esac
