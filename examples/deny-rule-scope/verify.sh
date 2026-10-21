#!/usr/bin/env bash
# Copyright 2026 Gooo Go Ports contributors. BSD-3-Clause.
set -euo pipefail
umask 077
if [ "$#" -ne 3 ]; then
  echo "Usage: verify.sh /absolute/compiler /absolute/go /absolute/new/output" >&2
  exit 2
fi
compiler="$1"
go_sdk="$2"
out="$3"
for executable in "$compiler" "$go_sdk"; do
  case "$executable" in /*) ;; *) echo "Executable paths must be absolute" >&2; exit 2 ;; esac
  [ -x "$executable" ] || { echo "Executable is unavailable: $executable" >&2; exit 2; }
done
here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../.." && pwd)"
python3 - "$root" "$out" <<'PY'
from pathlib import Path
import sys
root, out = map(Path, sys.argv[1:])
if (not out.is_absolute() or out.exists() or out.is_symlink()
        or out.resolve().is_relative_to(root.resolve())):
    raise SystemExit("Output must be a fresh absolute directory outside the checkout")
out.mkdir(mode=0o700)
PY
stage=identity
on_error() {
  status="$?"
  echo "Failed at $stage; original output and stderr retained in $out" >&2
  if [ -f "$out/$stage.stderr" ]; then cat "$out/$stage.stderr" >&2; fi
  exit "$status"
}
trap on_error ERR
cd "$root"
export PATH="$(dirname "$go_sdk"):$PATH"
export GOTOOLCHAIN=local GOWORK=off GOPROXY=off GOSUMDB=off GOTELEMETRY=off
export GOMAXPROCS=4 GOFLAGS=-p=4
"$compiler" version --build --json > "$out/identity.json" 2> "$out/$stage.stderr"
stage=identity-check
python3 -I -B "$here/check.py" --identity "$out/identity.json" --identity-only > "$out/identity-check.json" 2> "$out/$stage.stderr"
stage=input-generation
python3 -I -B "$here/make-inputs.py" --out "$out/inputs" > "$out/input-generation.json" 2> "$out/$stage.stderr"
stage=reference
"$go_sdk" run "$here/reference/main.go" "$out/inputs/oracle-inputs.json" > "$out/reference.json" 2> "$out/$stage.stderr"
stage=execution
"$compiler" package execute --json --inputs "$out/inputs/inputs.json" "$root/deny-rule-scope.workspace.json" > "$out/execution.json" 2> "$out/$stage.stderr"
stage=replay
"$compiler" package replay --json --receipt "$out/execution.json" --inputs "$out/inputs/inputs.json" "$root/deny-rule-scope.workspace.json" > "$out/replay.json" 2> "$out/$stage.stderr"
stage=verification
python3 -I -B "$here/check.py" --identity "$out/identity.json" --execution "$out/execution.json" \
  --replay "$out/replay.json" --expected "$out/reference.json" --generated-inputs "$out/inputs" \
  --go "$go_sdk" --out "$out/verification.json" > "$out/verification.stdout" 2> "$out/$stage.stderr"
cat "$out/verification.stdout"
