#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
revision=83928fa4538245abe33b08549da47f8cbdc50ed2
repository=https://github.com/kimjooyoon/meta-ontology-go.git
required_go=go1.27.2

fail() { printf 'bootstrap: %s\n' "$*" >&2; exit 1; }
for tool in git go mktemp; do
  command -v "$tool" >/dev/null 2>&1 || fail "required command is missing: $tool"
done
export GOWORK=off
actual_go=$(go env GOVERSION)
[ "$actual_go" = "$required_go" ] || fail "require $required_go, found $actual_go; select it with export GOTOOLCHAIN=$required_go and retry"

# Serialize publication. Never guess that an existing lock is safe to delete:
# another process, a different host, or an unhandled SIGKILL may have left it.
lock=.bootstrap.lock
if ! mkdir "$lock" 2>/dev/null; then
  fail "$lock already exists; another bootstrap may be running. If it has stopped, inspect and move the lock aside, then retry. No existing files were removed."
fi
stage=
legacy_backup=
cleanup() {
  result=$?
  trap - 0
  trap '' HUP INT TERM
  # A handled signal between these two directory renames must not strand the
  # original incomplete checkout. A published new checkout keeps its backup.
  if [ -n "$legacy_backup" ] && [ -d "$legacy_backup/compiler" ] && [ ! -e .compiler ] && [ ! -L .compiler ]; then
    mv "$legacy_backup/compiler" .compiler || printf 'bootstrap: original checkout remains at %s/compiler\n' "$legacy_backup" >&2
  fi
  [ -z "$stage" ] || rm -rf "$stage"
  rm -f "$lock/pid"
  rmdir "$lock" 2>/dev/null || printf 'bootstrap: additional files remain in %s; inspect them before retrying\n' "$lock" >&2
  exit "$result"
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
printf '%s\n' "$$" > "$lock/pid"
stage=$(mktemp -d "$(pwd -P)/$lock/work.XXXXXX")

[ ! -L .compiler ] || fail '.compiler is a symlink; preserve it elsewhere before retrying'
[ ! -L .gooo ] || fail '.gooo is a symlink; preserve it elsewhere before retrying'
if [ -e .gooo ] && [ ! -f .gooo ]; then
  fail '.gooo is not a regular file; preserve it elsewhere before retrying'
fi

check_incomplete() {
  [ -d .compiler ] && [ ! -L .compiler ] && [ -d .compiler/.git ] && [ ! -L .compiler/.git ] || fail '.compiler changed during installation; existing path preserved'
  [ "$(git -C .compiler rev-parse --show-toplevel)" = "$(pwd -P)/.compiler" ] || fail '.compiler is not its own repository root'
  [ -z "$(git -C .compiler rev-parse --verify HEAD 2>/dev/null || :)" ] || fail '.compiler gained a HEAD during installation; existing checkout preserved'
  # Recognize only the empty clone produced by the old bootstrap's failed
  # fetch. Preserve even that Git metadata in a separate backup on success.
  [ "$(git -C .compiler remote get-url origin 2>/dev/null || :)" = "$repository" ] || fail '.compiler has no HEAD and an unrecognized origin; existing checkout preserved'
  existing_refs=$(git -C .compiler for-each-ref --format='%(refname)') || fail 'cannot inspect .compiler refs; existing checkout preserved'
  [ -z "$existing_refs" ] || fail '.compiler has no HEAD but contains refs; existing checkout preserved'
  existing_index=$(git -C .compiler ls-files) || fail 'cannot inspect .compiler index; existing checkout preserved'
  [ -z "$existing_index" ] || fail '.compiler has no HEAD but contains tracked files; existing checkout preserved'
  existing_files=$(find .compiler/. ! -name . -prune ! -name .git -print) || fail 'cannot inspect .compiler files; existing checkout preserved'
  [ -z "$existing_files" ] || fail '.compiler has no HEAD but contains working files; existing checkout preserved'
}

fresh=true
legacy=false
if [ -e .compiler ]; then
  [ -d .compiler ] && [ -d .compiler/.git ] && [ ! -L .compiler/.git ] || fail '.compiler is not a regular compiler checkout; preserve it elsewhere before retrying'
  [ "$(git -C .compiler rev-parse --show-toplevel)" = "$(pwd -P)/.compiler" ] || fail '.compiler is not its own repository root'
  head=$(git -C .compiler rev-parse --verify HEAD 2>/dev/null || :)
  if [ -n "$head" ]; then
    [ "$head" = "$revision" ] || fail ".compiler is at $head, require $revision; existing checkout preserved"
    existing_status=$(git -C .compiler status --porcelain --untracked-files=all --ignored) || fail 'cannot inspect .compiler status; existing checkout preserved'
    [ -z "$existing_status" ] || fail '.compiler contains modified, untracked, or ignored files; existing checkout preserved'
    fresh=false
  else
    check_incomplete
    legacy=true
  fi
fi

compiler_source=$(pwd -P)/.compiler
if [ "$fresh" = true ]; then
  compiler_source=$stage/compiler
  git init "$compiler_source"
  git -C "$compiler_source" remote add origin "$repository"
  git -C "$compiler_source" fetch --depth=1 origin "$revision"
  git -C "$compiler_source" checkout --detach "$revision"
fi
[ "$(git -C "$compiler_source" rev-parse HEAD)" = "$revision" ] || fail 'staged compiler revision differs'
source_status=$(git -C "$compiler_source" status --porcelain --untracked-files=all --ignored) || fail 'cannot inspect compiler status'
[ -z "$source_status" ] || fail 'compiler sources are not clean'
(cd "$compiler_source" && go build -trimpath -o "$stage/gooo" ./cmd/gooo)
"$stage/gooo" version --build --json > "$stage/identity.json"
go run ./tools/bootstrap-check.go "$stage/identity.json"
# Catch source edits during the build before replacing a working executable.
[ "$(git -C "$compiler_source" rev-parse HEAD)" = "$revision" ] || fail 'compiler revision changed while building'
source_status=$(git -C "$compiler_source" status --porcelain --untracked-files=all --ignored) || fail 'cannot inspect compiler status after building'
[ -z "$source_status" ] || fail 'compiler sources changed while building'

if [ "$fresh" = true ]; then
  if [ "$legacy" = true ]; then
    check_incomplete
    legacy_backup=$(mktemp -d "$(pwd -P)/.compiler-interrupted.XXXXXX")
    mv .compiler "$legacy_backup/compiler"
    printf 'bootstrap: preserved incomplete checkout at %s/compiler\n' "$legacy_backup" >&2
  fi
  [ ! -e .compiler ] && [ ! -L .compiler ] || fail '.compiler appeared during installation; existing path preserved'
  mv "$compiler_source" .compiler
fi
[ ! -L .gooo ] || fail '.gooo became a symlink; existing path preserved'
[ ! -e .gooo ] || [ -f .gooo ] || fail '.gooo is no longer a regular file; existing path preserved'
mv -f "$stage/gooo" .gooo
cat "$stage/identity.json"
