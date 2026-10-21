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
import zipfile
REFERENCE_SOURCE = '9c29602f77492be0e60c0ef91afd6d3968890b24'

SOURCE_FILES = {'examples/service-grant-mcp/LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
 'examples/service-grant-mcp/README.md': '8ff5048ec7140f260af7c2289ed42fa6605cae0a781a260143ad29dc89e89c24',
 'examples/service-grant-mcp/fixtures/control-1.input': 'eaa120445bed9038ab6d9a4e71949344f6d92c44bad73fc7d713b8fc17c0dcb4',
 'examples/service-grant-mcp/fixtures/control-1.stdout': 'd16eee837f400c55fa49f6bbdef280d4747976efa8e4a00e1eb8b71a8a8e95c3',
 'examples/service-grant-mcp/fixtures/control-2.input': '947f10f40652f17eb97c9f7d7a372a493a9eca5e24976ab7dbbe9c1863bac34e',
 'examples/service-grant-mcp/fixtures/control-2.stdout': '351f9f89715a05e8a9f2b3374c2a581c01aa2dfcdc0274f44f1870003a48fb5c',
 'examples/service-grant-mcp/fixtures/control-3.input': 'a9f4c61b9e64f46c6c81b94610ed3d5181b735d3532eb10417b6e52da01c2bbc',
 'examples/service-grant-mcp/fixtures/control-3.stdout': '0b26cef07c173b573b3423720840b762ab6a35480029f716b55de56bb27bfb40',
 'examples/service-grant-mcp/fixtures/control-4.input': 'd744fc59eceac73a8cc68430047424c5b065137f4807daea9a411318a220cc0a',
 'examples/service-grant-mcp/fixtures/control-4.stdout': 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855',
 'examples/service-grant-mcp/fixtures/origin.json': '40915248a7743baaaf307bdf620fa2386d46790b255b1e90a58cc17d6142423c',
 'examples/service-grant-mcp/go.mod': '2dd303dce968a4c2b7fa8d1dc820b94c3bc53bcd8535fc7e14d3847179a306d4',
 'examples/service-grant-mcp/go.sum': 'a85b704ac364c538f41594af4a1e8bcb2850a5769376072f7c59d92a5fdf7678',
 'examples/service-grant-mcp/main.go': '57474ebcacd51849adbcdbf9b9a47b77392748a96a2cf90cade28a69640b4674',
 'examples/service-grant-mcp/main_test.go': 'bfd6a2b45c6ffeb293e0b68b0a9afa08a02e16409ba8b9e0ff7aa9e1e5e9f6d6',
 'examples/service-grant-mcp/verify.py': '0fd5a5f80748551cd177d05d2913c11c158d409d61464109c27da0bc21d98612',
 'examples/service-grant-mcp/verify_close_test.py': '63076fe685edbed02e8d6ebb821b1660f6c6504ca110ac44dcf5a01e5b523375',
 'examples/service-grant-mcp/verify_many.py': '2fb1602cc76f17b0c9ed8e8e3187560e9673d5db33bbc600b503ed905ded56cf'}

