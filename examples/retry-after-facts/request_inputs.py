#!/usr/bin/env python3
"""Strict transport-only request → one original Gooo input row. Stdlib only.

Read at most4097 raw bytes from a file or stdin, refusing anything over4096.
No policy, parser-result, clock, network or compiler calculation occurs here.
"""
import argparse
import json
import sys

MAX_RAW_BYTES = 4096
MAX_HEADER_BYTES = 64
MIN_INT64 = -(1 << 63)
MAX_INT64 = (1 << 63) - 1
ENTRY = 'app/retry-after-facts:AssessDelay'
FIELDS = ('header','header_present','elapsed_seconds','max_wait_seconds',
          'request_binding_current','guidance_applicable')


class InputRefusal(ValueError):
    pass


def need(condition, message):
    if not condition:
        raise InputRefusal(message)


def scalar_string(value):
    need(type(value) is str, 'header must be a string')
    need(not any(0xD800 <= ord(char) <= 0xDFFF for char in value),
         'header must contain only Unicode scalars')
    return value


def decoded_pairs(items):
    result={}
    for key,value in items:
        need(key not in result, 'duplicate decoded object key')
        result[key]=value
    return result


def exact_integer(token):
    value=int(token)
    need(MIN_INT64 <= value <= MAX_INT64, 'JSON integer outside signed-int64')
    return value


def reject_number(_):
    raise InputRefusal('floating-point, exponent and nonfinite JSON numbers are not supported')


def require_flat_json(text):
    # This only rejects nesting before the JSON decoder. It never accepts JSON:
    # the standard strict decoder below still validates all syntax/trailing text.
    quoted=False;escaped=False;depth=0
    for char in text:
        if quoted:
            if escaped:
                escaped=False
            elif char == '\\':
                escaped=True
            elif char == '"':
                quoted=False
        elif char == '"':
            quoted=True
        elif char == '[':
            raise InputRefusal('one flat JSON object required')
        elif char == '{':
            depth+=1
            need(depth <= 1, 'nested JSON objects are not supported')
        elif char == '}':
            depth-=1


def parse_request(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_RAW_BYTES,
         'nonempty request must be at most4096 raw bytes')
    try:
        text=raw.decode('utf-8', errors='strict')
        require_flat_json(text)
        request=json.loads(text,object_pairs_hook=decoded_pairs,parse_int=exact_integer,
                           parse_float=reject_number,parse_constant=reject_number)
    except (UnicodeError,json.JSONDecodeError,RecursionError) as error:
        raise InputRefusal('invalid UTF-8 or JSON request') from error
    need(type(request) is dict and set(request) == set(FIELDS),
         'request requires exactly the six original field names')
    header=scalar_string(request['header'])
    need(len(header.encode('utf-8', errors='strict')) <= MAX_HEADER_BYTES,
         'header exceeds64 decoded UTF-8 bytes')
    for name in ('elapsed_seconds','max_wait_seconds'):
        need(type(request[name]) is int and MIN_INT64 <= request[name] <= MAX_INT64,
             name+' must be an exact signed-int64 integer')
    for name in ('header_present','request_binding_current','guidance_applicable'):
        need(type(request[name]) is bool, name+' must be an exact Boolean')
    # Numeric policy domains, grammar, absence, applicability and stale binding
    # deliberately remain Gooo's job. In particular, preserve negative integers
    # and max_wait_seconds>86400 rather than repairing/refusing semantic facts.
    return request


def bind_request(raw):
    request=parse_request(raw)
    row={ENTRY+'.input'+str(index):request[name] for index,name in enumerate(FIELDS)}
    return (json.dumps({'schema':'gooo/body-composition-inputs/v1','inputs':[row]},
                       ensure_ascii=False,separators=(',',':'))+'\n').encode('utf-8')


def read_bounded(source):
    raw=source.read(MAX_RAW_BYTES+1)
    need(type(raw) is bytes and len(raw) <= MAX_RAW_BYTES,
         'request exceeds4096 raw bytes')
    return raw


def main(argv=None, *, stdin=None, stdout=None, stderr=None):
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('request',nargs='?',default='-',
                        help='JSON request file; - or no argument reads stdin, maximum4096 bytes')
    args=parser.parse_args(argv)
    stdin=sys.stdin.buffer if stdin is None else stdin
    stdout=sys.stdout.buffer if stdout is None else stdout
    stderr=sys.stderr if stderr is None else stderr
    try:
        if args.request == '-':
            raw=read_bounded(stdin)
        else:
            with open(args.request,'rb') as source:
                raw=read_bounded(source)
        bound=bind_request(raw)
    except (InputRefusal,OSError) as error:
        stderr.write('retry-after-facts input refused: '+str(error)+'\n')
        return 1
    # No output is written until the entire original request has passed checks.
    stdout.write(bound)
    return 0


if __name__ == '__main__':
    sys.exit(main())
