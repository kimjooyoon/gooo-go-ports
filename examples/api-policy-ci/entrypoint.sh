#!/usr/bin/env bash
set -euo pipefail
umask 077

# Caller text is carried only in environment/file arguments, never shell source.
if [[ "${RUNNER_OS:-}" != Linux || -z "${RUNNER_TEMP:-}" || -z "${GITHUB_ACTION_PATH:-}" || -z "${GITHUB_WORKSPACE:-}" || -z "${GITHUB_OUTPUT:-}" ]]; then
  printf '%s\n' 'Gooo policy action requires a Linux runner and the normal GitHub action environment.' >&2
  exit 2
fi
case "${GOOO_ACTION_MODE:-}" in compare|replay) ;; *) printf '%s\n' 'Invalid action mode.' >&2; exit 2 ;; esac
case "${GOOO_ACTION_FAIL_NEWLY_DENIED:-}" in true|false) ;; *) printf '%s\n' 'Invalid fail-on-newly-denied setting.' >&2; exit 2 ;; esac

unset GOFLAGS GOOS GOARCH GOROOT GOPRIVATE GONOPROXY GONOSUMDB GOCACHE GOMODCACHE GOTMPDIR
unset GH_TOKEN GITHUB_TOKEN GOOO_LAYA_URL GOOO_LAYA_API_KEY
export GOTOOLCHAIN=local GOWORK=off GOENV=off CGO_ENABLED=0 GOMAXPROCS=2
export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
if [[ "$(go env GOVERSION)" != go1.27.2 ]]; then
  printf '%s\n' 'Select Go 1.27.2 before this action; automatic toolchain installation is disabled.' >&2
  exit 2
fi

action_work="$(mktemp -d "$RUNNER_TEMP/gooo-api-policy-action.XXXXXXXX")"
chmod 700 "$action_work"
export GOOO_ACTION_WORK="$action_work"
export GOCACHE="$action_work/build-cache" GOMODCACHE="$action_work/module-cache"
export GOTMPDIR="$action_work/go-tmp" TMPDIR="$action_work/go-tmp"
mkdir -m 700 "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$action_work/bin"
# Preserve result files (or the first failed attempt) for the rest of this job.
# This pilot deliberately does not upload, post, or delete any caller files.
timeout --signal=TERM --kill-after=5s 180s go -C "$GITHUB_ACTION_PATH" build -mod=readonly -trimpath -o "$action_work/bin/review" ./cmd/gooo-policy-action
timeout --signal=TERM --kill-after=5s 60s env PATH='' "$action_work/bin/review"
