#!/usr/bin/env python3
"""Run the finite ByteOrReject paired experiment in a new output directory."""
import argparse
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--candidate', required=True, help='clean engine 92dc06b binary')
parser.add_argument('--published', required=True, help='published engine 366ee6 binary')
parser.add_argument('--go-bin', default='go', help='Go 1.27.2 executable')
parser.add_argument('--out', required=True, help='new output directory')
args = parser.parse_args()
fixtures = pathlib.Path(__file__).resolve().parent
root = pathlib.Path(args.out).resolve()
if root.exists():
    parser.error('--out must be a new directory')
go_binary = shutil.which(args.go_bin)
if not go_binary:
    parser.error('Go executable not found')
root.mkdir(parents=True)
# Keep generated observer mains outside the parent repository's Go package scan.
(root/'go.mod').write_text('module byteguard-experiment\n\ngo 1.27.2\n')
(root/'tools').mkdir()
for source, target in [(args.candidate, 'candidate-gooo'), (args.published, 'published-gooo')]:
    shutil.copyfile(pathlib.Path(source).resolve(), root/'tools'/target)
    (root/'tools'/target).chmod(0o755)
# Only the executable is linked; Go locates its installed toolchain via this link.
# tools/ is excluded from the evidence bundle.
(root/'tools/go').symlink_to(pathlib.Path(go_binary).resolve())
for name in ['baseline.gooo', 'constrained.gooo']:
    shutil.copyfile(fixtures/name, root/name)
for name in ['oracle.go', 'instrument.go', 'observe-main.go']:
    shutil.copyfile(fixtures/'support'/(name+'.txt'), root/name)
env = dict(os.environ, GOTOOLCHAIN='local', GOWORK='off')
for name in ['GOOO_LAYA_URL', 'GOOO_LAYA_API_KEY']:
    env.pop(name, None)
commands = []
def run(label, argv, stdin=None, expected_exit=0):
    result = subprocess.run(argv, cwd=root, env=env, input=stdin, capture_output=True)
    (root/(label+'.stdout.json')).write_bytes(result.stdout)
    (root/(label+'.stderr.log')).write_bytes(result.stderr)
    commands.append({'label':label, 'argv':argv, 'cwd':'.', 'exit_code':result.returncode})
    (root/'commands.json').write_text(json.dumps({'environment':{'GOTOOLCHAIN':'local','GOWORK':'off','unset':['GOOO_LAYA_URL','GOOO_LAYA_API_KEY']},'commands':commands}, indent=2)+'\n')
    if result.returncode != expected_exit:
        raise SystemExit(f'{label}: expected exit {expected_exit}, got {result.returncode}\n'+result.stderr.decode())
    print(label, result.returncode, flush=True)
    return result.stdout

run('candidate-identity',['tools/candidate-gooo','version','--build','--json'])
run('published-identity',['tools/published-gooo','version','--build','--json'])
candidate = json.loads((root/'candidate-identity.stdout.json').read_text())
published = json.loads((root/'published-identity.stdout.json').read_text())
assert candidate['compiler_source_sha']=='92dc06b38f72518443f8a90e5c98a645f0c756f1'
assert published['compiler_source_sha']=='366ee6ef8f9d2b9b57331dfd26b6f908c0289851'
assert candidate['source_status']==published['source_status']=='CLEAN_VCS'
assert candidate['go_version']==published['go_version']=='go1.27.2'
assert candidate['version']==published['version']=='0.6.24-dev'
tools = ['tools/candidate-gooo','tools/published-gooo','tools/go']
(root/'producer-hashes.json').write_text(json.dumps({name:hashlib.sha256((root/name).read_bytes()).hexdigest() for name in tools},indent=2)+'\n')

inputs = [-9223372036854775808,-9007199254740993,-257,-256,-2,-1]+list(range(256))+[256,257,511,512,9007199254740993,9223372036854775807]
(root/'inputs.json').write_text(json.dumps(inputs)+'\n')
oracle_raw = run('oracle',['tools/go','run','oracle.go'],(root/'inputs.json').read_bytes())
(root/'oracle.json').write_bytes(oracle_raw)
oracle = json.loads(oracle_raw)
by_input = {row['input']:row for row in oracle}
assert len(by_input)==268
def cases(name, rows):
    value = {'schema':'gooo/body-composition-cases/v1','cases':[{'inputs':{'Main':row['input']},'expected':{'Main':row['expected']}} for row in rows]}
    (root/name).write_text(json.dumps(value,indent=2)+'\n')
cases('construction-cases.json',[by_input[x] for x in [-2,0,255,256]])
for i,start in enumerate(range(0,len(oracle),128)):
    cases(f'evaluation-{i}.json',oracle[start:start+128])

for arm in ['baseline','constrained']:
    run(arm+'-fresh',['tools/candidate-gooo','body-construct','--source',arm+'.gooo','--entry','Main','--construction-cases','construction-cases.json','--cases','evaluation-0.json','--attempts','4','--go-bin','tools/go','--out',arm+'-fresh'])
    for batch in range(3):
        run(f'{arm}-replay-{batch}',['tools/candidate-gooo','body-construct','--source',arm+'-fresh/original.gooo','--construction',arm+'-fresh/construction.json','--cases',f'evaluation-{batch}.json','--go-bin','tools/go','--out',f'{arm}-replay-{batch}'])
run('published-constrained',['tools/published-gooo','body-construct','--source','constrained.gooo','--entry','Main','--construction-cases','construction-cases.json','--cases','evaluation-0.json','--attempts','4','--go-bin','tools/go','--out','published-constrained'],expected_exit=1)

run('observer-build',['tools/go','build','-trimpath','-o','tools/instrument','instrument.go'])
for arm in ['baseline','constrained']:
    target = arm+'-observer'
    (root/target).mkdir()
    shutil.copyfile(root/(arm+'-fresh/generated.go'),root/target/'generated.go')
    shutil.copyfile(root/'observe-main.go',root/target/'main.go')
    run(arm+'-observer-extract',['tools/instrument',target+'/generated.go',target+'/observer.go'])
    run(arm+'-observer',['tools/go','run',target+'/generated.go',target+'/observer.go',target+'/main.go'],(root/'inputs.json').read_bytes())

result = subprocess.run([sys.executable,str(fixtures/'support/validate.py'),str(root)],capture_output=True)
(root/'validation.stdout.json').write_bytes(result.stdout)
(root/'validation.stderr.log').write_bytes(result.stderr)
if result.returncode:
    raise SystemExit(result.stderr.decode())
print(result.stdout.decode(),end='')
# Preserve every evidence file byte-for-byte; executable tools are separately identified.
files = sorted(path for path in root.rglob('*') if path.is_file() and 'tools' not in path.relative_to(root).parts and path.name!='FILES.sha256')
(root/'FILES.sha256').write_text(''.join(hashlib.sha256(path.read_bytes()).hexdigest()+'  '+path.relative_to(root).as_posix()+'\n' for path in files))
