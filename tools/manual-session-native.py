#!/usr/bin/env python3
"""Bounded capture and cleanup for the one manual experimental-session job."""
import argparse
import datetime
import hashlib
import itertools
import json
import os
from pathlib import Path
import re
import selectors
import shutil
import signal
import stat
import subprocess
import sys
import time

LOG_CAP = 1 << 20
META_CAP = 8 << 20
UPLOAD_CAP = 64 << 20
OWNER_LIMIT = 82
DEFAULTS = dict(poll=.1, sample=10, close=30, term=2, kill=2, drain=2,
                allocated=8 << 30, free=8 << 30)


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def bounded_read(path, limit=META_CAP):
    path = Path(path)
    before = path.lstat()
    if path.resolve() != path or not stat.S_ISREG(before.st_mode) or before.st_size > limit:
        raise ValueError('nonregular, aliased or oversized metadata: ' + str(path))
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    try:
        opened = os.fstat(fd)
        if not os.path.samestat(before, opened):
            raise ValueError('metadata changed before open')
        data = bytearray()
        while len(data) <= limit:
            part = os.read(fd, min(65536, limit + 1 - len(data)))
            if not part:
                break
            data.extend(part)
        after, current = os.fstat(fd), path.lstat()
        identity = lambda s: (s.st_dev, s.st_ino, s.st_mode, s.st_size, s.st_mtime_ns)
        if len(data) > limit or len(data) != opened.st_size or identity(opened) != identity(after) or identity(opened) != identity(current):
            raise ValueError('metadata changed or exceeded bound')
        return bytes(data)
    finally:
        os.close(fd)


def write_json(path, value):
    raw = (json.dumps(value, indent=2) + '\n').encode()
    if len(raw) > LOG_CAP:
        raise ValueError('controller metadata exceeds1MiB')
    temporary = path.with_name(path.name + '.tmp')
    with temporary.open('wb') as stream:
        stream.write(raw)
    os.replace(temporary, path)


def group_exists(pid):
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except PermissionError:
        return True  # Existence is not disproved; never report absent.


