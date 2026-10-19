"""Package checked native MCP transports; never build Go or a language engine.

The optional native check reuses the pinned independent stdio verifier.
Original-CI equivalence mode is restricted to uncommitted private prototypes.
"""
import argparse
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import platform
import re
import selectors
import shutil
import signal
import stat
import struct
import subprocess
import sys
import tarfile
import time
import types
import unittest
REFERENCE_SOURCE = '9c29602f77492be0e60c0ef91afd6d3968890b24'

SOURCE_FILES = {'examples/service-grant-mcp/LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
 'examples/service-grant-mcp/README.md': 'b9a58f00738107a3c5280036835e9d230309f92ef3faae3e94839d5f3be353b8',
 'examples/service-grant-mcp/fixtures/control-1.input': 'eaa120445bed9038ab6d9a4e71949344f6d92c44bad73fc7d713b8fc17c0dcb4',
 'examples/service-grant-mcp/fixtures/control-1.stdout': '0617f412a5a00e9ad3e76c9c1ecca6acbef76b366f5f838a607dbdb9ed0944f9',
 'examples/service-grant-mcp/fixtures/control-2.input': '947f10f40652f17eb97c9f7d7a372a493a9eca5e24976ab7dbbe9c1863bac34e',
 'examples/service-grant-mcp/fixtures/control-2.stdout': '15060c44af10f8e8e02078557240332d7198ceb4faad9550d1d21439209231c0',
 'examples/service-grant-mcp/fixtures/control-3.input': 'a9f4c61b9e64f46c6c81b94610ed3d5181b735d3532eb10417b6e52da01c2bbc',
 'examples/service-grant-mcp/fixtures/control-3.stdout': '8a3832df10d53c674d2f15c51d5b275876307396fb451283cd8cf6499a98b4f3',
 'examples/service-grant-mcp/fixtures/control-4.input': 'd744fc59eceac73a8cc68430047424c5b065137f4807daea9a411318a220cc0a',
 'examples/service-grant-mcp/fixtures/control-4.stdout': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
 'examples/service-grant-mcp/fixtures/origin.json': '192814d54805c802cad17c6038a13daf12a3460bc4853922923ba10c016a75a7',
 'examples/service-grant-mcp/go.mod': '13ce78fd2039122563c9f6b28081ee6b0a196f8d208ae934430e586dec76a20f',
 'examples/service-grant-mcp/go.sum': 'c4ac5b32430704aa5ac7ea506fcacadc410ef5798eaf8c4f6e5c15e6f7f11926',
 'examples/service-grant-mcp/main.go': 'ff1dbc06e7a63f2f5fcb75f4a9fbeafa68a2fd840488871f1429f25b2ed4ffbd',
 'examples/service-grant-mcp/main_test.go': '482e38af1241697b94dffc69a4f2b58144b23b2c513a8a114a8a1115b1e06aa8',
 'examples/service-grant-mcp/verify.py': 'a55b628f0b6d2bd2d7f490685449b571edd379382d4124675eb83200c8df6c46',
 'examples/service-grant-mcp/verify_close_test.py': '63076fe685edbed02e8d6ebb821b1660f6c6504ca110ac44dcf5a01e5b523375'}

