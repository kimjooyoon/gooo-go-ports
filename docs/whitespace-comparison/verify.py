import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parent


def require(condition, message):
    if not condition:
        raise ValueError(message)


go = json.loads((root / 'go-results.json').read_text())
js = json.loads((root / 'js-results.json').read_text())
require(go['runtime'] == 'go1.27.2', 'Unexpected Go runtime version')
require(js['runtime'] == 'v22.23.1', 'Unexpected Node runtime version')
require(go['examined'] == js['examined'] == 0x110000 - 0x800,
        'Unexpected scalar enumeration count')
require(go['is_space_trim_disagreements'] == js['trim_regexp_disagreements'] == 0,
        'Predicates disagree inside one runtime')
go_points = set(go['whitespace_code_points'])
js_points = set(js['whitespace_code_points'])
require(len(go_points) == go['whitespace_count'] == 25, 'Unexpected Go whitespace set')
require(len(js_points) == js['whitespace_count'] == 25, 'Unexpected JS whitespace set')
require(go_points - js_points == {0x85}, 'Unexpected Go-only difference')
require(js_points - go_points == {0xfeff}, 'Unexpected JS-only difference')
require(set(js['unicode_property_code_points']) == go_points,
        'JavaScript Unicode White_Space and Go disagree')
manual = {'U+0085': (True, False), 'U+FEFF': (False, True),
          'U+200B': (False, False), 'U+00A0': (True, True)}
require(len(go['samples']) == len(js['samples']) == len(manual), 'Wrong sample count')
for g, j in zip(go['samples'], js['samples']):
    require(g['code_point'] == j['code_point'], 'Mismatched sample code point')
    require((g['trim_empty'], j['trim_empty']) == manual[g['code_point']],
            'Sample differs from manual expectation')
    require(g['is_space'] == j['unicode_white_space'], 'Sample property mismatch')
files = ['main.go', 'compare.cjs', 'verify.py', 'go-results.json', 'js-results.json']
result = {
    'status': 'PASS', 'runtime_versions': [go['runtime'], js['runtime']],
    'scalar_values_per_runtime': go['examined'],
    'go_only': ['U+0085'], 'javascript_only': ['U+FEFF'],
    'shared_whitespace_count': len(go_points & js_points),
    'javascript_unicode_property_matches_go': True,
    'manual_examples': 4,
    'commands': ['GOTOOLCHAIN=go1.27.2 go run main.go > go-results.json',
                 'node compare.cjs > js-results.json', 'python3 verify.py'],
    'source_and_result_sha256': {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in files},
    'scope': 'Exact installed versions, one-scalar strings. No multi-character normalization, grapheme, invalid UTF-8, performance, production incident, or security guarantee. This compares Go and JavaScript, not an exhaustive Gooo run.',
}
(root / 'comparison.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result, indent=2))
