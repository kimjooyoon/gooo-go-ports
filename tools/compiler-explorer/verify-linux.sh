#!/usr/bin/env bash
set -euo pipefail

tool_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
ports_dir=$(cd "$tool_dir/../.." && pwd -P)
pins_file=$tool_dir/pins.json
evidence_dir=$ports_dir/out/compiler-explorer-linux
server_pid=
phase=preflight
result=FAIL

fail() { printf 'compiler-explorer Linux verification: %s\n' "$*" >&2; exit 1; }
pin() { python3 - "$pins_file" "$1" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))[sys.argv[2]])
PY
}

case "${1:-}" in
  '') [ "$#" -eq 0 ] || fail 'unexpected arguments' ;;
  --preflight) [ "$#" -eq 1 ] || fail 'unexpected arguments' ;;
  *) fail 'usage: verify-linux.sh [--preflight]' ;;
esac
[ "$(uname -s)" = Linux ] || fail 'requires Linux; no verification was run'
[ "$(uname -m)" = x86_64 ] || fail 'requires Linux x86_64; no verification was run'
for command in python3 git curl tar timeout node npm c++filt cmp tee mktemp; do
  command -v "$command" >/dev/null || fail "required command is missing: $command"
done
[ "$(node --version)" = "v$(pin node_version)" ] || fail 'Node version differs from pins.json'
[ "$(npm --version)" = "$(pin npm_version)" ] || fail 'npm version differs from pins.json'
demangler_version=$(c++filt --version)
[[ "$demangler_version" == *'GNU c++filt'* ]] || fail 'requires GNU c++filt on PATH'

python3 - "$tool_dir" <<'PY'
import hashlib, json, pathlib, re, sys
root = pathlib.Path(sys.argv[1])
pins = json.loads((root / 'pins.json').read_bytes())
assert pins['schema'] == 'gooo/compiler-explorer-linux-pins/v1'
assert len(pins['patched_files_sha256']) == 17
for revision in ('ce_revision', 'compiler_revision'):
    assert re.fullmatch('[0-9a-f]{40}', pins[revision]), revision
assert hashlib.sha256((root / 'gooo-source-view.patch').read_bytes()).hexdigest() == pins['patch_sha256']
for name, expected in pins['fixtures_sha256'].items():
    assert pathlib.PurePosixPath(name).name == name, name
    assert hashlib.sha256((root / 'fixtures' / name).read_bytes()).hexdigest() == expected, name
print('PASS: pinned patch and fixture hashes')
PY
if [ "${1:-}" = --preflight ]; then
  printf 'PASS: Linux preflight; no download, compiler execution or CE checks were run\n'
  exit 0
fi

[ ! -e "$evidence_dir" ] || fail 'out/compiler-explorer-linux already exists; preserve it before a new run'
mkdir -p "$evidence_dir/logs" "$evidence_dir/cli"
scratch_dir=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/gooo-ce-linux.XXXXXX")
ce_dir=$scratch_dir/compiler-explorer
compiler_dir=$scratch_dir/compiler
cli_dir=$evidence_dir/cli
mkdir -p "$compiler_dir"

