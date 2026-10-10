#!/bin/sh
set -eu
pilot_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$pilot_root"
mkdir -p .tmp .cache/go-build
export TMPDIR="$pilot_root/.tmp"
export GOCACHE="$pilot_root/.cache/go-build"
export GOTOOLCHAIN=local GOWORK=off GOENV=off GOPROXY=off GOSUMDB=off
export GOOO_LAYA_URL= GOOO_LAYA_API_KEY=
pilot_go=${PILOT_GO_BIN:-go}
pilot_version=$("$pilot_go" version)
case "$pilot_version" in
  'go version go1.27.2 '*) ;;
  *) printf 'pilot: require installed Go 1.27.2; found %s. Set PILOT_GO_BIN to its executable.\n' "$pilot_version" >&2; exit 1 ;;
esac
exec "$pilot_go" run . "$@"