DEPENDENCIES = [{'dependency': {'Path': 'github.com/google/jsonschema-go',
                 'Sum': 'h1:/DBOLZTfDow7pe2GmaJNhltueGTtDKICi8V8p+DQPd0=',
                 'Version': 'v0.4.3'},
  'notices': {'LICENSE': '2d56c53449691d85d9aea245eb8dac12713e9075d70d5557b82ae1e94805b357'}},
 {'dependency': {'Path': 'github.com/kimjooyoon/gooo-go-ports/examples/agentmesh-service-policy',
                 'Sum': 'h1:wiJUz5Om53/Ng69iQj/43qBKD7DSuqlqrdGLr6HgZDA=',
                 'Version': 'v0.0.0-20261020182808-7180a4a0f36a'},
  'notices': {'LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
              'LICENSE-APACHE-2.0': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4',
              'NOTICE.md': '90163940130087486fa104d9db6e9c56da1c7de37c2e32364340c0cdc367bc30',
              'upstream/LICENSE': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4'}},
 {'dependency': {'Path': 'github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many',
                 'Sum': 'h1:p3zs2m5zxSwlgn0Z3Wq1rl+cfcBN4JN2JhrmjhARgkk=',
                 'Version': 'v0.0.0-20261020182808-7180a4a0f36a'},
  'notices': {'LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
              'LICENSE-APACHE-2.0': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4',
              'NOTICE.md': 'abd202e771c34e66987ee4b9bc006acb0841ea6321abf21f7dd819e36915779d'}},
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
 'src/cmd/compile/internal/ssa/_gen/vendor/golang.org/x/tools/LICENSE': {'scope': 'compiler/tooling '
                                                                                  'distribution only; not '
                                                                                  'claimed included in MCP '
                                                                                  'executable',
                                                                         'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/compile/internal/ssa/_gen/vendor/golang.org/x/tools/PATENTS': {'scope': 'compiler/tooling '
                                                                                  'distribution only; not '
                                                                                  'claimed included in MCP '
                                                                                  'executable',
                                                                         'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/github.com/google/pprof/LICENSE': {'scope': 'compiler/tooling distribution only; not '
                                                             'claimed included in MCP executable',
                                                    'sha256': 'cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30'},
 'src/cmd/vendor/github.com/google/pprof/third_party/svgpan/LICENSE': {'scope': 'compiler/tooling '
                                                                                'distribution only; not '
                                                                                'claimed included in MCP '
                                                                                'executable',
                                                                       'sha256': '154c946c17de61ca71d28e13673d76a5798e6e25995a7397fbae157afc8f62b4'},
 'src/cmd/vendor/github.com/ianlancetaylor/demangle/LICENSE': {'scope': 'compiler/tooling distribution only; '
                                                                        'not claimed included in MCP '
                                                                        'executable',
                                                               'sha256': 'b73277f7730993ef8e7f80aed83e5309762fed4d77eced982abb2c00657c074d'},
 'src/cmd/vendor/golang.org/x/arch/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '8cda009bd927676a95ee7ce2dae921442ac4d9584d041b56ecfcba6839032f0b'},
 'src/cmd/vendor/golang.org/x/arch/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/build/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                        'included in MCP executable',
                                               'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/build/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                        'included in MCP executable',
                                               'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/mod/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                      'included in MCP executable',
                                             'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/mod/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                      'included in MCP executable',
                                             'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/sync/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/sync/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/sys/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                      'included in MCP executable',
                                             'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/sys/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                      'included in MCP executable',
                                             'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/telemetry/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                            'included in MCP executable',
                                                   'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/telemetry/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                            'included in MCP executable',
                                                   'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/term/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/term/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/text/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/text/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                       'included in MCP executable',
                                              'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/golang.org/x/tools/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                        'included in MCP executable',
                                               'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/cmd/vendor/golang.org/x/tools/PATENTS': {'scope': 'compiler/tooling distribution only; not claimed '
                                                        'included in MCP executable',
                                               'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/cmd/vendor/rsc.io/markdown/LICENSE': {'scope': 'compiler/tooling distribution only; not claimed '
                                                     'included in MCP executable',
                                            'sha256': '2d36597f7117c38b006835ae7f537487207d8ec407aa9d9980794b2030cbc067'},
 'src/crypto/internal/boring/LICENSE': {'scope': 'toolchain component text; inclusion in this MCP executable '
                                                 'not established',
                                        'sha256': '56210f826b8f0fbac3160dfe55c97f4019eb6cdda3963b5a0eab8a8bdb62360e'},
 'src/runtime/testdata/testgoroutineleakprofile/goker/LICENSE': {'scope': 'toolchain testdata only; not '
                                                                          'claimed included in MCP '
                                                                          'executable',
                                                                 'sha256': 'f99df8ae6da6de047ed002d9dbe8682d9a19c88e9a7473abdadf3ac206621a85'},
 'src/vendor/golang.org/x/crypto/LICENSE': {'scope': 'Go standard-library vendored component text; specific '
                                                     'linked packages not derived in this task',
                                            'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/crypto/PATENTS': {'scope': 'Go standard-library vendored component text; specific '
                                                     'linked packages not derived in this task',
                                            'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/vendor/golang.org/x/net/LICENSE': {'scope': 'Go standard-library vendored component text; specific '
                                                  'linked packages not derived in this task',
                                         'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/net/PATENTS': {'scope': 'Go standard-library vendored component text; specific '
                                                  'linked packages not derived in this task',
                                         'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/vendor/golang.org/x/sys/LICENSE': {'scope': 'Go standard-library vendored component text; specific '
                                                  'linked packages not derived in this task',
                                         'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/sys/PATENTS': {'scope': 'Go standard-library vendored component text; specific '
                                                  'linked packages not derived in this task',
                                         'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'},
 'src/vendor/golang.org/x/text/LICENSE': {'scope': 'Go standard-library vendored component text; specific '
                                                   'linked packages not derived in this task',
                                          'sha256': '911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad'},
 'src/vendor/golang.org/x/text/PATENTS': {'scope': 'Go standard-library vendored component text; specific '
                                                   'linked packages not derived in this task',
                                          'sha256': '96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc'}}

SUPPORT_HASHES = {'api_policy_cli_bundle.py': '2efcaa9a75148cd189748468e50d25e8abe4db5c46637253ad66ac5ac1a0f9ad',
 'service_policy_cli_bundle.py': '3431cdcdacb194fad52e5efe77c5a188ab6a41bce0fc28fe781d075f148c2c20'}

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
GENERATION = '3c86dd9273168f1fe4e0712aa7cfdfc075a211d05b64ff2fbd0b8c9135315a3b'
PIN = '205896a413e5dea92366309c35ae85e542d055f6'


MANY_MODULE = 'github.com/kimjooyoon/gooo-go-ports/examples/service-grant-many'
MANY_VERSION = 'v0.0.0-20261020182808-7180a4a0f36a'
MANY_SUM = 'h1:p3zs2m5zxSwlgn0Z3Wq1rl+cfcBN4JN2JhrmjhARgkk='
MANY_MOD_SUM = 'h1:E0cBwR8ybR1+jlHM6zdepav8UP3gwHQuFPpw35R5C6k='
MANY_PIN = '205896a413e5dea92366309c35ae85e542d055f6'
MANY_GENERATION = '86d714e4793f04d7d45d53e824cd7066ddee5f7d1e987697f26f4216a6b4e164'
MANY_FILES = {'LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
 'LICENSE-APACHE-2.0': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4',
 'NOTICE.md': 'abd202e771c34e66987ee4b9bc006acb0841ea6321abf21f7dd819e36915779d',
 'README.md': 'ba027040f55031a430190b16f10635940ce927c33e475991bfe8e6e179c3eb0b',
 'all-cases.json': '0bd2386252984cb2e1aaf7297e9263e71d221b03b3a55e05788aa8bad2abc165',
 'bindings.go': '560045a704dea6fd85c7a33a97bdfc4f4f8ef70500ef10da17432127a28cdc8a',
 'bridge_generated.go': 'e4b88a9db690bec6077ec5d0d26c605149e4946ad121781b4ca7e44b7d4a2cd9',
 'case-facts.json': 'f31e31fe2f9d017086a3189507602c4cac7fd9d23dabb57f31f861bd3004ef10',
 'cmd/evaluate/main.go': 'e43dd9533b14d9b85a7da4916aa71dff7ee92ad78d1cc74c4aa811920766c2f5',
 'cmd/evaluate/main_test.go': '5c6f5aac865e8c9e17aaee20bc313b6c2d39d4393ff40e4e12f07ad5d5f883ca',
 'compiler-pin.json': 'd25ca85cb16b4b89664838eccdc896439d62400766f997b9dca6a43d69f2b5d8',
 'compiler.json': '503d1daed25690a552b207f0f8b3a19ebff309cfe8127f72d43be50059a4c428',
 'generate.py': '9a4c54cef5788acd296cfd7d1860b9ffa94a349eb9306d6eaa934985dfa1880f',
 'generation.json': '86d714e4793f04d7d45d53e824cd7066ddee5f7d1e987697f26f4216a6b4e164',
 'go.mod': '1ce2c12582bbbffdf68fdf75568ca571ffaf7334b03c8e376a32944d12fb406e',
 'input.go': '1c24ea415bd88ea9c50370df3de4f735a29910fae0dd83ac76ae4cccdce9b451',
 'model.go': 'f3f296ffa4d93244b19261af3c1436e948b6ab333d7f16c0a13459b8b31b51bd',
 'normal-cases.json': 'a88fb3846ea66dfb3e81d808f1f390cdcbbafeea7aef49fbc94bed8420feeea5',
 'policy_test.go': 'c6871032df101c4256528a2315de9cb93cb41655c5afb8877addb38c762cef17',
 'projection_generated.go': '530eb18b45065838ddd04c0f0a00c9dc08e582b9c7d629fc4b8a8aac0e1ceb36',
 'reference-cases.json': '1bf13f43ed05f56c77da79e0eac716cb059a1d7afef67acf841414a57f8518e9',
 'reference-expected.json': '5c4a1cc34a0b8c3b94d694a295149be15cfa05c17189bf82eb4a1be8b12af895',
 'remaining-cases.json': '711253c360d8244173dc180da40abc7e801654b59c5c34d5a84cf114a51a9760',
 'source-provenance.json': 'ca5e4401702346b91ef420f58df5229af900b29b7b8d843e68de2895a6e94896',
 'verify.py': 'ff3d94c39782619e06f64d5fbcfc3218733013332293f07b14368e5e939d3f0f',
 'verify_saved.py': '36d41d580d75353ac2027cd46f54a106f0a7cc467ad248a8190a4cb97f067974'}
MANY_REFERENCE_ASSETS = {'reference/LICENSE': '099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48',
 'reference/go.mod': '8b3744d0552c31de216c0766457bbbea1de2c4c55b702e9f82ce3490d0b21329',
 'reference/main.go': 'f21b3ea164c8df4b201cfc1de14b1b416247a343d720d0125b6d0fd71223c58f',
 'reference/source-pin.json': '4196b726d4f596363131994d637960d8750ec27ca0b0b5165e7d6e634bd3d25a',
 'reference/upstream/LICENSE': 'c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4',
 'reference/upstream/extracted.go': '01efb5b882fa532d537e8051148bba780af01806c5e5ee328a2b9327bde1c2fe'}
MANY_PUBLIC_SOURCES = {'examples/agentmesh-service-policy/checked_refinement.gooo': 'sha256:6628f15bf8a260e48bc7b58e5d05699c48138b145a1e42c6e6f872efd6f81772',
 'examples/agentmesh-service-policy/domain.gooo': 'sha256:b866ea5aa827e6122541faa54fd920c9ec1942a308bf72adb82fb244f1b3c33c',
 'examples/agentmesh-service-policy/refinement.gooo': 'sha256:90bcc738c88eb5bec8b41b5306cccf274a5740560d7dd62b7f3a57b30b441ff0',
 'ports/agentmesh/service_pattern.gooo': 'sha256:e9aabc13addf3db59a3a5448f8ad2e01e8b731a6da5f6e5eac7b749bf93bda7d',
 'ports/service-policy/multi_required.gooo': 'sha256:097e2304fdd70e47d31f7295c8ea5de161afa4f8ab4c2d87a435a993513332ba'}

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
    need(build["GoVersion"] == "go1.27.3" and build["Path"] == MODULE, "Go version/main package differs")
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
    many_generation, _ = many_module_source(modcache, root)
    return generation, many_generation


def structural_schema(schema):
    """Project descriptive annotations; retain every validation keyword.

    The source-pinned wire verifier separately checks descriptions/examples
    and executes the discovery example. Original structural contracts remain
    unchanged; unknown or altered constraints cannot disappear here.
    """
    if isinstance(schema, dict):
        result = {}
        for key, value in schema.items():
            if key == "description":
                need(type(value) is str and bool(value.strip()), "empty schema description")
            elif key == "examples":
                need(schema.get("type") == "string" and type(value) is list and bool(value)
                     and all(type(x) is str for x in value), "invalid string examples")
            else:
                result[key] = structural_schema(value)
        return result
    if isinstance(schema, list):
        return [structural_schema(value) for value in schema]
    return schema


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
             "discovery_example_native_CLI_equal": True,
             "unknown_tool_error": -32602, **SCHEMAS} for p in ("2025-11-25", "2026-07-28")]
    observations = []
    for observed in report["observations"]:
        row = dict(observed)
        for key in ("input_schema", "output_schema"):
            row[key] = structural_schema(row[key])
        observations.append(row)
    exact(observations, want)


def many_module_source(modcache, root):
    """Verify the public nested module separately from the legacy112-file module."""
    directory = modcache/(MANY_MODULE+"@"+MANY_VERSION)
    archive_path = modcache/"cache/download"/MANY_MODULE/"@v"/(MANY_VERSION+".zip")
    prefix = MANY_MODULE+"@"+MANY_VERSION+"/"
    files = {}
    with zipfile.ZipFile(io.BytesIO(read(archive_path, 1 << 20))) as archive:
        rows = archive.infolist()
        need(len(rows) == 26 and sum(row.file_size for row in rows) == 261017,
             "complete many module roster/size differs")
        for row in rows:
            name = row.filename
            relative = name.removeprefix(prefix)
            need(name.startswith(prefix) and relative in MANY_FILES and name not in files
                 and "\\" not in name and "\n" not in name and "\r" not in name
                 and all(part not in ("", ".", "..") for part in relative.split("/"))
                 and not row.is_dir() and row.file_size <= MAX_SOURCE
                 and (row.external_attr >> 16) & 0o170000 in (0, stat.S_IFREG),
                 "unsafe, duplicate or foreign many module member")
            content = archive.read(row)  # Full ZIP CRC and declared-size validation.
            need(sha(content) == MANY_FILES[relative], "many module source bytes differ: "+relative)
            need(member(directory, relative) == content, "many installed module differs from ZIP")
            files[name] = content
    need(SERVICE.dirhash(files) == MANY_SUM, "many public module h1 differs")
    relative_files = {name[len(prefix):]: raw for name, raw in files.items()}
    actual = set()
    need(stat.S_ISDIR(directory.lstat().st_mode), "regular many module directory required")
    for current, dirs, names in os.walk(directory, followlinks=False):
        need(all(not (Path(current)/x).is_symlink() for x in dirs), "many module directory symlink")
        for name in names:
            path = Path(current)/name
            need(stat.S_ISREG(path.lstat().st_mode), "many module nonregular file")
            actual.add(path.relative_to(directory).as_posix())
    need(actual == set(MANY_FILES), "many installed module complete roster differs")
    need(SERVICE.dirhash({"go.mod": relative_files["go.mod"]}) == MANY_MOD_SUM,
         "many go.mod h1 differs")
    generation_raw = relative_files["generation.json"]
    need(sha(generation_raw) == MANY_GENERATION, "many generation bytes differ")
    generation = load(generation_raw)
    need(generation.get("schema") == "gooo/service-grant-many-generation/v1"
         and generation.get("origin") == "fresh_pinned_generation"
         and generation.get("compiler_source_sha") == MANY_PIN
         and type(generation.get("assets")) is dict and len(generation["assets"]) == 30
         and type(generation.get("public_sources")) is dict and len(generation["public_sources"]) == 5,
         "many generation identity/scope differs")
    embedded, outside = 0, 0
    for relative, digest in generation["assets"].items():
        need(type(digest) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", digest), "many asset hash differs")
        if relative in relative_files:
            need(digest == "sha256:"+sha(relative_files[relative]), "many embedded asset bytes differ")
            embedded += 1
        else:
            need(relative in MANY_REFERENCE_ASSETS
                 and digest == "sha256:"+MANY_REFERENCE_ASSETS[relative]
                 and digest == "sha256:"+sha(member(root, "examples/service-grant-many/"+relative)),
                 "many omitted nested-reference asset bytes differ")
            outside += 1
    need(embedded == 24 and outside == 6, "many embedded/reference asset partition differs")
    exact(generation["public_sources"], MANY_PUBLIC_SOURCES)
    for relative, digest in generation["public_sources"].items():
        need(digest == "sha256:"+sha(member(root, relative)), "many public Gooo source bytes differ")
    need(member(root, "examples/service-grant-many/generation.json") == generation_raw,
         "packaging checkout has another many generation")
    return generation, directory


def many_result(facts, decision, generation):
    return {"facts": facts, "decision": decision,
            "bindings": {"compiler_source_sha": MANY_PIN,
                         "generation_sha256": "sha256:"+MANY_GENERATION,
                         "sources": generation["public_sources"], "assets": generation["assets"]}}


def many_summary(proof):
    return {"status": proof["decision"]["status"], "reason": proof["decision"]["reason"],
            "facts_sha256": sha(json.dumps(proof["facts"], separators=(",", ":"), ensure_ascii=True).encode()),
            "full_proof_sha256": sha(json.dumps(proof, separators=(",", ":"), ensure_ascii=True).encode())}


def checked_many_negative_requests(request_lines, first_facts):
    """Bind every refusal to the exact malformed bytes the pinned client sends."""
    base = json.dumps({"before": first_facts["before"], "after": first_facts["after"],
        "required_requests": [{"type": kind, "name": name} for kind, name in
            zip(first_facts["required_types"], first_facts["required_names"])]},
        separators=(",", ":"), ensure_ascii=True).encode()
    maximum = b"{" + b" "*((1 << 20)-len(base)) + base[1:]
    need(len(maximum) == 1 << 20, "many argument fixture size differs")
    exact_arguments = {
        20: maximum,
        21: b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},{"type":"mcp","name":"weather","name":"calculator"}]}',
        22: b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},null]}',
        23: b'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},{"type":"mcp","name":"weather","decision":"ELIGIBLE"}]}',
        24: br'{"before":"*","after":"mcp://weather","required_requests":[{"type":"mcp","name":"calculator"},{"type":"mcp","name":"\ud800"}]}',
        25: maximum[:-1] + b" }",
    }
    for identifier, expected in exact_arguments.items():
        raw = request_lines[identifier]
        prefix, separator, tail = raw.rpartition(b',"arguments":')
        need(separator and tail.endswith(b"}}"), "many refusal argument framing differs")
        # Strictly parse the outer request without reparsing/re-encoding its
        # deliberately duplicate/surrogate/over-limit argument object.
        outer = load(prefix + b"}}")
        need(type(outer) is dict and set(outer) == {"jsonrpc", "id", "method", "params"}
             and outer["jsonrpc"] == "2.0" and type(outer["id"]) is int
             and outer["id"] == identifier and outer["method"] == "tools/call",
             "many refusal method/identity differs")
        params = outer["params"]
        need(type(params) is dict and {"name"} <= set(params) <= {"name", "_meta"}
             and params["name"] == "assess_service_grant_change",
             "many refusal tool differs")
        need(tail[:-2] == expected, "many refusal exact argument bytes differ: "+str(identifier))


def checked_many_report(report_path, binary, root, modcache, generation):
    report = load(read(report_path, MAX_SOURCE))
    need(type(report) is dict and set(report) == {"status", "binary", "legacy_verifier", "generation",
         "independent_expected", "observations", "actual_protocol_sessions", "Go_compiler_or_model_calls",
         "live_authority_or_application", "other_hosts_OSes_or_external_adoption_proven"},
         "many wire report fields differ")
    need(report["status"] == "PRIVATE_MANY_MCP_TWO_PROTOCOL_WIRE_REFERENCE_CASES_ACCEPTED",
         "many wire report status differs")
    module = modcache/(MANY_MODULE+"@"+MANY_VERSION)
    for key, path in [("binary", binary), ("legacy_verifier", root/"examples/service-grant-mcp/verify.py"),
                      ("generation", module/"generation.json"),
                      ("independent_expected", module/"reference-expected.json")]:
        content = read(path, BASE.MAX_BINARY if key == "binary" else MAX_SOURCE)
        exact(report[key], {"path": str(path), "bytes": len(content), "sha256": sha(content)})
    need(type(report["actual_protocol_sessions"]) is int and report["actual_protocol_sessions"] == 2
         and type(report["Go_compiler_or_model_calls"]) is int and report["Go_compiler_or_model_calls"] == 0
         and report["live_authority_or_application"] is False
         and report["other_hosts_OSes_or_external_adoption_proven"] is False,
         "many wire scope differs")
    reference = load(member(module, "reference-expected.json"))
    need(type(reference) is list and len(reference) == 16, "many reference population differs")
    paired = [row for row in reference if len(row["facts"]["required_types"]) == len(row["facts"]["required_names"])]
    need(len(paired) == 15 and [x["name"] for x in reference if x not in paired] == ["length-mismatch"],
         "many pair-transport population differs")
    many_proofs = [many_result(row["facts"], row["decision"], generation) for row in paired]
    boundary = many_result({"before": "*", "after": "mcp://*", "required_types": ["mcp"]*257,
        "required_names": ["weather"]*257}, {"status": "INVALID_INPUT", "reason": "REQUIRED_REQUEST_LIMIT_EXCEEDED",
        "invalid_field": "required_types", "invalid_index": -1, "first_counterexample": -1,
        "request_count": 257, "checked_count": 0, "all_preserved": False, "supported_subset": False,
        "subset_rule_id": "", "counterexample_before_match": False, "counterexample_after_match": False}, generation)
    legacy = load(member(root, "examples/service-grant-mcp/fixtures/control-1.stdout"))[0]
    legacy_summary = {"fixture": 1, "status": legacy["decision"]["status"],
        "proof_sha256": sha(json.dumps(legacy, separators=(",", ":"), ensure_ascii=True).encode())}
    errors = ["later-duplicate", "later-null", "later-unknown", "later-surrogate", "argument-byte-limit-plus-one"]
    observations = [{"protocol": protocol, "legacy_tool_case": legacy_summary,
        "reference_cases": [{"name": case["name"], **many_summary(proof)} for case, proof in zip(paired, many_proofs)],
        "native_policy257": many_summary(boundary), "argument_bytes_at_limit": 1 << 20,
        "structural_failures_no_partial_proof": errors, "after_error_repeat_equal": True}
        for protocol in ("2025-11-25", "2026-07-28")]
    exact(report["observations"], observations)
    # Independently compare the retained whole wire proofs and closure records;
    # report PASS/hash summaries alone are not enough for a distribution candidate.
    for label, protocol in [("many-legacy-session", "2025-11-25"), ("many-current-session", "2026-07-28")]:
        process = load(member(report_path.parent, label+"/process.json"))
        for key in ("streams_EOF", "waited", "process_group_ESRCH", "all_stdout_frames_parsed",
                    "runtime_PATH_empty", "runtime_GOROOT_absent"):
            need(process.get(key) is True, "many wire process closure differs: "+key)
        need(type(process.get("pid")) is int and process["pid"] > 0
             and type(process.get("exit_code")) is int and process["exit_code"] == 0
             and process.get("failure") is None and process.get("stdin_close_error") is None
             and process.get("teardown_error") is None
             and process.get("server_created_work_files") == []
             and process.get("client_uses_MCP_SDK") is False
             and type(process.get("model_calls")) is int and process["model_calls"] == 0,
             "many wire direct process result differs")
        need(member(report_path.parent, label+"/stderr", MAX_STREAM) == b""
             and process.get("stderr_bytes") == 0, "many wire stderr differs")
        received = member(report_path.parent, label+"/stdout", 2 << 20)
        need(received.endswith(b"\n"), "many wire stdout frame EOF differs")
        frames = [load(line) for line in received.splitlines()]
        need(len(frames) == 26 and process.get("protocol_frames") == 26
             and all(type(frame) is dict and frame.get("jsonrpc") == "2.0"
                     and type(frame.get("id")) is int and frame["id"] == i for i, frame in enumerate(frames, 1)),
             "many wire response IDs/population differ")
        sent_raw = member(report_path.parent, label+"/stdin", 3 << 20)
        need(sent_raw.endswith(b"\n"), "many wire input frame EOF differs")
        # Intentionally malformed argument objects are retained in negative inputs.
        # Strict parsing still applies to every positive request and all responses.
        sent_lines = sent_raw.splitlines()
        sent = [json.loads(line) for line in sent_lines]
        need(all(type(frame.get("id")) is int for frame in sent if "id" in frame), "many input ID type differs")
        request_lines = {frame["id"]: line for frame, line in zip(sent, sent_lines) if "id" in frame}
        requests = {frame["id"]: frame for frame in sent if "id" in frame}
        need(len(requests) == 26 and set(requests) == set(range(1, 27)), "many wire input IDs differ")
        positives = {3: legacy, **{i: proof for i, proof in enumerate(many_proofs, 4)},
                     19: boundary, 20: many_proofs[0], 26: many_proofs[0]}
        for identifier, proof in positives.items():
            request = load(request_lines[identifier])
            need(request["method"] == "tools/call", "many proof request method differs")
            facts = proof["facts"]
            arguments = facts if identifier == 3 else {"before": facts["before"], "after": facts["after"],
                "required_requests": [{"type": kind, "name": name}
                    for kind, name in zip(facts["required_types"], facts["required_names"])]}
            exact(request["params"]["arguments"], arguments)
            need(request["params"]["name"] == ("assess_service_grant" if identifier == 3 else "assess_service_grant_change"),
                 "many proof tool name differs")
            frame = frames[identifier-1]
            need(set(frame) == {"jsonrpc", "id", "result"}, "many proof is a protocol error")
            result = frame["result"]
            allowed = {"content", "structuredContent", "isError"}
            if protocol == "2026-07-28":
                allowed |= {"resultType", "_meta"}
                need(result.get("resultType") == "complete", "many modern result is not complete")
                exact(result.get("_meta"), {"io.modelcontextprotocol/serverInfo": {"name": "gooo-service-grant", "version": "0.1.0-dev"}})
            need(type(result) is dict and set(result) <= allowed
                 and type(result.get("isError", False)) is bool and result.get("isError", False) is False,
                 "many complete result fields differ")
            exact(result["structuredContent"], proof)
            need(type(result["content"]) is list and len(result["content"]) == 1
                 and set(result["content"][0]) == {"type", "text"}
                 and result["content"][0]["type"] == "text", "many proof text shape differs")
            exact(load(result["content"][0]["text"].encode()), proof)
        checked_many_negative_requests(request_lines, many_proofs[0]["facts"])
        for identifier in range(21, 26):
            frame = frames[identifier-1]
            if "error" in frame:
                need(set(frame) == {"jsonrpc", "id", "error"} and type(frame["error"]) is dict
                     and set(frame["error"]) <= {"code", "message"}
                     and type(frame["error"].get("code")) is int and frame["error"]["code"] == -32602
                     and type(frame["error"].get("message")) is str and frame["error"]["message"],
                     "many negative protocol result differs")
            else:
                need(set(frame) == {"jsonrpc", "id", "result"}, "many negative frame fields differ")
                result = frame["result"]
                allowed = {"isError", "content"} | ({"resultType", "_meta"} if protocol == "2026-07-28" else set())
                need(type(result) is dict and set(result) <= allowed and result.get("isError") is True,
                     "many structural failure returned a partial proof")
                if protocol == "2026-07-28":
                    need(result.get("resultType") == "complete", "many modern error is not complete")
                    exact(result.get("_meta"), {"io.modelcontextprotocol/serverInfo": {"name": "gooo-service-grant", "version": "0.1.0-dev"}})
                need(type(result.get("content")) is list and len(result["content"]) == 1
                     and set(result["content"][0]) == {"type", "text"}
                     and result["content"][0]["type"] == "text"
                     and result["content"][0]["text"].startswith("INVALID_FACTS:"),
                     "many structural failure diagnostic differs")
    return report


def native_many_verify(root, binary, modcache, output):
    output.mkdir(mode=0o700)
    tracker = OwnedChildren(output)
    verifier = imported("mcp_bundle_many_wire_verifier", root/"examples/service-grant-mcp/verify_many.py")
    verifier.subprocess = types.SimpleNamespace(Popen=tracker.spawn, PIPE=subprocess.PIPE,
                                               TimeoutExpired=subprocess.TimeoutExpired)
    stdout, stderr, failure = BoundedText(), BoundedText(), None
    previous_argv = sys.argv
    old_handlers, pending = {}, []
    def interrupted(number, _frame):
        # No asynchronous exception is allowed between finally entry and close,
        # or inside a per-child close. The client checks this pending state.
        if not pending:
            pending.append(number)
        if verifier.PENDING_SIGNAL is None:
            verifier.PENDING_SIGNAL = number
    try:
        for number in (signal.SIGINT, signal.SIGTERM, signal.SIGALRM):
            old_handlers[number] = signal.getsignal(number)
            signal.signal(number, interrupted)
        sys.argv = [str(root/"examples/service-grant-mcp/verify_many.py"), "--binary", str(binary),
            "--legacy-verifier", str(root/"examples/service-grant-mcp/verify.py"), "--many-module",
            str(modcache/(MANY_MODULE+"@"+MANY_VERSION)), "--out", str(output/"wire")]
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            verifier.main()
    except BaseException as error:
        failure = error
    finally:
        # The latch remains installed through diagnostics and every child wait.
        # Keep the existing global180s timer armed: its signal becomes a pending
        # client cancellation, never permission to skip owned cleanup.
        sys.argv = previous_argv
        try:
            (output/"verifier.stdout").write_text(stdout.getvalue())
            (output/"verifier.stderr").write_text(stderr.getvalue())
        except BaseException as error:
            if failure is None:
                failure = error
            elif hasattr(failure, "add_note"):
                failure.add_note("many verifier diagnostics: "+str(error))
        try:
            tracker.close(failure)
        except BaseException as error:
            if failure is None:
                failure = error
            elif error is not failure and hasattr(failure, "add_note"):
                failure.add_note("many child cleanup: "+str(error))
        if failure is None and pending:
            failure = verifier.WireCancelled("optional wire interrupted by signal "+str(pending[0]))
        try:
            if failure is not None:
                (output/"first-failure.json").write_bytes(encoded({"error": repr(failure),
                    "pending_signals": pending, "owned_children": tracker.records}))
        except BaseException as error:
            if failure is None:
                failure = error
            elif hasattr(failure, "add_note"):
                failure.add_note("many first-failure diagnostic: "+str(error))
        finally:
            for number, handler in old_handlers.items():
                signal.signal(number, handler)
    if failure is not None:
        raise failure
    return output/"wire/acceptance.json"


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
        inventory.append({"file": name, "GoVersion": "go1.27.3", "source_relative_path": relative,
                          "sha256": row["sha256"], "scope": row["scope"]})
    for name, relative, digest in [
        ("LICENSE-transport-BSD", "examples/service-grant-mcp/LICENSE", "099e1adf9145f6fe45357f004f503231a21b58244fdf105a34117d1a33b4bd48"),
        ("LICENSE-generation-tool-Gooo-MIT", "tools/licenses/gooo-83928fa-MIT.txt", "5177457bc30b96eae6f786d0fbe3d1fda382edef61ecef25eb6f446a8b35b4f3")]:
        raw = member(root, relative)
        need(sha(raw) == digest, "transport/generator notice bytes differ")
        files[name] = (raw, 0o644)
        inventory.append({"file": name, "source_relative_path": relative, "sha256": digest,
                          "scope": "Generation tool attribution only; no Gooo compiler is included" if "Gooo" in name else "transport license"})
    need(len(files) == 58, "exact 58 notice texts required")
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
            packager_commit=None, verify_native=False, many_wire_path=None):
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
        generation, many_generation = source_contract(root, modcache)
        report = load(read(wire_path, MAX_SOURCE))
        checked_report(report, build, sha(raw), root, generation)
        need(isinstance(many_wire_path, Path), "source-bound many wire report required")
        many_report = checked_many_report(many_wire_path, binary, root, modcache, many_generation)
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
                 "Optional: args [--many-required] additionally exposes assess_service_grant_change.\n"
                 "Its OBJECT arguments use before, after and required_requests [{type,name},...]; order and duplicates survive.\n"
                 f"Optional results retain ordered-column facts, 12 decision fields, compiler {MANY_PIN}, generation {MANY_GENERATION}, and 30-asset/5-source bindings.\n"
                 "No authority is authenticated and no grant is applied in either mode.\n"
                 "Some included Go toolchain notices concern compiler/test components; inventory scope is explicit.\n")
        files["START-HERE.txt"] = (guide.encode(), 0o644)
        manifest = {"schema": "gooo/service-grant-mcp-native-bundle/v2", "platform": target,
                    "source_commit": commit, "legacy_transport_reference": REFERENCE_SOURCE, "source_files": SOURCE_FILES,
                    "packager": {"commit": packager_commit, "kind": "uncommitted-private-prototype" if packager_commit is None else "committed-CI-candidate",
                                 "source_sha256": sha(read(Path(__file__), MAX_SOURCE)), "support_sources": SUPPORT_HASHES},
                    "compiler_source_sha": PIN, "generation_sha256": "sha256:"+GENERATION,
                    "generation_origin": "fresh_pinned_generation", "assets": generation["assets"],
                    "optional_profiles": {"many_required": {"argv": ["--many-required"],
                        "additional_tool": "assess_service_grant_change", "module": {"Path": MANY_MODULE, "Version": MANY_VERSION, "Sum": MANY_SUM},
                        "compiler_source_sha": MANY_PIN, "generation_sha256": "sha256:"+MANY_GENERATION,
                        "assets": many_generation["assets"], "sources": many_generation["public_sources"]}},
                    "build_info": build, "binary_sha256": sha(raw), "notice_texts": 58,
                    "finite_checks": {"original_wire_report_binary_SHA_equal": True, "original_report_proofs": 8,
                                      "original_many_report_binary_SHA_equal": True, "many_reference_comparisons": 30,
                                      "many_policy257_observations": 2,
                                      "fresh_extracted_native_wire_requested": verify_native,
                                      "new_Linux_execution": verify_native and target == "linux/amd64",
                                      "grant_applied": False, "producer_authenticated": False, "external_adoption": False},
                    "members": {name: {"bytes": len(content), "sha256": sha(content), "mode": mode}
                                for name, (content, mode) in sorted(files.items())}}
        files["BUNDLE.json"] = (encoded(manifest), 0o644)
        need(len(files) == 63, "exact sixty-three archive members required")
        name = COMMAND+"-"+target.replace("/", "-")+".tar.gz"
        candidate = work/name
        SERVICE.write_archive(candidate, files)
        extracted = work/"extracted"
        extract_checked(candidate, files, extracted)
        new_report, new_many_report = None, None
        if verify_native:
            need(SERVICE.native_platform() == target, "native runtime platform differs")
            new_report = native_verify(root, extracted/COMMAND, extracted/"BUILD-INFO.json", out/"native-verification")
            checked_report(new_report, build, sha(raw), root, generation)
            native_many_path = native_many_verify(root, extracted/COMMAND, modcache, out/"many-native-verification")
            new_many_report = checked_many_report(native_many_path, extracted/COMMAND, root, modcache, many_generation)
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
                  "new_extracted_many_wire": new_many_report is not None,
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
    for name in ("binary", "build-info", "wire-report", "many-wire-report", "module-cache", "go-root", "root", "out"):
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
                         args.out.absolute(), args.source_commit, args.packager_commit, args.verify_native,
                         args.many_wire_report.absolute())
        print(json.dumps({"status": result["status"], "platform": result["platform"],
                          "archive_sha256": result["archive"]["sha256"], "new_native_wire": result["new_extracted_native_wire"]}))
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)


if __name__ == "__main__":
    main()