write_summary() {
  python3 - "$evidence_dir" "$pins_file" "$result" "$phase" <<'PY'
import datetime, json, pathlib, sys
root, pins_path, result, phase = sys.argv[1:]
pins = json.loads(pathlib.Path(pins_path).read_bytes())
summary = {
    'schema': 'gooo/compiler-explorer-linux-proof/v1',
    'observed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'status': 'LINUX_ACTUAL_COMPILER_EXPLORER_PASS' if result == 'PASS' else 'LINUX_VERIFICATION_FAILED',
    'last_phase': phase,
    'ce_revision': pins['ce_revision'],
    'patch_sha256': pins['patch_sha256'],
    'compiler_revision': pins['compiler_revision'],
    'compiler_version': pins['compiler_version'],
    'external_hosted_support': False,
    'customer_adoption': False,
}
if result == 'PASS':
    api = json.loads((pathlib.Path(root) / 'cli/api-observation.json').read_bytes())
    assert len(api['cases']) == 11 and all(case['pass'] for case in api['cases'])
    summary['actual_compiler_api_cases'] = len(api['cases'])
    summary['checks'] = ['npm ci', 'npm run check', 'npm run test:props', 'npm run webpack']
(pathlib.Path(root) / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
PY
}

cleanup() {
  local exit_status=$?
  trap - EXIT HUP INT TERM
  if [ -n "$server_pid" ]; then
    if kill -0 "$server_pid" 2>/dev/null; then
      kill "$server_pid" 2>/dev/null || true
      for ((attempt = 0; attempt < 50; attempt++)); do
        kill -0 "$server_pid" 2>/dev/null || break
        sleep 0.1
      done
      if kill -0 "$server_pid" 2>/dev/null; then kill -KILL "$server_pid" 2>/dev/null || true; fi
    fi
    wait "$server_pid" 2>/dev/null || true
  fi
  if [ "$exit_status" -ne 0 ]; then result=FAIL; fi
  write_summary || exit_status=1
  exit "$exit_status"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

run_phase() {
  phase=$1
  local limit=$2
  shift 2
  printf 'Checking %s\n' "$phase"
  timeout --kill-after=10s "$limit" "$@" 2>&1 | tee "$evidence_dir/logs/$phase.log"
}

run_phase probe-safety 60s node --test "$tool_dir/api-probe.test.mjs"
cp "$pins_file" "$evidence_dir/pins.json"
cp "$tool_dir/api-probe.mjs" "$cli_dir/api-probe.mjs"
cp "$tool_dir/fixtures/"* "$cli_dir/"
python3 - "$evidence_dir" "$ports_dir" "$demangler_version" <<'PY'
import hashlib, json, os, pathlib, platform, subprocess, sys
root, source_root, demangler = sys.argv[1:]
command = lambda *args: subprocess.check_output(args, text=True).strip()
identity = {
    'schema': 'gooo/compiler-explorer-linux-runtime/v1',
    'system': platform.system(), 'architecture': platform.machine(),
    'kernel': platform.release(), 'node_version': command('node', '--version'),
    'npm_version': command('npm', '--version'), 'git_version': command('git', '--version'),
    'gnu_demangler_version': demangler,
    'ports_revision': command('git', '-C', source_root, 'rev-parse', 'HEAD'),
    'pins_sha256': hashlib.sha256((pathlib.Path(root) / 'pins.json').read_bytes()).hexdigest(),
    'api_probe_sha256': hashlib.sha256((pathlib.Path(root) / 'cli/api-probe.mjs').read_bytes()).hexdigest(),
    'github_run_id': os.environ.get('GITHUB_RUN_ID'),
    'github_run_attempt': os.environ.get('GITHUB_RUN_ATTEMPT'),
}
(pathlib.Path(root) / 'runtime.json').write_text(json.dumps(identity, indent=2) + '\n')
PY

ce_revision=$(pin ce_revision)
run_phase ce-init 30s git init "$ce_dir"
git -C "$ce_dir" remote add origin "$(pin ce_repository)"
run_phase ce-fetch 180s git -C "$ce_dir" fetch --depth=1 origin "$ce_revision"
run_phase ce-checkout 30s git -C "$ce_dir" checkout --detach "$ce_revision"
[ "$(git -C "$ce_dir" rev-parse HEAD)" = "$ce_revision" ] || fail 'CE checkout revision differs'
[ -z "$(git -C "$ce_dir" status --porcelain --untracked-files=all)" ] || fail 'CE base is not clean'
phase=patch
git -C "$ce_dir" apply --check "$tool_dir/gooo-source-view.patch"
git -C "$ce_dir" apply "$tool_dir/gooo-source-view.patch"
verify_source_hashes() {
  python3 - "$ce_dir" "$pins_file" "$evidence_dir/patched-source-sha256.json" <<'PY'
import hashlib, json, pathlib, sys
root, pins_file, output = map(pathlib.Path, sys.argv[1:])
pins = json.loads(pins_file.read_bytes())
actual = {}
for name, expected in pins['patched_files_sha256'].items():
    relative = pathlib.PurePosixPath(name)
    assert not relative.is_absolute() and '..' not in relative.parts, name
    actual[name] = hashlib.sha256((root / name).read_bytes()).hexdigest()
    assert actual[name] == expected, name
output.write_text(json.dumps(actual, indent=2) + '\n')
print('PASS: all 17 patched source hashes')
PY
}
verify_source_hashes | tee "$evidence_dir/logs/patched-source.log"

archive_name=$(pin compiler_archive)
archive_path=$compiler_dir/$archive_name
archive_url=$(pin compiler_repository)/releases/download/$(pin compiler_tag)/$archive_name
run_phase compiler-download 180s curl --fail --location --proto '=https' --tlsv1.2 \
  --connect-timeout 20 --max-time 150 --output "$archive_path" "$archive_url"
phase=compiler-archive
python3 - "$archive_path" "$pins_file" "$evidence_dir/compiler-archive.json" <<'PY'
import hashlib, json, pathlib, sys
archive, pins_file, output = map(pathlib.Path, sys.argv[1:])
pins = json.loads(pins_file.read_bytes())
actual = {'name': archive.name, 'bytes': archive.stat().st_size,
          'sha256': hashlib.sha256(archive.read_bytes()).hexdigest()}
assert actual['bytes'] == pins['compiler_archive_bytes']
assert actual['sha256'] == pins['compiler_archive_sha256']
output.write_text(json.dumps(actual, indent=2) + '\n')
PY
[ "$(tar -tzf "$archive_path")" = gooo ] || fail 'compiler archive members differ from the pinned layout'
tar -xzf "$archive_path" --no-same-owner --no-same-permissions -C "$compiler_dir"
compiler=$compiler_dir/gooo
[ -f "$compiler" ] && [ ! -L "$compiler" ] || fail 'compiler archive did not contain a regular gooo binary'
chmod +x "$compiler"
export GOOO_LAYA_URL='' GOOO_LAYA_API_KEY=''
phase=compiler-identity
timeout --kill-after=5s 30s "$compiler" version --build --json > "$evidence_dir/compiler-build.json" \
  2> "$evidence_dir/logs/compiler-build.stderr"
python3 - "$evidence_dir/compiler-build.json" "$pins_file" <<'PY'
import json, pathlib, sys
identity, pins = [json.loads(pathlib.Path(name).read_bytes()) for name in sys.argv[1:]]
assert identity['schema'] == 'gooo/build-identity/v1'
assert identity['version'] == pins['compiler_version']
assert identity['compiler_source_sha'] == identity['vcs_revision'] == pins['compiler_revision']
assert identity['source_status'] == 'CLEAN_VCS' and identity['vcs_modified'] == 'false'
assert identity['go_version'] == identity['native_go_required'] == pins['compiler_go_version']
print('PASS: clean pinned Linux compiler build identity')
PY

phase=cli
cd "$cli_dir"
for name in clamp changed selected-double; do
  source=$name.gooo
  activity=Clamp
  if [ "$name" = selected-double ]; then source=two-activities.gooo; activity=Double; fi
  timeout --kill-after=5s 30s "$compiler" body-codegen --activity "$activity" "$source" \
    > "$name.stdout" 2> "$name.stderr"
  cmp "$name.stdout" "$tool_dir/fixtures/$name.stdout"
done
for name in invalid-body malformed; do
  exit_code=0
  timeout --kill-after=5s 30s "$compiler" body-codegen --activity Clamp "$name.gooo" \
    > "$name.stdout" 2> "$name.stderr" || exit_code=$?
  [ "$exit_code" -gt 0 ] && [ "$exit_code" -lt 124 ] || fail "$name did not produce a bounded compiler rejection"
  [ ! -s "$name.stdout" ] || fail "$name emitted source on failure"
  [ -s "$name.stderr" ] || fail "$name produced no diagnostic"
  printf '%s\n' "$exit_code" > "$name.exit-code"
done
printf 'PASS: fresh Linux CLI stdout and error cases\n' | tee "$evidence_dir/logs/cli.log"

cd "$ce_dir"
run_phase npm-ci 360s env CYPRESS_INSTALL_BINARY=0 npm ci
run_phase checks 360s npm run check
run_phase properties 120s npm run test:props
run_phase webpack 480s npm run webpack
verify_source_hashes | tee "$evidence_dir/logs/patched-source-final.log"
[ -s out/dist/manifest.json ] || fail 'webpack did not produce its manifest'
[ ! -e manifest.json ] && [ ! -L manifest.json ] || fail 'unexpected root manifest before scratch configuration'
ln -s out/dist/manifest.json manifest.json
cat > etc/config/gooo.local.properties <<CONFIG
compilers=gooolocal
defaultCompiler=gooolocal
compiler.gooolocal.name=Gooo $(pin compiler_version) generated Go
compiler.gooolocal.exe=$compiler
compiler.gooolocal.versionFlag=version
compiler.gooolocal.versionRe=^gooo ([^ ]+)
CONFIG
printf 'defaultLanguage=gooo\n' > etc/config/compiler-explorer.local.properties
phase=api-server
python3 - <<'PY'
import socket
with socket.socket() as listener:
    listener.bind(('127.0.0.1', 10249))
PY
NODE_ENV=production node --no-warnings=ExperimentalWarning --import=tsx app.ts \
  --language gooo --host 127.0.0.1 --port 10249 --no-remote-fetch \
  --static out/webpack/static > "$evidence_dir/logs/server.log" 2>&1 &
server_pid=$!
ready=false
readiness_deadline=$((SECONDS + 60))
while [ "$SECONDS" -lt "$readiness_deadline" ]; do
  kill -0 "$server_pid" 2>/dev/null || fail 'owned CE server exited before readiness'
  if curl --silent --fail --max-time 1 \
    'http://127.0.0.1:10249/api/languages?fields=id,name' > /dev/null; then
    ready=true
    break
  fi
  sleep 1
done
[ "$ready" = true ] || fail 'CE server was not ready within the bounded wait'
run_phase api 90s node "$cli_dir/api-probe.mjs"
kill -0 "$server_pid" 2>/dev/null || fail 'owned CE server exited during API verification'
result=PASS
phase=complete
printf 'PASS: Linux Compiler Explorer checks and 11 actual-compiler API cases\n'
