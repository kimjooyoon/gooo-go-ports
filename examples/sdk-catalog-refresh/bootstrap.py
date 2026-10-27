"""Build only the independent library profile in a fresh, caller-owned directory."""
import argparse
import importlib.util
import json
import os
import traceback
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('sdk_bootstrap_evidence', HERE/'evidence.py')
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)
h = e.host


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    out = args.out.resolve()
    assert not out.is_relative_to(e.ROOT.resolve())
    out.mkdir(mode=0o700, exist_ok=False)
    h.R = out
    first = None
    summary = {'schema': 'gooo/sdk-library-profile-bootstrap/v1', 'accepted': False, 'profile': e.PROFILE,
               'global_pin_changed': False, 'source_removed': False}
    try:
        go = args.go.resolve()
        assert go.is_file() and go.name == 'go'
        env = dict(os.environ, PATH=str(go.parent)+':/usr/bin:/bin', GOTOOLCHAIN='local',
                   GOWORK='off', GOENV='off', GOFLAGS='-mod=readonly -p=2',
                   GIT_TERMINAL_PROMPT='0', GOTELEMETRY='off')

        def run(name, argv, seconds=90, cwd=e.ROOT):
            return h.owned(name, argv, env, seconds, cwd)

        version = run('go-identity', [go, 'version'], 15).decode().strip()
        assert version in ['go version '+e.PROFILE['go_version']+' '+t for t in ['linux/amd64', 'darwin/arm64']]
        source = out/'source'
        run('git-init', ['git', 'init', source])
        run('git-origin', ['git', '-C', source, 'remote', 'add', 'origin', 'https://github.com/kimjooyoon/meta-ontology-go.git'])
        run('git-fetch', ['git', '-C', source, 'fetch', '--depth='+str(e.PROFILE['source_depth']), '--no-tags', 'origin',
                          e.PROFILE['compiler_revision'], 'refs/tags/'+e.PROFILE['version_tag']+':refs/tags/'+e.PROFILE['version_tag']], 120)
        run('git-checkout', ['git', '-C', source, 'checkout', '--detach', e.PROFILE['compiler_revision']])

        def clean(stem):
            assert run(stem+'-head', ['git', '-C', source, 'rev-parse', 'HEAD']).decode().strip() == e.PROFILE['compiler_revision']
            assert not run(stem+'-status', ['git', '-C', source, 'status', '--porcelain', '--untracked-files=all', '--ignored'])
            for suffix, key in [('', 'version_tag_object'), ('^{commit}', 'version_tag_commit')]:
                assert run(stem+'-tag-'+key, ['git', '-C', source, 'rev-parse', 'refs/tags/'+e.PROFILE['version_tag']+suffix]).decode().strip() == e.PROFILE[key]
            run(stem+'-ancestor', ['git', '-C', source, 'merge-base', '--is-ancestor', e.PROFILE['version_tag_commit'], e.PROFILE['compiler_revision']])

        clean('before')
        binary = out/'gooo'
        run('build', [go, 'build', '-trimpath', '-o', binary, './cmd/gooo'], 180, source)
        identity = h.strict(run('compiler-identity', [binary, 'version', '--build', '--json'], 15))
        e.identity(identity)
        clean('after')
        summary.update(accepted=True, compiler=h.ref(binary), compiler_identity=identity,
                       clean_checkout_before_and_after=True, go_identity=dict(h.ref(go), version=version))
    except BaseException as error:
        first = {'type': type(error).__name__, 'message': str(error), 'traceback': traceback.format_exc()}
        h.save(out/'first-failure.json', first)
    finally:
        pending = h.pending_cancellation()
        if pending is not None and first is None:
            first = {'type': type(pending).__name__, 'message': str(pending)}
        closed = bool(h.records) and all(r['waited'] and r['streams_EOF'] and r['PG_ESRCH'] for r in h.records)
        if first is None and not closed: first = {'type': 'RuntimeError', 'message': 'process closure not established'}
        summary.update(accepted=summary['accepted'] and first is None, first_failure=first,
                       all_direct_wait_EOF_PG_ESRCH=closed, processes=h.records)
        h.save(out/'summary.json', summary)
    if first is not None: raise SystemExit(1)
    print(json.dumps({'accepted': summary['accepted'], 'compiler': summary['compiler']}))


if __name__ == '__main__':
    with h.cancellation_handlers():
        main()