def proc_identity(pid, owner_root, compiler, verifier_pid, proc_root=None):
    """Linux-only proof before signaling a recorded separately grouped owner."""
    proc = (Path('/proc') if proc_root is None else Path(proc_root)) / str(pid)
    def read(name, cap):
        fd = os.open(proc / name, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
        try:
            data = os.read(fd, cap + 1)
            if len(data) > cap:
                raise ValueError('proc field exceeds bound')
            return data
        finally:
            os.close(fd)
    try:
        status = read('stat', 4096).decode().rsplit(')', 1)[1].split()
        start = status[19]
        pgid, sid = os.getpgid(pid), os.getsid(pid)
        if status[0] == 'Z':
            return dict(start=start, pgid=pgid, sid=sid, state='Z', binding=False)
        argv = read('cmdline', 16384).rstrip(b'\0').split(b'\0')
        expected = [os.fsencode(compiler), b'package', b'session']
        valid = (argv == expected and Path(os.readlink(proc / 'cwd')) == owner_root and
                 os.path.samefile(proc / 'exe', compiler))
        again = read('stat', 4096).decode().rsplit(')', 1)[1].split()
        if again[19] != start:
            raise ValueError('owner PID identity changed')
        return dict(start=start, pgid=pgid, sid=sid, state=status[0], binding=valid)
    except (FileNotFoundError, ProcessLookupError):
        return None


class Owners:
    def __init__(self, records, compiler, go_tool, verifier_pid, probe=proc_identity):
        self.records, self.compiler, self.go_tool = records, compiler, go_tool
        self.verifier_pid, self.probe = verifier_pid, probe
        self.groups, self.unresolved = {}, {}

    def discover(self):
        if not self.records.exists():
            return
        roots = sorted(self.records.glob('*/session-transport'))
        if len(roots) > OWNER_LIMIT:
            raise ValueError('owner roster exceeds82')
        for transport in roots:
            owner = transport.parent
            if str(owner) in {g['owner_root'] for g in self.groups.values()}:
                continue
            if transport.resolve() != transport or not transport.is_dir():
                self.unresolved[str(owner)] = 'aliased owner directory'
                continue
            errors = []
            for source in [transport/'started.json', transport/'process.json', owner/'session-owner.json']:
                if not source.exists():
                    continue
                try:
                    raw = bounded_read(source)
                    doc = json.loads(raw)
                    pid = doc['pid']
                    if type(pid) is not int or pid <= 1 or pid in (os.getpid(), self.verifier_pid):
                        raise ValueError('invalid owner PID')
                    if (doc['compiler'] != self.compiler or doc['configured_go_tool'] != self.go_tool or
                            doc['argv'] != [self.compiler, 'package', 'session'] or
                            doc['native_work'] != str(transport/'native-work')):
                        raise ValueError('owner command/path identity differs')
                    if doc['schema'] == 'gooo/native-session-process-start/v1':
                        if doc['cwd'] != str(owner):
                            raise ValueError('started owner cwd differs')
                    elif doc['schema'] == 'gooo/native-session-transport-summary/v1':
                        if doc['evidence_root'] != str(transport):
                            raise ValueError('process owner evidence root differs')
                    else:
                        raise ValueError('unknown owner process schema')
                    item = dict(pid=pid, owner_root=str(owner), source=str(source),
                                source_bytes=len(raw), source_sha256=hashlib.sha256(raw).hexdigest(),
                                proof=None, signals=[], absent_seen=False)
                    if not group_exists(pid):
                        item['absent_seen'] = True
                    else:
                        proof = self.probe(pid, owner, self.compiler, self.verifier_pid)
                        if not proof or not proof['binding'] or proof['pgid'] != pid or proof['sid'] != self.verifier_pid:
                            raise ValueError('live owner is not bound to this verifier session')
                        item['proof'] = proof
                    self.groups[pid] = item
                    self.unresolved.pop(str(owner), None)
                    break
                except (OSError, ValueError, KeyError, TypeError) as error:
                    errors.append(str(error)[:512])
            else:
                self.unresolved[str(owner)] = '; '.join(errors)[:1024] or 'started/process/final metadata absent'

    def alive(self):
        live = []
        for pid, item in self.groups.items():
            if not group_exists(pid):
                item['absent_seen'] = True
            else:
                live.append(pid)
        return live

    def signal(self, pid, sig):
        item = self.groups[pid]
        if item['absent_seen'] or not group_exists(pid):
            item['absent_seen'] = True
            return
        previous = item['proof']
        try:
            current = self.probe(pid, Path(item['owner_root']), self.compiler, self.verifier_pid)
        except (OSError, ValueError) as error:
            item['refused_signal'] = 'owner probe failed: '+str(error)[:512]
            return
        # A missing leader may leave its already-bound group alive. A PGID
        # cannot be reused while that group exists; never signal after absence.
        if not previous or (current and (current['start'] != previous['start'] or
                current['pgid'] != pid or current['sid'] != self.verifier_pid or
                not (current['binding'] or current['state'] == 'Z'))):
            item['refused_signal'] = 'owner identity changed or was not proved'
            return
        try:
            os.killpg(pid, sig)
            item['signals'].append(signal.Signals(sig).name)
        except ProcessLookupError:
            item['absent_seen'] = True
        except PermissionError:
            item['refused_signal'] = 'permission denied signaling recorded group'

    def stop(self, pump, limits):
        try:
            self.discover()
        except Exception as error:
            self.unresolved['discovery'] = str(error)[:1024]
        for sig, seconds in [(signal.SIGTERM, limits['term']), (signal.SIGKILL, limits['kill'])]:
            for pid in self.alive():
                self.signal(pid, sig)
            until = time.monotonic() + seconds
            while self.alive() and time.monotonic() < until:
                for pid in self.groups:
                    try:
                        os.waitpid(pid, os.WNOHANG)
                    except (ChildProcessError, PermissionError):
                        pass  # Only a direct child can be reaped; still check group absence.
                pump(.02)
        for pid, item in self.groups.items():
            item['group_remaining'] = group_exists(pid)
        return dict(groups=list(self.groups.values()), unresolved=self.unresolved,
                    remaining=self.alive())


def run_command(work, label, argv, timeout, native=None, limits=None,
                probe=proc_identity, sample=None, writer=write_json):
    work = Path(work).absolute()
    if work.resolve() != work or not re.fullmatch('[a-z0-9-]{1,40}', label):
        raise ValueError('invalid work path or label')
    diag = work/'diagnostics'; diag.mkdir(exist_ok=True)
    logs = diag/'logs'; logs.mkdir(exist_ok=True)
    path = diag/(label+'-process.json')
    if path.exists():
        raise ValueError('refuse a repeated command')
    limit = dict(DEFAULTS); limit.update(limits or {})
    record = dict(schema='gooo/manual-session-bounded-command/v1', argv=argv,
                  started_at_utc=now(), state='STARTING', first_error=None,
                  guard_samples=[], controller_errors=[])
    writer(path, record)  # A failed initial write must not start a producer.
    selector = selectors.DefaultSelector(); streams = {}; process = None
    owners = None; started = time.monotonic(); received = []
    def fail(reason):
        if record['first_error'] is None:
            record['first_error'] = str(reason)[:4096]
    def store():
        try:
            writer(path, record)
        except Exception as error:
            fail('process-record write: '+str(error)); record['controller_errors'].append(str(error)[:512])
            try:
                write_json(diag/(label+'-fallback.json'), record)
            except Exception:
                pass
    def pump(wait):
        for key, _ in selector.select(wait):
            name = key.data; stream = streams[name]
            try:
                raw = os.read(key.fileobj.fileno(), 65536)
                if not raw:
                    stream['eof'] = True; selector.unregister(key.fileobj); key.fileobj.close(); continue
                stream['hash'].update(raw); stream['observed'] += len(raw)
                kept = raw[:max(0, LOG_CAP-stream['retained'])]
                stream['file'].write(kept); stream['retained'] += len(kept)
                if stream['observed'] > LOG_CAP:
                    fail(name+' exceeded retained1MiB prefix; command canceled')
            except OSError as error:
                fail('stream capture: '+str(error))
                selector.unregister(key.fileobj); key.fileobj.close()
    def pause(seconds, until_exit=False):
        end = time.monotonic()+seconds
        while time.monotonic()<end:
            pump(min(limit['poll'], max(0, end-time.monotonic())))
            if owners:
                try:
                    owners.discover()
                except Exception as error:
                    fail('owner discovery: '+str(error))
            if until_exit and process.poll() is not None:
                break
    def on_signal(sig, frame):
        received.append(signal.Signals(sig).name); fail('controller received '+received[-1])
    prior = {sig:signal.signal(sig, on_signal) for sig in (signal.SIGINT, signal.SIGTERM)}
    try:
        env = {k:v for k,v in os.environ.items() if not k.startswith('GOOO_')}
        env.update(GOOO_LAYA_URL='', GOOO_LAYA_API_KEY='')
        process = subprocess.Popen(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   start_new_session=True, env=env)
        record.update(pid=process.pid, state='STARTED')
        for name, pipe in [('stdout', process.stdout), ('stderr', process.stderr)]:
            output=logs/(label+'.'+name); file=output.open('xb')
            streams[name]=dict(file=file, path=output, hash=hashlib.sha256(), observed=0, retained=0, eof=False)
            selector.register(pipe, selectors.EVENT_READ, name)
        if native:
            owners=Owners(work/'records', native['compiler'], native['go_tool'], process.pid, probe)
        store(); last_sample=-limit['sample']
        while process.poll() is None and record['first_error'] is None:
            pump(limit['poll'])
            if owners:
                owners.discover()
            elapsed=time.monotonic()-started
            if elapsed-last_sample >= limit['sample']:
                if sample:
                    observation=sample()
                else:
                    allocated=0
                    if native and (work/'records').exists():
                        for root, dirs, names in os.walk(work/'records', followlinks=False):
                            for name in names:
                                value=(Path(root)/name).lstat()
                                if not stat.S_ISREG(value.st_mode):
                                    raise ValueError('nonregular native evidence')
                                allocated += value.st_blocks*512
                    observation=dict(allocated_bytes=allocated, free_bytes=shutil.disk_usage(work).free)
                if len(record['guard_samples'])<256:
                    record['guard_samples'].append(dict(elapsed_seconds=elapsed, **observation))
                if native and (observation['allocated_bytes']>limit['allocated'] or observation['free_bytes']<limit['free']):
                    fail('sampled storage/free-space guard')
                store(); last_sample=elapsed
            if elapsed>timeout:
                fail('command deadline')
        if process.poll() not in (None, 0):
            fail('original exit status '+str(process.returncode))
    except BaseException as error:
        fail('controller exception '+type(error).__name__+': '+str(error))
    finally:
        if process is not None:
            if process.poll() is None:
                record['interrupt_requested']=now()
                try:
                    process.send_signal(signal.SIGINT)
                except ProcessLookupError:
                    pass
                pause(limit['close'], until_exit=True)
            if owners:
                try:
                    owners.discover()
                except Exception as error:
                    fail('owner discovery during cleanup: '+str(error))
                try:
                    if owners.alive() or owners.unresolved:
                        fail('owner groups require cleanup or lack identity metadata')
                    record['owner_cleanup']=owners.stop(pump, limit)
                except BaseException as error:
                    fail('owner cleanup exception: '+str(error))
                    record['owner_cleanup_error']=str(error)[:4096]
            if process.poll() is None or group_exists(process.pid):
                record['forced_verifier_group_cleanup']=True
                fail('verifier group required forced cleanup')
                for sig, seconds in [(signal.SIGTERM,limit['term']), (signal.SIGKILL,limit['kill'])]:
                    try:
                        os.killpg(process.pid,sig)
                    except ProcessLookupError:
                        pass
                    except OSError as error:
                        fail('verifier group signal: '+str(error))
                    pause(seconds,until_exit=True)
            if process.poll() is None:
                try:
                    process.kill()
                except OSError as error:
                    fail('verifier kill: '+str(error))
            try:
                record['exit_code']=process.wait(timeout=limit['kill']); record['waited']=True
            except subprocess.TimeoutExpired:
                record.update(exit_code=None,waited=False)
                fail('verifier wait did not complete within cleanup bound')
            until=time.monotonic()+limit['drain']
            while selector.get_map() and time.monotonic()<until:
                pump(limit['poll'])
            record['streams_joined']=not selector.get_map() and all(s['eof'] for s in streams.values())
            if selector.get_map():
                fail('stream EOF not observed before cleanup deadline')
            record['verifier_group_remaining']=group_exists(process.pid)
        else:
            record.update(exit_code=None, waited=False, streams_joined=False)
        for sig, handler in prior.items():
            signal.signal(sig,handler)
        for key in list(selector.get_map().values()):
            key.fileobj.close()
        selector.close()
        for name, stream in streams.items():
            try:
                stream['file'].close()
            except OSError as error:
                fail('capture file close: '+str(error))
            record[name]=dict(path=str(stream['path']),observed_bytes=stream['observed'],
                              retained_bytes=stream['retained'],observed_sha256=stream['hash'].hexdigest(),
                              hash_complete=stream['eof'],clipped=stream['observed']>stream['retained'])
        record.update(finished_at_utc=now(),wall_seconds=time.monotonic()-started,received_signals=received,state='COMPLETED')
        if process and record['exit_code'] != 0:
            fail('original exit status '+str(record['exit_code']))
        store()
    status=0 if record['exit_code']==0 and record['first_error'] is None else 1
    return status,record


def collect(work):
    work=Path(work); diag=work/'diagnostics'; records=work/'records'; upload=work/'upload'; upload.mkdir(exist_ok=True)
    diagnostics=list(itertools.islice((p for p in diag.rglob('*') if p.is_file()),65))
    selected=[(p,Path('diagnostics')/p.relative_to(diag),LOG_CAP) for p in diagnostics[:64]]
    discovery_rejections=[]
    if len(diagnostics)>64:
        discovery_rejections.append(dict(path='diagnostics',reason='more than64 diagnostic files'))
    selected += [(records/n,Path('records')/n,META_CAP) for n in ['summary.json','session-observations.json','session-source.json','session-source-final.json'] if (records/n).exists()]
    transports=list(itertools.islice(records.glob('*/session-transport'),OWNER_LIMIT+1))
    if len(transports)>OWNER_LIMIT:
        discovery_rejections.append(dict(path='records',reason='more than82 owner directories'))
    for transport in transports[:OWNER_LIMIT]:
        owner=transport.parent; final=owner/'session-owner.json'
        choices=[transport/'started.json',final] if final.exists() else [transport/'started.json',transport/'process.json']
        selected += [(p,Path('records')/p.relative_to(records),META_CAP) for p in choices if p.exists()]
    for pattern in ['*exhaustive*/exhaustive.json','*exhaustive*/native-test.txt']:
        paths=list(itertools.islice(records.glob(pattern),8))
        if len(paths)>7:
            discovery_rejections.append(dict(path=pattern,reason='more than7 exhaustive files'))
        selected += [(p,Path('records')/p.relative_to(records),META_CAP) for p in paths[:7]]
    kept=[]; rejected=discovery_rejections; total=0
    budget=UPLOAD_CAP-LOG_CAP  # Reserve1MiB for inventory itself.
    for source,relative,cap in sorted(selected,key=lambda x:str(x[1])):
        if len(kept)>=320 or total>=budget:
            rejected.append(dict(path=str(relative),reason='aggregate/count bound')); continue
        target=upload/relative; target.parent.mkdir(parents=True,exist_ok=True)
        cap=min(cap,budget-total); fd=None
        try:
            before=source.lstat()
            if source.resolve()!=source or not stat.S_ISREG(before.st_mode):
                raise ValueError('nonregular or aliased diagnostic')
            fd=os.open(source,os.O_RDONLY|os.O_NONBLOCK|os.O_NOFOLLOW); opened=os.fstat(fd)
            if not os.path.samestat(before,opened):
                raise ValueError('diagnostic changed before open')
            h=hashlib.sha256(); count=0; eof=False
            with target.open('xb') as out:
                while count<cap:
                    raw=os.read(fd,min(65536,cap-count))
                    if not raw:
                        eof=True; break
                    count+=len(raw); total+=len(raw)
                    out.write(raw); h.update(raw)
                if count==cap:
                    eof=not os.read(fd,1)
            after=os.fstat(fd); current=source.lstat()
            unchanged=(opened.st_mode==after.st_mode==current.st_mode and opened.st_size==after.st_size==current.st_size and opened.st_mtime_ns==after.st_mtime_ns==current.st_mtime_ns and os.path.samestat(opened,current))
            complete=eof and unchanged and count==opened.st_size
            kept.append(dict(path=str(relative),source_bytes_from_stat=opened.st_size,retained_bytes=count,
                             retained_sha256=h.hexdigest(),source_sha256=h.hexdigest() if complete else None,
                             complete=complete,clipped=not eof))
            if not complete:
                rejected.append(dict(path=str(relative),reason='clipped or changed; prefix retained'))
        except (OSError,ValueError) as error:
            rejected.append(dict(path=str(relative),reason=str(error)[:512]))
        finally:
            if fd is not None:
                os.close(fd)
    inventory=dict(kept=kept,rejected=rejected,retained_bytes=total,
                   scope='Bounded metadata/prefixes only; no retained independent full raw-receipt audit.')
    write_json(upload/'inventory.json',inventory)
    return 1 if rejected else 0,inventory


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode',choices=['capture','native','collect'])
    parser.add_argument('--work',type=Path,required=True)
    parser.add_argument('--label'); parser.add_argument('--timeout',type=float)
    parser.add_argument('--compiler'); parser.add_argument('--go')
    given=sys.argv[1:]; split=given.index('--') if '--' in given else len(given)
    command=given[split+1:] if split<len(given) else []
    args=parser.parse_args(given[:split])
    if args.mode=='collect':
        code,record=collect(args.work)
    else:
        if args.mode=='native':
            if not args.compiler or not args.go or shutil.disk_usage(args.work).free < 12<<30:
                parser.error('native requires exact compiler/Go paths and12GiB free')
            if (args.work/'records').exists():
                parser.error('refuse existing native evidence')
            argv=[str(args.work/'verify-session'),'--execution-profile','experimental-session-276-gzip-v1',
                  '--compiler',args.compiler,'--out',str(args.work/'records'),'--workers','4','--suite-order','catalog',
                  '--case-profile','legacy','--workspace-profile','legacy','--fail-fast','--timeout','20m']
            code,record=run_command(args.work,'native',argv,1320,native=dict(compiler=args.compiler,go_tool=args.go))
        else:
            argv=command
            if not args.label or not args.timeout or not argv:
                parser.error('capture requires label/timeout and command after --')
            code,record=run_command(args.work,args.label,argv,args.timeout,limits={'close':3})
    print(json.dumps(dict(status=code,first_error=record.get('first_error'),scope='Bounded controller/metadata only.')))
    return code


if __name__=='__main__':
    raise SystemExit(main())