DEPENDENCIES = [{'dependency': {'Path': 'github.com/google/jsonschema-go',
                 'Sum': 'h1:/DBOLZTfDow7pe2GmaJNhltueGTtDKICi8V8p+DQPd0=',
                 'Version': 'v0.4.3'},
  'notices': {'LICENSE': '2d56c53449691d85d9aea245eb8dac12713e9075d70d5557b82ae1e94805b357'}},
 {'dependency': {'Path': 'github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy',
                 'Sum': 'h1:up4zblnjEjgePzF/8OZb3qd0tgSJ33J/WvMZY3IHl8A=',
                 'Version': 'v0.0.0-20261018191951-2aa0f96805f1'},
  'notices': {'LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
              'LICENSE-APACHE-2.0': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4',
              'NOTICE.md': '90163940130087486fa104d9db6e9c56da1c7de37c2e32364340c0cdc367bc30',
              'upstream/LICENSE': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4'}},
 {'dependency': {'Path': 'github.com/modelcontextprotocol/go-sdk',
                 'Sum': 'h1:KIvahhYqwtbeniWVPs3TcXEA7b8jEtwfBpOTAI+Urx4=',
                 'Version': 'v1.8.0'},
  'notices': {'LICENSE': 'af679003d933f045393a6a029f43da113f9ae364eac651d9ae268392985580f5'}},
 {'dependency': {'Path': 'github.com/segmentio/asm',
                 'Sum': 'h1:WM03sfUOENvvKexOLp+pCqgb/WDjsi7EK8gIsICtzhc=',
                 'Version': 'v1.1.3'},
  'notices': {'LICENSE': 'e2a78de21d6d8ded2dff0f3189cd32e011630d785da127ebfbc8949012c0947b'}},
 {'dependency': {'Path': 'github.com/segmentio/encoding',
                 'Sum': 'h1:OW1VRern8Nw6ITAtwSZ7Idrl3MXCFwXHPgqESYfvNt0=',
                 'Version': 'v0.5.4'},
  'notices': {'LICENSE': 'd6d71a1f7dc6539e371120cc7af6e3257e55ca79634d473211f217b8965b0f16',
              'json/fuzz/LICENSE': 'b40930bbcf80744c86c46a12bc9da056641d722716c378f5659b9e555ef833e1'}},
 {'dependency': {'Path': 'github.com/yosida95/uritemplate/v3',
                 'Sum': 'h1:Ed3Oyj9yrmi9087+NczuL5BwkIc4wvTb5zIM+UJPGz4=',
                 'Version': 'v3.0.2'},
  'notices': {'LICENSE': '0761aadfb1921103752869ee942d4a71bdd54494697684d4b13dc17ad9781191'}},
 {'dependency': {'Path': 'golang.org/x/oauth2',
                 'Sum': 'h1:Mv2mzuHuZuY2+bkyWXIHMfhNdJAdwW3FuWeCPYN5GVQ=',
                 'Version': 'v0.35.0'},
  'notices': {'LICENSE': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'}},
 {'dependency': {'Path': 'golang.org/x/sync',
                 'Sum': 'h1:e0PTpb7pjO8GAtTs2dQ6jYa5BWYlMuX047Dco/pItO4=',
                 'Version': 'v0.20.0'},
  'notices': {'LICENSE': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad',
              'PATENTS': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'}},
 {'dependency': {'Path': 'golang.org/x/sys',
                 'Sum': 'h1:Ivj+2Cp/ylzLiEU89QhWblYnOE9zerudt9Ftecq2C6k=',
                 'Version': 'v0.41.0'},
  'notices': {'LICENSE': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad',
              'PATENTS': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'}},
 {'dependency': {'Path': 'golang.org/x/time',
                 'Sum': 'h1:bbrp8t3bGUeFOx08pvsMYRTCVSMk89u4tKbNOZbp88U=',
                 'Version': 'v0.15.0'},
  'notices': {'LICENSE': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad',
              'PATENTS': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'}}]

GO_NOTICES = {'LICENSE': {'scope': 'toolchain root text; runtime distribution reference',
             'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'PATENTS': {'scope': 'toolchain root text; runtime distribution reference',
             'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/compile/internal/ssa/_gen/vendor/golang.org/x/tools/LICENSE': {'scope': 'compiler/tooling distribution '
                                                                                  'only; not claimed included in '
                                                                                  'MCP executable',
                                                                         'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/compile/internal/ssa/_gen/vendor/golang.org/x/tools/PATENTS': {'scope': 'compiler/tooling distribution '
                                                                                  'only; not claimed included in '
                                                                                  'MCP executable',
                                                                         'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/github.com/google/pprof/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                             'included in MCP executable',
                                                    'sha256': 'cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30'},
 'src/cmd/vendor/github.com/google/pprof/third_party/svgpan/LICENSE': {'scope': 'compiler/tooling distribution '
                                                                                'only; not claimed included in '
                                                                                'MCP executable',
                                                                       'sha256': '154c946c17de61ca71d28e13673d76a5798e6e25995a7397fbae157afc8f62b4'},
 'src/cmd/vendor/github.com/ianlancetaylor/demangle/LICENSE': {'scope': 'compiler/tooling distribution only; not '
                                                                        'claimed included in MCP executable',
                                                               'sha256': 'b73277f7730993ef8e7f80aed83e5309762fed4d77eced982abb2c00657c074d'},
 'src/cmd/vendor/golang.org/x/arch/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '8cda009bd927676a95ee7ce2dae921442ac4d9584d041b56ecfcba6839032f0b'},
 'src/cmd/vendor/golang.org/x/arch/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/build/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                        'in MCP executable',
                                               'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/build/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                        'in MCP executable',
                                               'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/mod/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                      'in MCP executable',
                                             'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/mod/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                      'in MCP executable',
                                             'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/sync/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/sync/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/sys/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                      'in MCP executable',
                                             'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/sys/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                      'in MCP executable',
                                             'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/telemetry/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                            'included in MCP executable',
                                                   'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/telemetry/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                            'included in MCP executable',
                                                   'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/term/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/term/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/text/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/text/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                       'in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/tools/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                        'in MCP executable',
                                               'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/tools/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed included '
                                                        'in MCP executable',
                                               'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/rsc.io/markdown/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed included in '
                                                     'MCP executable',
                                            'sha256': '2d36597f7117c38b006835ae7f537487207d8ec407aa9d9980794b2030cbc067'},
 'src/crypto/internal/boring/LICENSE': {'scope': 'toolchain component text; inclusion in this MCP executable not '
                                                 'established',
                                        'sha256': '56210f826b8f0fbac3160dfe55c97f4019eb6cdda3963b5a0eab8a8bdb62360e'},
 'src/runtime/testdata/testgoroutineleakprofile/goker/LICENSE': {'scope': 'toolchain testdata only; not claimed '
                                                                          'included in MCP executable',
                                                                 'sha256': 'f99df8ae6da6de047ed002d9dbe8682d9a19c88e9a7473abdadf3ac206621a85'},
 'src/vendor/golang.org/x/crypto/LICENSE': {'scope': 'Go standard-library vendored component text; specific '
                                                     'linked packages not derived in this task',
                                            'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/crypto/PATENTS': {'scope': 'Go standard-library vendored component text; specific '
                                                     'linked packages not derived in this task',
                                            'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/vendor/golang.org/x/net/LICENSE': {'scope': 'Go standard-library vendored component text; specific linked '
                                                  'packages not derived in this task',
                                         'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/net/PATENTS': {'scope': 'Go standard-library vendored component text; specific linked '
                                                  'packages not derived in this task',
                                         'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/vendor/golang.org/x/sys/LICENSE': {'scope': 'Go standard-library vendored component text; specific linked '
                                                  'packages not derived in this task',
                                         'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/sys/PATENTS': {'scope': 'Go standard-library vendored component text; specific linked '
                                                  'packages not derived in this task',
                                         'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/vendor/golang.org/x/text/LICENSE': {'scope': 'Go standard-library vendored component text; specific linked '
                                                   'packages not derived in this task',
                                          'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/text/PATENTS': {'scope': 'Go standard-library vendored component text; specific linked '
                                                   'packages not derived in this task',
                                          'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'}}

SUPPORT_HASHES = {'api_policy_cli_bundle.py': 'd67ad930f9a8405d945e9831f61c47291ce17985ea264167b8a309271560000b',
 'service_policy_cli_bundle.py': '568a6ec152c988fbdc68b3408b48b635d53007e545792de26e48d9a5eb7f1900'}

SCHEMAS = {'input_schema': {'additionalProperties': False,
                  'properties': {'after': {'type': 'string'},
                                 'before': {'type': 'string'},
                                 'required_name': {'type': 'string'},
                                 'required_type': {'type': 'string'}},
                  'required': ['before', 'after', 'required_type', 'required_name'],
                  'type': 'object'},
 'output_schema': {'additionalProperties': False,
                   'properties': {'bindings': {'additionalProperties': False,
                                               'properties': {'assets': {'additionalProperties': {'type': 'string'},
                                                                         'type': 'object'},
                                                              'compiler_source_sha': {'type': 'string'},
                                                              'generation_origin': {'type': 'string'},
                                                              'generation_sha256': {'type': 'string'}},
                                               'required': ['compiler_source_sha',
                                                            'generation_sha256',
                                                            'generation_origin',
                                                            'assets'],
                                               'type': 'object'},
                                  'decision': {'additionalProperties': False,
                                               'properties': {'invalid_field': {'type': 'string'},
                                                              'reason': {'type': 'string'},
                                                              'required_after_match': {'type': 'boolean'},
                                                              'required_before_match': {'type': 'boolean'},
                                                              'required_observed': {'type': 'boolean'},
                                                              'status': {'enum': ['ELIGIBLE',
                                                                                  'REFUSED',
                                                                                  'INVALID_INPUT'],
                                                                         'type': 'string'},
                                                              'subset_rule_id': {'type': 'string'}},
                                               'required': ['status',
                                                            'reason',
                                                            'invalid_field',
                                                            'required_observed',
                                                            'required_before_match',
                                                            'required_after_match',
                                                            'subset_rule_id'],
                                               'type': 'object'},
                                  'facts': {'additionalProperties': False,
                                            'properties': {'after': {'type': 'string'},
                                                           'before': {'type': 'string'},
                                                           'required_name': {'type': 'string'},
                                                           'required_type': {'type': 'string'}},
                                            'required': ['before', 'after', 'required_type', 'required_name'],
                                            'type': 'object'}},
                   'required': ['facts', 'decision', 'bindings'],
                   'type': 'object'}}

MODULE = "github.com/kimjooyoon/gooo-go-ports/examples/service-grant-mcp"
COMMAND = "gooo-service-grant-mcp"
MAX_STREAM = 64 << 10
NATIVE_STDOUT_CAP = 512 << 10
NATIVE_STDOUT_TOTAL_CAP = 1 << 20
MAX_SOURCE = 2 << 20
GENERATION = "40ae704abbf2cae1974d2eec51cec2564c8d72dd44536b31c9c4f55f71fc0f85"
PIN = "83928fa4538245abe33b08549da47f8cbdc50ed2"


def imported(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


HERE = Path(__file__).resolve().parent
for support, digest in SUPPORT_HASHES.items():
    if hashlib.sha256((HERE/support).read_bytes()).hexdigest() != digest:
        raise ValueError("packaging support source differs: " + support)
BASE = imported("mcp_archive_base", HERE/"api_policy_cli_bundle.py")
SERVICE = imported("mcp_service_base", HERE/"service_policy_cli_bundle.py")
need, sha, read, encoded, unique = BASE.need, BASE.sha, BASE.read, BASE.encoded, BASE.unique


def load(raw):
    return json.loads(raw.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def exact(a, b):
    need(type(a) is type(b), "JSON type differs")
    if isinstance(a, dict):
        need(a.keys() == b.keys(), "JSON fields differ")
        for k in a:
            exact(a[k], b[k])
    elif isinstance(a, list):
        need(len(a) == len(b), "JSON array length differs")
        for x, y in zip(a, b):
            exact(x, y)
    else:
        need(a == b, "JSON value differs")


def member(root, relative, cap=MAX_SOURCE):
    need(stat.S_ISDIR(root.lstat().st_mode), "regular root directory required")
    need(relative and all(x not in ("", ".", "..") for x in relative.split("/"))
         and not relative.startswith("/") and "\\" not in relative, "unsafe member path")
    path = root
    for part in relative.split("/")[:-1]:
        path = path/part
        need(stat.S_ISDIR(path.lstat().st_mode), "directory symlink refused")
    return read(root/relative, cap)


def binary_platform(raw):
    if raw[:4] == b"\xcf\xfa\xed\xfe":
        need(len(raw) >= 32 and struct.unpack("<I", raw[4:8])[0] == 0x0100000C
             and struct.unpack("<I", raw[12:16])[0] == 2, "Mach-O architecture/type differs")
        return "darwin/arm64"
    need(len(raw) >= 64 and raw[:6] == b"\x7fELF\x02\x01", "native Mach-O or little-endian ELF64 required")
    need(struct.unpack("<HH", raw[16:20]) == (2, 62), "ELF architecture/type differs")
    offset = struct.unpack("<Q", raw[32:40])[0]
    size, count = struct.unpack("<HH", raw[54:58])
    need(size == 56 and count <= 64 and offset + size*count <= len(raw), "ELF program headers differ")
    need(all(struct.unpack("<I", raw[offset+i*size:offset+i*size+4])[0] not in (2, 3)
             for i in range(count)), "dynamic ELF interpreter/dependencies refused")
    return "linux/amd64"


def embedded_info(raw):
    magic = b"\xff Go buildinf:"
    offset = raw.find(magic)
    need(offset >= 0 and raw.find(magic, offset+1) == -1
         and raw[offset+14:offset+16] == bytes([8, 2]), "inline Go BuildInfo required")
    def uv(i):
        value = shift = 0
        while True:
            need(i < len(raw) and shift < 64, "invalid BuildInfo length")
            x = raw[i]
            value |= (x & 127) << shift
            i += 1
            if x < 128:
                return value, i
            shift += 7
    n, i = uv(offset+32)
    version = raw[i:i+n].decode()
    n, i = uv(i+n)
    data = raw[i:i+n]
    need(data[:16].hex() == "3077af0c9274080241e1c107e6d618e6"
         and data[-16:].hex() == "f932433186182072008242104116d8f2", "BuildInfo framing differs")
    result = {"GoVersion": version, "Path": None, "Main": None, "Deps": [], "Settings": []}
    for line in data[16:-16].decode().splitlines():
        fields = line.split("\t")
        if fields[0] == "path":
            need(result["Path"] is None, "duplicate build path")
            result["Path"] = fields[1]
        elif fields[0] in ("mod", "dep"):
            row = {"Path": fields[1], "Version": fields[2]}
            if len(fields) > 3 and fields[3]:
                row["Sum"] = fields[3]
            if fields[0] == "mod":
                need(result["Main"] is None, "duplicate main module")
                result["Main"] = row
            else:
                result["Deps"].append(row)
        elif fields[0] == "build":
            k, v = fields[1].split("=", 1)
            result["Settings"].append({"Key": k, "Value": v})
        else:
            raise ValueError("unsupported BuildInfo entry")
    return result


def build_identity(build, target, commit):
    need(re.fullmatch(r"[0-9a-f]{40}", commit), "exact source commit required")
    need(type(build) is dict and set(build) == {"GoVersion", "Path", "Main", "Deps", "Settings"}, "BuildInfo fields differ")
    need(build["GoVersion"] == "go1.27.2" and build["Path"] == MODULE, "Go version/main package differs")
    need(type(build["Main"]) is dict and set(build["Main"]) == {"Path", "Version"}
         and build["Main"]["Path"] == MODULE
         and re.fullmatch(r"v0\.0\.0-\d{14}-"+commit[:12], build["Main"]["Version"]), "main version/replacement differs")
    exact(build["Deps"], [row["dependency"] for row in DEPENDENCIES])
    settings = unique((row["Key"], row["Value"]) for row in build["Settings"])
    goos, arch = target.split("/")
    archkey, archvalue = ("GOARM64", "v8.0") if arch == "arm64" else ("GOAMD64", "v1")
    want = {"-buildmode": "exe", "-compiler": "gc", "-trimpath": "true", "CGO_ENABLED": "0",
            "GOOS": goos, "GOARCH": arch, archkey: archvalue, "vcs": "git",
            "vcs.revision": commit, "vcs.modified": "false", "vcs.time": settings.get("vcs.time")}
    need(re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", want["vcs.time"] or ""), "source time missing")
    exact(settings, want)
    return build


def source_contract(root, modcache):
    for name, digest in SOURCE_FILES.items():
        need(sha(member(root, name)) == digest, "MCP source/fixture differs: " + name)
    module = root/"examples/service-grant-mcp"
    actual = set()
    for current, dirs, names in os.walk(module, followlinks=False):
        need(all(not (Path(current)/x).is_symlink() for x in dirs), "MCP source directory symlink")
        for name in names:
            path = Path(current)/name
            need(stat.S_ISREG(path.lstat().st_mode), "MCP nonregular source")
            actual.add(path.relative_to(root).as_posix())
    need(actual == set(SOURCE_FILES), "complete MCP source roster differs")
    runtime = modcache/(SERVICE.MODULE+"@"+SERVICE.VERSION)
    archive = modcache/"cache/download"/SERVICE.MODULE/"@v"/(SERVICE.VERSION+".zip")
    generation, _ = SERVICE.module_source(runtime, archive, root)
    return generation


def checked_report(report, build, binary_sha, root, generation):
    need(type(report) is dict and set(report) == {"status", "observations", "binary_sha256", "build_identity", "sdk_client_calls", "model_calls", "external_adoption", "scope"}, "wire report fields differ")
    need(report["status"] == "INDEPENDENT_WIRE_TRIAL_PASS" and report["binary_sha256"] == binary_sha, "wire binary/acceptance differs")
    exact(report["build_identity"], build)
    for key in ("sdk_client_calls", "model_calls"):
        need(type(report[key]) is int and report[key] == 0, "wire client scope differs")
    need(report["external_adoption"] is False, "unexpected adoption claim")
    proofs = []
    for index in (1, 2, 3, 1):
        value = load(member(root, f"examples/service-grant-mcp/fixtures/control-{index}.stdout"))
        need(type(value) is list and len(value) == 1, "single native proof required")
        proof = value[0]
        need(set(proof) == {"facts", "decision", "bindings"} and len(proof["facts"]) == 4 and len(proof["decision"]) == 7, "whole proof fields differ")
        exact(proof["bindings"], {"compiler_source_sha": PIN, "generation_sha256": "sha256:"+GENERATION,
                                  "generation_origin": "fresh_pinned_generation", "assets": generation["assets"]})
        compact = json.dumps(proof, separators=(",", ":"), ensure_ascii=True).encode()
        proofs.append({"fixture": index, "status": proof["decision"]["status"], "proof_sha256": sha(compact)})
    errors = ["duplicate-key", "missing-field", "extra-field", "null", "wrong-type", "unpaired-surrogate", "array", "missing-arguments"]
    want = [{"protocol": p, "native_CLI_equal": proofs, "strict_tool_errors": errors,
             "unknown_tool_error": -32602, **SCHEMAS} for p in ("2025-11-25", "2026-07-28")]
    exact(report["observations"], want)


def notice_files(modcache, go_root, root):
    files, inventory = {}, []
    for index, row in enumerate(DEPENDENCIES, 1):
        dep = row["dependency"]
        base = modcache/(dep["Path"]+"@"+dep["Version"])
        for relative, digest in row["notices"].items():
            raw = member(base, relative)
            need(sha(raw) == digest, "dependency notice bytes differ")
            name = f"NOTICE-module-{index:02d}-"+relative.replace("/", "__")
            files[name] = (raw, 0o644)
            inventory.append({"file": name, "module": dep, "source_relative_path": relative, "sha256": digest})
    for index, (relative, row) in enumerate(GO_NOTICES.items(), 1):
        raw = member(go_root, relative)
        need(sha(raw) == row["sha256"], "Go toolchain notice bytes differ")
        name = f"NOTICE-Go-{index:02d}-"+relative.replace("/", "__")
        files[name] = (raw, 0o644)
        inventory.append({"file": name, "GoVersion": "go1.27.2", "source_relative_path": relative,
                          "sha256": row["sha256"], "scope": row["scope"]})
    for name, relative, digest in [
        ("LICENSE-transport-BSD", "examples/service-grant-mcp/LICENSE", "099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48"),
        ("LICENSE-generation-tool-Gooo-MIT", "tools/licenses/gooo-83928fa-MIT.txt", "5177457bc30b96eae6f786d0fbe3d1fda382edef61ecef25eb6f446a8b35b4f3")]:
        raw = member(root, relative)
        need(sha(raw) == digest, "transport/generator notice bytes differ")
        files[name] = (raw, 0o644)
        inventory.append({"file": name, "source_relative_path": relative, "sha256": digest,
                          "scope": "Generation tool attribution only; no Gooo compiler is included" if "Gooo" in name else "transport license"})
    need(len(files) == 55, "exact 55 notice texts required")
    return files, inventory


def safe_archive(path, files):
    need(all(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", n) for n in files), "flat safe archive names required")
    return SERVICE.verify_archive(path, files)


def extract_checked(path, files, directory):
    safe_archive(path, files)
    directory.mkdir(mode=0o700)
    with tarfile.open(path, "r:gz") as archive:
        for name, (content, mode) in files.items():
            with archive.extractfile(name) as source:
                extracted = source.read(len(content)+1)
            need(extracted == content, "archive extraction bytes differ")
            with (directory/name).open("xb") as stream:
                stream.write(extracted)
            (directory/name).chmod(0o700 if mode == 0o755 else 0o600)
    need(read(directory/COMMAND, BASE.MAX_BINARY) == files[COMMAND][0], "extracted executable differs")


def absent(pid):
    try:
        os.killpg(pid, 0)
        return False
    except ProcessLookupError:
        return True


class OwnedChildren:
    def __init__(self, output):
        self.output, self.children, self.records = output, [], []

    def spawn(self, *args, **kwargs):
        need(kwargs.get("start_new_session") is True, "new owned child session required")
        blocked = {signal.SIGTERM, signal.SIGINT, signal.SIGALRM}
        previous = signal.pthread_sigmask(signal.SIG_BLOCK, blocked)
        try:
            child = subprocess.Popen(*args, **kwargs)
            self.children.append(child)
            (self.output/"started-children.json").write_bytes(encoded([{"PID": p.pid, "PGID": p.pid} for p in self.children]))
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, previous)
        return child

    def close(self, first=None):
        for child in self.children:
            try:
                child.poll()  # Cache the direct leader's status before group cleanup.
                for sig in (signal.SIGTERM, signal.SIGKILL):
                    if absent(child.pid):
                        break
                    os.killpg(child.pid, sig)
                    try:
                        child.wait(timeout=.5)
                    except subprocess.TimeoutExpired:
                        pass
                child.wait(timeout=1)
                need(absent(child.pid), "owned child group remains")
                closed = all(s is None or s.closed for s in (child.stdin, child.stdout, child.stderr))
                if not closed:
                    for stream in (child.stdin, child.stdout, child.stderr):
                        if stream is not None and not stream.closed:
                            stream.close()
                    raise ValueError("verifier left owned child pipe objects open")
                self.records.append({"PID": child.pid, "PGID": child.pid, "exit_code": child.returncode,
                                     "direct_wait": True, "PG_ESRCH": True,
                                     "pipe_objects_closed": closed})
            except BaseException as error:
                if first is None:
                    first = error
                elif hasattr(first, "add_note"):
                    first.add_note("child cleanup: "+str(error))
        try:
            (self.output/"owned-child-closure.json").write_bytes(encoded(self.records))
        except BaseException as diagnostic:
            if first is None:
                first = diagnostic
            elif hasattr(first, "add_note"):
                first.add_note("child closure diagnostic: "+str(diagnostic))
        if first is not None:
            raise first


class BoundedText(io.StringIO):
    def write(self, value):
        need(len((self.getvalue()+value).encode()) <= MAX_STREAM, "diagnostic stream exceeds 64KiB")
        return super().write(value)


def native_verify(root, binary, build_path, output):
    """Use unchanged source verifier assertions; tighten IO and journal every child."""
    output.mkdir(mode=0o700)
    tracker = OwnedChildren(output)
    verifier_path = root/"examples/service-grant-mcp/verify.py"
    verifier = imported("mcp_bundle_wire_verifier", verifier_path)
    verifier.subprocess = types.SimpleNamespace(Popen=tracker.spawn, PIPE=subprocess.PIPE,
                                               TimeoutExpired=subprocess.TimeoutExpired)
    original_session = verifier.Session
    native_sessions = []
    native_stdout_total = 0
    class BoundedSession(original_session):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, **kwargs)
            if self.label in ("legacy-session", "current-session"):
                native_sessions.append(self)
        def read(self):
            nonlocal native_stdout_total
            need(time.monotonic()-self.started < 30, "session deadline")
            for key, _ in self.selector.select(.05):
                native = self.label in ("legacy-session", "current-session")
                cap = NATIVE_STDOUT_CAP if native and key.data == "stdout" else MAX_STREAM
                room = cap-len(self.buffers[key.data])
                data = os.read(key.fd, min(65536, room+1))
                if not data:
                    self.selector.unregister(key.fileobj)
                else:
                    self.buffers[key.data].extend(data[:room])
                    need(len(data) <= room, "native stdio stream exceeds scoped cap")
                    if native and key.data == "stdout":
                        native_stdout_total += len(data)
                        need(native_stdout_total <= NATIVE_STDOUT_TOTAL_CAP, "native stdout total exceeds 1MiB")
                    if key.data == "stdout":
                        self.linebuf.extend(data)
    verifier.Session = BoundedSession
    regression = imported("mcp_bundle_close_regression", root/"examples/service-grant-mcp/verify_close_test.py")
    close_out = output/"teardown"
    close_out.mkdir(mode=0o700)
    child = close_out/"exited-child.py"
    child.write_text("#!"+sys.executable+"\nimport os\nos.write(1,b'child stdout retained\\n')\nos.write(2,b'child stderr retained\\n')\nos._exit(0)\n")
    child.chmod(0o700)
    verifier.BINARY, verifier.OUTPUT = child, close_out
    regression.MODULE = verifier
    stdout, stderr, failure = BoundedText(), BoundedText(), None
    try:
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            result = unittest.TextTestRunner(stream=stderr, verbosity=2).run(
                unittest.defaultTestLoader.loadTestsFromTestCase(regression.CloseRegression))
            need(result.testsRun == 5 and result.wasSuccessful(), "existing teardown regression failed")
            need(len(regression.RECORDS) == 5 and all(r["waited"] and r["process_group_ESRCH"]
                 and r["stdout_closed"] and r["stderr_closed"] for r in regression.RECORDS), "teardown child closure incomplete")
            (close_out/"regression.json").write_bytes(encoded({"tests_run": result.testsRun, "success": True,
                                                             "actual_children": regression.RECORDS}))
            wire = output/"wire"
            wire.mkdir(mode=0o700)
            verifier.BINARY, verifier.OUTPUT, verifier.BUILD_INFO = binary, wire, build_path
            verifier.main()  # One unchanged finite two-protocol wire trial.
    except BaseException as error:
        failure = error
    finally:
        try:
            (output/"verifier.stdout").write_text(stdout.getvalue())
            (output/"verifier.stderr").write_text(stderr.getvalue())
            (output/"protocol-resource-budget.json").write_bytes(encoded({
                "outer_each_stream_cap": MAX_STREAM, "native_each_stdout_cap": NATIVE_STDOUT_CAP,
                "native_each_stderr_cap": MAX_STREAM, "two_native_stdout_total_cap": NATIVE_STDOUT_TOTAL_CAP,
                "native_stdout_total_received": native_stdout_total,
                "sessions": [{"label": s.label, "stdout_bytes": len(s.buffers["stdout"]),
                              "stderr_bytes": len(s.buffers["stderr"])} for s in native_sessions]}))
        except BaseException as error:
            if failure is None:
                failure = error
            elif hasattr(failure, "add_note"):
                failure.add_note("verifier diagnostics: "+str(error))
        tracker.close(failure)
    return load(read(output/"wire/verification.json", MAX_SOURCE))


def produce(binary, build_path, wire_path, modcache, go_root, root, out, commit,
            packager_commit=None, verify_native=False):
    need(out.is_dir() and not out.is_symlink() and not any(out.iterdir()), "fresh owned output directory required")
    need(verify_native or packager_commit is None, "committed packaging requires fresh extracted-native verification")
    need(packager_commit is None or re.fullmatch(r"[0-9a-f]{40}", packager_commit), "packager commit differs")
    work = out/"work"
    created, failure, result = [], None, None
    try:
        work.mkdir(mode=0o700)
        raw = read(binary, BASE.MAX_BINARY)
        need(binary.stat().st_mode & 0o111, "executable bit required")
        target = binary_platform(raw)
        build = load(read(build_path, MAX_SOURCE))
        exact(embedded_info(raw), build)
        build_identity(build, target, commit)
        generation = source_contract(root, modcache)
        report = load(read(wire_path, MAX_SOURCE))
        checked_report(report, build, sha(raw), root, generation)
        files, notices = notice_files(modcache, go_root, root)
        files[COMMAND] = (raw, 0o755)
        files["BUILD-INFO.json"] = (encoded(build), 0o644)
        files["NOTICE-INVENTORY.json"] = (encoded(notices), 0o644)
        guide = ("Native Gooo MCP service-grant assessment — "+target+"\n"
                 "Configure your MCP host's stdio command as the absolute path to gooo-service-grant-mcp, with no arguments.\n"
                 "No Go, Gooo compiler, Python, API key, HTTP server or model is required at runtime.\n"
                 "Tool: assess_service_grant. Arguments are an OBJECT: {\"before\":\"*\",\"after\":\"mcp://*\",\"required_type\":\"mcp\",\"required_name\":\"weather\"}.\n"
                 "This is not the preview12 evaluate CLI, which accepts a JSON array.\n"
                 "ELIGIBLE, REFUSED and INVALID_INPUT are decisions on supplied unsigned facts; no grant is applied.\n"
                 "Preserve complete facts, decision and 89 asset bindings. Report first diagnostics with synthetic public facts.\n"
                 "Some included Go toolchain notices concern compiler/test components; inventory scope is explicit.\n")
        files["START-HERE.txt"] = (guide.encode(), 0o644)
        manifest = {"schema": "gooo/service-grant-mcp-native-bundle/v1", "platform": target,
                    "source_commit": commit, "reference_module_bytes": REFERENCE_SOURCE,
                    "packager": {"commit": packager_commit, "kind": "uncommitted-private-prototype" if packager_commit is None else "committed-CI-candidate",
                                 "source_sha256": sha(read(Path(__file__), MAX_SOURCE)), "support_sources": SUPPORT_HASHES},
                    "compiler_source_sha": PIN, "generation_sha256": "sha256:"+GENERATION,
                    "generation_origin": "fresh_pinned_generation", "assets": generation["assets"],
                    "build_info": build, "binary_sha256": sha(raw), "notice_texts": 55,
                    "finite_checks": {"original_wire_report_binary_SHA_equal": True, "original_report_proofs": 8,
                                      "fresh_extracted_native_wire_requested": verify_native,
                                      "new_Linux_execution": verify_native and target == "linux/amd64",
                                      "grant_applied": False, "producer_authenticated": False, "external_adoption": False},
                    "members": {name: {"bytes": len(content), "sha256": sha(content), "mode": mode}
                                for name, (content, mode) in sorted(files.items())}}
        files["BUNDLE.json"] = (encoded(manifest), 0o644)
        need(len(files) == 60, "exact sixty archive members required")
        name = COMMAND+"-"+target.replace("/", "-")+".tar.gz"
        candidate = work/name
        SERVICE.write_archive(candidate, files)
        extracted = work/"extracted"
        extract_checked(candidate, files, extracted)
        new_report = None
        if verify_native:
            need(SERVICE.native_platform() == target, "native runtime platform differs")
            new_report = native_verify(root, extracted/COMMAND, extracted/"BUILD-INFO.json", out/"native-verification")
            checked_report(new_report, build, sha(raw), root, generation)
        need(read(extracted/COMMAND, BASE.MAX_BINARY) == raw == read(binary, BASE.MAX_BINARY), "binary changed during checks")
        final_raw = read(candidate, BASE.MAX_ARCHIVE)
        shutil.rmtree(work)
        need(not work.exists(), "owned work cleanup incomplete")
        final = out/name
        with final.open("xb") as stream:
            created.append(final)
            stream.write(final_raw)
        checked = safe_archive(final, files)
        checksum = out/(name+".sha256")
        checksum_raw = (checked["sha256"]+"  "+name+"\n").encode()
        with checksum.open("xb") as stream:
            created.append(checksum)
            stream.write(checksum_raw)
        need(read(checksum, 4096) == checksum_raw, "checksum readback differs")
        result = {"status": "PRIVATE_PROTOTYPE_ARCHIVE_VERIFIED" if packager_commit is None else "CI_NATIVE_ARCHIVE_VERIFIED",
                  "platform": target, "archive": checked, "checksum": {"name": checksum.name, "sha256": sha(checksum_raw)},
                  "source_commit": commit, "binary_sha256": sha(raw), "members": sorted(files), "manifest": manifest,
                  "extracted_binary_SHA_equal": True, "new_extracted_native_wire": new_report is not None,
                  "Linux_scope": "fresh native stdio" if verify_native and target == "linux/amd64" else "no new Linux execution; previous original-CI byte equivalence only",
                  "owned_work_removed": True, "not_published": True}
        (out/"verification.json").write_bytes(encoded(result))
    except BaseException as error:
        failure = error
        for path in reversed(created):
            try:
                path.unlink()
            except BaseException as cleanup:
                if hasattr(error, "add_note"):
                    error.add_note("partial artifact cleanup: "+str(cleanup))
    finally:
        if work.exists():
            try:
                need(not work.is_symlink() and work.stat().st_uid == os.getuid(), "owned work identity differs")
                shutil.rmtree(work)
            except BaseException as cleanup:
                if failure is None:
                    failure = cleanup
                elif hasattr(failure, "add_note"):
                    failure.add_note("owned work cleanup: "+str(cleanup))
        if failure is not None:
            try:
                (out/"first-failure.json").write_bytes(encoded({"status": "REFUSED", "error": repr(failure), "owned_work_removed": not work.exists()}))
            except BaseException as diagnostic:
                if hasattr(failure, "add_note"):
                    failure.add_note("first-failure diagnostic: "+str(diagnostic))
    if failure is not None:
        raise failure
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("binary", "build-info", "wire-report", "module-cache", "go-root", "root", "out"):
        parser.add_argument("--"+name, type=Path, required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--packager-commit")
    parser.add_argument("--verify-native", action="store_true")
    args = parser.parse_args()
    def interrupted(number, _frame):
        raise RuntimeError("packager interrupted by signal "+str(number))
    for sig in (signal.SIGINT, signal.SIGTERM, signal.SIGALRM):
        signal.signal(sig, interrupted)
    signal.setitimer(signal.ITIMER_REAL, 180)
    args.out.mkdir(mode=0o700, parents=True, exist_ok=True)
    try:
        result = produce(args.binary.absolute(), args.build_info.absolute(), args.wire_report.absolute(),
                         args.module_cache.absolute(), args.go_root.absolute(), args.root.absolute(),
                         args.out.absolute(), args.source_commit, args.packager_commit, args.verify_native)
        print(json.dumps({"status": result["status"], "platform": result["platform"],
                          "archive_sha256": result["archive"]["sha256"], "new_native_wire": result["new_extracted_native_wire"]}))
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)


if __name__ == "__main__":
    main()
